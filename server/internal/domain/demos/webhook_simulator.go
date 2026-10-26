package demos

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// The webhooks topic's exhibit: a REAL deterministic delivery simulation.
// The HMAC signing, the timestamp+nonce verification, and the exponential
// backoff schedule below are the real thing — crypto/hmac and crypto/sha256
// from the standard library — run over static exhibit facts so every output
// is reproducible. The webhooks topic quotes this file as real code; the
// live demo (api:webhook-simulator) is this simulation's ledger on the wire.
//
// Determinism rules: exhibit times are FIXED RFC 3339 strings (no wall
// clock), the retry ladder computes delays as DATA (1s/2s/4s/8s — nothing
// sleeps), and no goroutine is needed anywhere: the simulation is a pure
// function from constants to its report.

// Exhibit facts — the one delivery every signature case reasons about.
const (
	// webhookSecret is the exhibit signing key. It is demo material baked
	// into the guide on purpose: a REAL secret arrives by environment
	// (WEBHOOK_SIGNING_SECRET), never as source. The payload says so.
	webhookSecret = "whsec_exhibit_7f3a9c"

	// webhookIssuedAt is the sender's clock when the delivery was signed.
	webhookIssuedAt = "2026-10-26T09:00:00Z"

	// webhookNonce breaks two deliveries signed in the same second apart.
	webhookNonce = "n-4f2c1b"

	// webhookBody is the canonical signed body: the exact bytes on the
	// wire, which is the only thing a receiver may build the HMAC over.
	webhookBody = `{"eventId":"evt_0142","type":"invoice.paid","version":"1",` +
		`"occurredAt":"2026-10-26T08:59:59Z","data":{"invoiceId":"in_88","amountCents":42000,"currency":"USD"}}`
)

// Signature discipline knobs, shared by sender and receiver.
const (
	// webhookTolerance is how old a signed timestamp may be and still
	// count as fresh — the replay window.
	webhookTolerance = 5 * time.Minute

	// webhookHeaderName is the header the signature rides in.
	webhookHeaderName = "X-Guide-Signature"

	// webhookHeaderFormat documents the header's shape for the payload.
	webhookHeaderFormat = "t={timestamp},n={nonce},v1={hex-signature}"
)

// Retry ladder knobs: the schedule the simulation walks.
const (
	webhookMaxAttempts  = 5
	webhookBaseDelay    = 1 * time.Second
	webhookMaxDelay     = 30 * time.Second
	webhookDelayPattern = "base × 2^(attempt-1), capped at max"
)

// The receiver's rejections — typed, so the payload reports distinct
// reasons and the receiving topic can teach the ORDER they fire in.
var (
	// errStaleTimestamp: outside the tolerance window — do not even look
	// at the signature yet; a replay should die at the clock check.
	errStaleTimestamp = errors.New(
		"timestamp outside the tolerance window: replay or badly skewed clock",
	)
	// errSignatureMismatch: the rebuilt HMAC does not equal the presented
	// one. A wrong key and a tampered body land here identically — the
	// receiver cannot (and need not) tell them apart.
	errSignatureMismatch = errors.New("signature mismatch: wrong key or altered payload")
)

// signDelivery computes the v1 signature over the canonical message
// "{timestamp}.{nonce}.{body}" with HMAC-SHA256 — the exact string the
// receiver must rebuild before comparing. Body is the RAW wire bytes: a
// receiver that re-marshals JSON before verifying changes the bytes and
// the signature with it.
func signDelivery(secret, timestamp, nonce string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%s.%s.%s", timestamp, nonce, body)
	return hex.EncodeToString(mac.Sum(nil))
}

// signatureParts is one parsed X-Guide-Signature header.
type signatureParts struct {
	Timestamp string
	Nonce     string
	Signature string
}

// parseSignatureHeader splits "t=…,n=…,v1=…" into its parts, refusing any
// malformed or incomplete header instead of guessing defaults.
func parseSignatureHeader(header string) (signatureParts, error) {
	var parts signatureParts
	seen := 0
	for _, field := range strings.Split(header, ",") {
		key, value, found := strings.Cut(strings.TrimSpace(field), "=")
		if !found {
			return signatureParts{}, fmt.Errorf("malformed signature field %q", field)
		}
		switch key {
		case "t":
			parts.Timestamp = value
		case "n":
			parts.Nonce = value
		case "v1":
			parts.Signature = value
		default:
			return signatureParts{}, fmt.Errorf("unknown signature field %q", key)
		}
		seen++
	}
	if seen != 3 || parts.Timestamp == "" || parts.Nonce == "" || parts.Signature == "" {
		return signatureParts{}, fmt.Errorf("incomplete signature header %q", header)
	}
	return parts, nil
}

// verifyDelivery is the receiver's gate, in the order that matters: the
// clock check runs FIRST (a replay dies before any crypto), then the
// rebuilt HMAC is compared to the presented one in constant time.
func verifyDelivery(secret string, parts signatureParts, receivedAt time.Time, body []byte) error {
	issuedAt, err := time.Parse(time.RFC3339, parts.Timestamp)
	if err != nil {
		return fmt.Errorf("unparseable timestamp %q: %w", parts.Timestamp, err)
	}
	// Absolute delta: far-past AND far-future timestamps are stale.
	if delta := receivedAt.Sub(issuedAt); delta < -webhookTolerance || delta > webhookTolerance {
		return errStaleTimestamp
	}
	expected := signDelivery(secret, parts.Timestamp, parts.Nonce, body)
	if !hmac.Equal([]byte(expected), []byte(parts.Signature)) {
		return errSignatureMismatch
	}
	return nil
}

// backoffDelay is the wait BEFORE the given attempt (1-based): base ×
// 2^(attempt-1), capped at max. Attempt 1 waits nothing — the schedule
// spaces out the RETRIES, not the first try. The result is data: callers
// decide how to wait (or, in this simulation, just report it).
func backoffDelay(attempt int, base, max time.Duration) time.Duration {
	if attempt <= 1 {
		return 0
	}
	delay := base
	for i := 2; i < attempt; i++ {
		delay *= 2
		if delay >= max {
			return max
		}
	}
	return delay
}

// SimulatorConfig is the fixed knob set the whole simulation runs under.
type SimulatorConfig struct {
	Algorithm        string `json:"algorithm"`
	SignatureHeader  string `json:"signatureHeader"`
	HeaderFormat     string `json:"headerFormat"`
	MessageFormat    string `json:"messageFormat"`
	ToleranceSeconds int    `json:"toleranceSeconds"`
	MaxAttempts      int    `json:"maxAttempts"`
	BaseDelaySeconds int    `json:"baseDelaySeconds"`
	MaxDelaySeconds  int    `json:"maxDelaySeconds"`
	DelayPattern     string `json:"delayPattern"`
}

// SignatureCase is one verification verdict: a receiver configuration, a
// body, a presented signature — and what the real gate did with them.
// Reason is "" on the verified case (empty is DATA: no error happened).
type SignatureCase struct {
	ID            string `json:"id"`
	Description   string `json:"description"`
	ReceiverClock string `json:"receiverClock"`
	Presented     string `json:"presented"`
	Verified      bool   `json:"verified"`
	Reason        string `json:"reason"`
	Note          string `json:"note"`
}

// SigningExhibit is the sender's signing material plus the four verdicts
// the receiver's gate produced for it.
type SigningExhibit struct {
	SecretHint string          `json:"secretHint"`
	Body       string          `json:"body"`
	Issued     IssuedSignature `json:"issued"`
	Cases      []SignatureCase `json:"cases"`
}

// IssuedSignature is what the sender computed and put on the wire.
type IssuedSignature struct {
	Timestamp  string `json:"timestamp"`
	Nonce      string `json:"nonce"`
	Signature  string `json:"signature"`
	HeaderLine string `json:"headerLine"`
}

// AttemptRow is one attempt in a delivery's ledger: the wait that preceded
// it, the endpoint's simulated status, and the ladder's ruling.
type AttemptRow struct {
	Attempt     int    `json:"attempt"`
	WaitSeconds int    `json:"waitSeconds"`
	Status      int    `json:"status"`
	Ruling      string `json:"ruling"`
}

// DeliveryLedger is one delivery walked through the retry ladder: every
// attempt with its backoff wait, the final outcome, and — when the ladder
// exhausted — the dead-letter record it landed in.
type DeliveryLedger struct {
	DeliveryID       string       `json:"deliveryId"`
	Event            string       `json:"event"`
	Endpoint         string       `json:"endpoint"`
	Attempts         []AttemptRow `json:"attempts"`
	Outcome          string       `json:"outcome"`
	AttemptsUsed     int          `json:"attemptsUsed"`
	TotalWaitSeconds int          `json:"totalWaitSeconds"`
	DeadLetter       *DeadLetter  `json:"deadLetter,omitempty"`
}

// DeadLetter is the DLQ record: where the delivery went when the ladder
// gave up, and the operational door out (replay) it left behind.
type DeadLetter struct {
	Queue      string `json:"queue"`
	Reason     string `json:"reason"`
	Replayable bool   `json:"replayable"`
}

// ReceiverStep is one move in the receiving side's front-door sequence.
type ReceiverStep struct {
	Step   int    `json:"step"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

// ReceiverRun is the receiving beat: the verified gate run, the immediate
// 2xx answer, and the work that happened AFTER the answer (async, by
// design), ending in the idempotency dedupe every at-least-once sender
// forces onto its receivers.
type ReceiverRun struct {
	AnsweredStatus int            `json:"answeredStatus"`
	AnsweredBody   string         `json:"answeredBody"`
	AsyncBefore2xx []ReceiverStep `json:"asyncBefore2xx"`
	AsyncAfter2xx  []ReceiverStep `json:"asyncAfter2xx"`
}

// WebhookSimulatorPayload is the wire shape of GET
// /api/demo/api/webhook-simulator (camelCase JSON) — the webhooks topic's
// inline demo: the signing discipline as real verdicts, two deliveries
// through the retry ladder (one recovers, one dead-letters), and the
// receiver's front door as it actually answers.
type WebhookSimulatorPayload struct {
	Config     SimulatorConfig  `json:"config"`
	Signing    SigningExhibit   `json:"signing"`
	Deliveries []DeliveryLedger `json:"deliveries"`
	Receiver   ReceiverRun      `json:"receiver"`
}

// webhookClocks are the fixed receiver clocks the four signature cases
// run at — all RFC 3339, all deterministic.
const (
	// clockFresh: 41 seconds after issuing — inside the window.
	clockFresh = "2026-10-26T09:00:41Z"
	// clockLate: 7 minutes after issuing — outside the 5-minute window.
	clockLate = "2026-10-26T09:07:00Z"
)

// parseWebhookClock parses a fixed exhibit clock; the values are constants,
// so a parse failure is a programmer error in this file.
func parseWebhookClock(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic("webhook simulator: exhibit clock " + value + " must parse")
	}
	return parsed
}

// signingExhibit signs the exhibit body for real, then runs the receiver's
// gate over four configurations: the honest delivery, a tampered payload,
// a stale timestamp, and a signature made with the wrong key.
func signingExhibit() SigningExhibit {
	issued := IssuedSignature{
		Timestamp: webhookIssuedAt,
		Nonce:     webhookNonce,
		Signature: signDelivery(webhookSecret, webhookIssuedAt, webhookNonce, []byte(webhookBody)),
		HeaderLine: fmt.Sprintf(
			"t=%s,n=%s,v1=%s",
			webhookIssuedAt, webhookNonce, signDelivery(webhookSecret, webhookIssuedAt, webhookNonce, []byte(webhookBody)),
		),
	}

	honestParts, err := parseSignatureHeader(issued.HeaderLine)
	if err != nil {
		// The line was built from three known fields; a failure here is
		// a bug in this exhibit, not a runtime condition.
		panic("webhook simulator: exhibit header must parse: " + err.Error())
	}

	// Case 2 tampers with the body AFTER signing: same signature, richer
	// invoice. The gate must rebuild over what ARRIVED and reject.
	tamperedBody := strings.Replace(webhookBody, `"amountCents":42000`, `"amountCents":99000`, 1)

	// Case 4 signs with the wrong key — a misconfigured sender or an
	// attacker. The receiver cannot tell that from tampering: both are a
	// mismatch, and both are rejected identically.
	wrongKey := "whsec_attacker_deadbeef"
	wrongKeyParts := signatureParts{
		Timestamp: webhookIssuedAt,
		Nonce:     webhookNonce,
		Signature: signDelivery(wrongKey, webhookIssuedAt, webhookNonce, []byte(webhookBody)),
	}

	// Every case below is VERDICTED BY THE REAL GATE: verifyDelivery runs
	// for each configuration and its error (or its absence) becomes the
	// row's verdict and reason — no hand-asserted outcomes.
	caseFor := func(id, description, clock string, body []byte, parts signatureParts, note string) SignatureCase {
		err := verifyDelivery(webhookSecret, parts, parseWebhookClock(clock), body)
		reason := ""
		if err != nil {
			reason = err.Error()
		}
		return SignatureCase{
			ID:            id,
			Description:   description,
			ReceiverClock: clock,
			Presented:     "v1=" + parts.Signature[:16] + "…",
			Verified:      err == nil,
			Reason:        reason,
			Note:          note,
		}
	}

	return SigningExhibit{
		SecretHint: "whsec_… (exhibit key, demo-only; real keys come from the environment)",
		Body:       webhookBody,
		Issued:     issued,
		Cases: []SignatureCase{
			caseFor("correct-delivery",
				"the honest delivery: original body, original signature, clock 41s after signing",
				clockFresh, []byte(webhookBody), honestParts,
				"verified: rebuilt HMAC equals the presented one, timestamp inside the window"),
			caseFor("tampered-payload",
				"amount rewritten to 99000 after signing — signature still the original one",
				clockFresh, []byte(tamperedBody), honestParts,
				"the gate rebuilds over the bytes that ARRIVED, so the rewrite breaks the MAC — even one digit"),
			caseFor("stale-timestamp",
				"a seven-minute-old timestamp replayed at the receiver — signature itself is valid",
				clockLate, []byte(webhookBody), honestParts,
				"rejected at the clock check BEFORE any crypto: an intercepted delivery cannot be replayed next week"),
			caseFor("wrong-key",
				"signature computed with a different secret (misconfigured sender or forger)",
				clockFresh, []byte(webhookBody), wrongKeyParts,
				"indistinguishable from tampering by design: the receiver just answers mismatch — either way, reject"),
		},
	}
}

// simulateDelivery walks one delivery through the retry ladder over a
// static outcome script (the endpoint statuses the simulation assigns).
// Nothing sleeps: the waits are computed, recorded as data, and summed.
func simulateDelivery(deliveryID, event, endpoint string, endpointStatuses []int) DeliveryLedger {
	ledger := DeliveryLedger{
		DeliveryID: deliveryID,
		Event:      event,
		Endpoint:   endpoint,
		Attempts:   make([]AttemptRow, 0, len(endpointStatuses)),
	}
	for attempt, status := range endpointStatuses {
		wait := backoffDelay(attempt+1, webhookBaseDelay, webhookMaxDelay)
		// The wait PRECEDES its attempt, so it is waited even when the
		// attempt then succeeds or exhausts the ladder.
		ledger.TotalWaitSeconds += int(wait.Seconds())
		row := AttemptRow{
			Attempt:     attempt + 1,
			WaitSeconds: int(wait.Seconds()),
			Status:      status,
			Ruling:      "retry-scheduled",
		}
		if status >= 200 && status < 300 {
			row.Ruling = "delivered"
			ledger.Attempts = append(ledger.Attempts, row)
			ledger.Outcome = "delivered"
			ledger.AttemptsUsed = attempt + 1
			return ledger
		}
		if attempt+1 == webhookMaxAttempts {
			row.Ruling = "dead-lettered"
			ledger.Attempts = append(ledger.Attempts, row)
			ledger.Outcome = "dead-lettered"
			ledger.AttemptsUsed = attempt + 1
			ledger.DeadLetter = &DeadLetter{
				Queue:      "webhook-dlq",
				Reason:     fmt.Sprintf("no 2xx after %d attempts", webhookMaxAttempts),
				Replayable: true,
			}
			return ledger
		}
		ledger.Attempts = append(ledger.Attempts, row)
	}
	return ledger
}

// receptionOutcome is what the receiving side's front door answered, and
// (when it refused) why — internal to the simulation; the payload's
// receiver beat carries the narrative form.
type receptionOutcome struct {
	status int
	body   string
	reason string
}

// receiveDelivery is the receiving side's front door, sequenced exactly as
// the topic teaches: parse the header, verify against the RAW body, answer
// 2xx immediately — business work happens after the answer, elsewhere.
func receiveDelivery(secret, headerLine string, receivedAt time.Time, rawBody []byte) receptionOutcome {
	parts, err := parseSignatureHeader(headerLine)
	if err == nil {
		err = verifyDelivery(secret, parts, receivedAt, rawBody)
	}
	if err == nil {
		return receptionOutcome{status: httpStatusOK, body: `{"received":true}`}
	}
	return receptionOutcome{status: httpStatusUnauthorized, body: `{"received":false}`, reason: err.Error()}
}

// httpStatus constants keep the receiver beat free of a net/http import
// while still speaking real status codes on the wire.
const (
	httpStatusOK           = 200
	httpStatusUnauthorized = 401
)

// receiverRun performs the exhibit's receiving beat against the honest
// delivery: the two steps that gate the answer, the answer itself, and
// the three that happen asynchronously after it.
func receiverRun() ReceiverRun {
	headerLine := fmt.Sprintf(
		"t=%s,n=%s,v1=%s",
		webhookIssuedAt, webhookNonce, signDelivery(webhookSecret, webhookIssuedAt, webhookNonce, []byte(webhookBody)),
	)
	outcome := receiveDelivery(webhookSecret, headerLine, parseWebhookClock(clockFresh), []byte(webhookBody))
	if outcome.status != httpStatusOK {
		// The exhibit's honest delivery must pass its own gate; anything
		// else is a bug in this file, worth failing loudly.
		panic("webhook simulator: the honest delivery must verify")
	}
	return ReceiverRun{
		AnsweredStatus: outcome.status,
		AnsweredBody:   outcome.body,
		AsyncBefore2xx: []ReceiverStep{
			{Step: 1, Title: "Read the raw body", Detail: "io.ReadAll on the request stream — no JSON parsing yet: the signature covers the exact bytes, and re-marshaling changes them."},
			{Step: 2, Title: "Verify before trusting", Detail: "parse X-Guide-Signature, check the timestamp window, rebuild the HMAC, compare in constant time — this demo's own gate, on the exhibit delivery."},
		},
		AsyncAfter2xx: []ReceiverStep{
			{Step: 3, Title: "Answer 2xx immediately", Detail: `write 200 {"received":true} and return from the handler — the sender's retry clock stops now, not when processing finishes.`},
			{Step: 4, Title: "Hand off to a worker", Detail: "push the verified event onto a channel or queue (in this guide's terms: topic 29 background-jobs) — the handler is done."},
			{Step: 5, Title: "Dedupe by eventId", Detail: "at-least-once means duplicates WILL arrive; evt_0142 gets an insert-only ledger row, so the second copy is a no-op."},
		},
	}
}

// BuildWebhookSimulatorPayload assembles the whole demo payload. It is
// pure: same call, same bytes — fixed clocks, computed HMACs, a retry
// ladder made of arithmetic — the handler layer adds only transport.
func BuildWebhookSimulatorPayload() WebhookSimulatorPayload {
	return WebhookSimulatorPayload{
		Config: SimulatorConfig{
			Algorithm:        "HMAC-SHA256",
			SignatureHeader:  webhookHeaderName,
			HeaderFormat:     webhookHeaderFormat,
			MessageFormat:    "{timestamp}.{nonce}.{body}",
			ToleranceSeconds: int(webhookTolerance.Seconds()),
			MaxAttempts:      webhookMaxAttempts,
			BaseDelaySeconds: int(webhookBaseDelay.Seconds()),
			MaxDelaySeconds:  int(webhookMaxDelay.Seconds()),
			DelayPattern:     webhookDelayPattern,
		},
		Signing: signingExhibit(),
		Deliveries: []DeliveryLedger{
			// A flaky endpoint that recovers: 500, 503, then 200 on the
			// third attempt, after 1s + 2s of computed backoff.
			simulateDelivery(
				"dlv_0207", "invoice.paid", "https://hooks.example.com/invoices",
				[]int{500, 503, 200},
			),
			// A hard-down endpoint: five failures, waits 0/1/2/4/8s, and
			// the ladder gives up into the dead-letter queue.
			simulateDelivery(
				"dlv_0208", "subscription.canceled", "https://legacy.example.com/hooks",
				[]int{500, 502, 500, 504, 500},
			),
		},
		Receiver: receiverRun(),
	}
}
