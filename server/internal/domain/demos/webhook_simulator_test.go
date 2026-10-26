package demos

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// The webhook simulator's payload is computed live from real HMACs, real
// clock arithmetic, and a real backoff schedule; these tests pin the facts
// the webhooks topic's prose claims — and attack them: every hostile case
// (tampered body, stale timestamp, wrong key, malformed header) must FAIL
// the gate, the honest one must pass, and nothing anywhere may sleep.

func webhookPayload() WebhookSimulatorPayload {
	return BuildWebhookSimulatorPayload()
}

func TestWebhookSimulatorSignsDeterministically(t *testing.T) {
	// The same inputs must produce the same hex signature — the receiver's
	// rebuild depends on bit-for-bit agreement.
	first := signDelivery(webhookSecret, webhookIssuedAt, webhookNonce, []byte(webhookBody))
	second := signDelivery(webhookSecret, webhookIssuedAt, webhookNonce, []byte(webhookBody))
	if first != second || len(first) != 64 {
		t.Fatalf("signature = %q (len %d), want a stable 64-char hex digest", first, len(first))
	}
	// And it is genuinely an HMAC over the canonical message: one byte of
	// key or message difference must change it entirely.
	if altered := signDelivery(webhookSecret+"x", webhookIssuedAt, webhookNonce, []byte(webhookBody)); altered == first {
		t.Error("changing the secret must change the signature")
	}
	if altered := signDelivery(webhookSecret, webhookIssuedAt, webhookNonce, []byte(webhookBody+" ")); altered == first {
		t.Error("changing the body must change the signature")
	}
}

func TestWebhookSimulatorVerifyAcceptsTheHonestDelivery(t *testing.T) {
	parts, err := parseSignatureHeader("t=" + webhookIssuedAt + ",n=" + webhookNonce +
		",v1=" + signDelivery(webhookSecret, webhookIssuedAt, webhookNonce, []byte(webhookBody)))
	if err != nil {
		t.Fatalf("honest header must parse: %v", err)
	}
	if err := verifyDelivery(webhookSecret, parts, parseWebhookClock(clockFresh), []byte(webhookBody)); err != nil {
		t.Fatalf("honest delivery must verify: %v", err)
	}
}

func TestWebhookSimulatorVerifyRejectsEveryHostileCase(t *testing.T) {
	signature := signDelivery(webhookSecret, webhookIssuedAt, webhookNonce, []byte(webhookBody))
	parts := signatureParts{Timestamp: webhookIssuedAt, Nonce: webhookNonce, Signature: signature}
	fresh := parseWebhookClock(clockFresh)

	tampered := strings.Replace(webhookBody, "42000", "99000", 1)
	if tampered == webhookBody {
		t.Fatal("tamper fixture failed to change the body")
	}

	cases := []struct {
		name string
		err  error
		want error
	}{
		{"tampered payload", verifyDelivery(webhookSecret, parts, fresh, []byte(tampered)), errSignatureMismatch},
		{"stale timestamp", verifyDelivery(webhookSecret, parts, parseWebhookClock(clockLate), []byte(webhookBody)), errStaleTimestamp},
		{
			"wrong key signature",
			verifyDelivery(webhookSecret, signatureParts{
				Timestamp: webhookIssuedAt,
				Nonce:     webhookNonce,
				Signature: signDelivery("whsec_attacker_deadbeef", webhookIssuedAt, webhookNonce, []byte(webhookBody)),
			}, fresh, []byte(webhookBody)),
			errSignatureMismatch,
		},
		{
			"far-future timestamp",
			verifyDelivery(webhookSecret, signatureParts{
				Timestamp: "2026-10-26T09:10:00Z",
				Nonce:     webhookNonce,
				Signature: signature,
			}, fresh, []byte(webhookBody)),
			errStaleTimestamp,
		},
	}
	for _, tc := range cases {
		if tc.err == nil {
			t.Errorf("%s: gate ACCEPTED a hostile delivery", tc.name)
			continue
		}
		if tc.err != tc.want {
			t.Errorf("%s: err = %v, want %v", tc.name, tc.err, tc.want)
		}
	}
}

func TestWebhookSimulatorSignatureHeaderParsingRefusesMalformedInput(t *testing.T) {
	good := "t=2026-10-26T09:00:00Z,n=n-4f2c1b,v1=ab12cd34"
	if _, err := parseSignatureHeader(good); err != nil {
		t.Fatalf("well-formed header must parse: %v", err)
	}
	for _, bad := range []string{
		"",                                      // empty
		"v1=ab12cd34",                           // missing t and n
		"t=2026-10-26T09:00:00Z,n=,v1=ab12cd34", // empty nonce
		"t=2026-10-26T09:00:00Z v1=ab12cd34",    // missing comma
		"t=2026-10-26T09:00:00Z,n=n-4f2c1b,v1=ab12cd34,x=9", // unknown field
	} {
		if _, err := parseSignatureHeader(bad); err == nil {
			t.Errorf("malformed header %q must be refused", bad)
		}
	}
}

func TestWebhookSimulatorBackoffScheduleIsComputedData(t *testing.T) {
	// 1s/2s/4s/8s: pure arithmetic, no clock, no sleep.
	want := []time.Duration{0, time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}
	for attempt, delay := range want {
		if got := backoffDelay(attempt+1, webhookBaseDelay, webhookMaxDelay); got != delay {
			t.Errorf("backoffDelay(%d) = %v, want %v", attempt+1, got, delay)
		}
	}
	// The cap engages: doubling 1→2→4→8→16→32 hits the 30s cap at
	// attempt 7 and stays there — never overflows.
	for attempt := 7; attempt < 40; attempt++ {
		if got := backoffDelay(attempt, webhookBaseDelay, webhookMaxDelay); got != webhookMaxDelay {
			t.Fatalf("backoffDelay(%d) = %v, want the %v cap", attempt, got, webhookMaxDelay)
		}
	}
}

func TestWebhookSimulatorLedgerRecoversOnThirdAttempt(t *testing.T) {
	ledger := webhookPayload().Deliveries[0]
	if ledger.DeliveryID != "dlv_0207" || ledger.Outcome != "delivered" {
		t.Fatalf("first ledger = %+v, want dlv_0207 delivered", ledger)
	}
	if ledger.AttemptsUsed != 3 || len(ledger.Attempts) != 3 {
		t.Fatalf("attempts used = %d of %d rows, want 3 of 3", ledger.AttemptsUsed, len(ledger.Attempts))
	}
	wantWaits := []int{0, 1, 2}
	for i, row := range ledger.Attempts {
		if row.WaitSeconds != wantWaits[i] {
			t.Errorf("attempt %d wait = %ds, want %ds", row.Attempt, row.WaitSeconds, wantWaits[i])
		}
	}
	if ledger.Attempts[2].Ruling != "delivered" || ledger.Attempts[2].Status != 200 {
		t.Errorf("third attempt = %+v, want delivered on 200", ledger.Attempts[2])
	}
	if ledger.TotalWaitSeconds != 3 || ledger.DeadLetter != nil {
		t.Errorf("recovered delivery totalWait = %d, deadLetter = %+v, want 3s and none", ledger.TotalWaitSeconds, ledger.DeadLetter)
	}
}

func TestWebhookSimulatorLedgerDeadLettersAfterMaxAttempts(t *testing.T) {
	ledger := webhookPayload().Deliveries[1]
	if ledger.DeliveryID != "dlv_0208" || ledger.Outcome != "dead-lettered" {
		t.Fatalf("second ledger = %+v, want dlv_0208 dead-lettered", ledger)
	}
	if ledger.AttemptsUsed != webhookMaxAttempts || len(ledger.Attempts) != webhookMaxAttempts {
		t.Fatalf("attempts used = %d of %d rows, want %d of %d", ledger.AttemptsUsed, len(ledger.Attempts), webhookMaxAttempts, webhookMaxAttempts)
	}
	wantWaits := []int{0, 1, 2, 4, 8}
	for i, row := range ledger.Attempts {
		if row.WaitSeconds != wantWaits[i] {
			t.Errorf("attempt %d wait = %ds, want %ds", row.Attempt, row.WaitSeconds, wantWaits[i])
		}
		if row.Ruling == "delivered" {
			t.Errorf("exhausted ladder row %d claims delivered", row.Attempt)
		}
	}
	if ledger.Attempts[webhookMaxAttempts-1].Ruling != "dead-lettered" {
		t.Errorf("final ruling = %q, want dead-lettered", ledger.Attempts[webhookMaxAttempts-1].Ruling)
	}
	// 1+2+4+8 = 15s of computed waiting before the DLQ.
	if ledger.TotalWaitSeconds != 15 {
		t.Errorf("totalWait = %ds, want 15", ledger.TotalWaitSeconds)
	}
	if ledger.DeadLetter == nil || !ledger.DeadLetter.Replayable || ledger.DeadLetter.Queue != "webhook-dlq" {
		t.Errorf("dead letter = %+v, want a replayable webhook-dlq record", ledger.DeadLetter)
	}
}

func TestWebhookSimulatorSigningCasesMatchTheRealGate(t *testing.T) {
	exhibit := webhookPayload().Signing
	if len(exhibit.Cases) != 4 {
		t.Fatalf("cases = %d, want 4", len(exhibit.Cases))
	}
	wantVerdicts := []bool{true, false, false, false}
	for i, tc := range exhibit.Cases {
		if tc.Verified != wantVerdicts[i] {
			t.Errorf("case %q verified = %v, want %v", tc.ID, tc.Verified, wantVerdicts[i])
		}
		if !tc.Verified && strings.TrimSpace(tc.Reason) == "" {
			t.Errorf("rejected case %q must carry the gate's reason", tc.ID)
		}
	}
	if exhibit.Cases[0].Reason != "" {
		t.Errorf("verified case reason = %q, want empty (empty is data)", exhibit.Cases[0].Reason)
	}
	// The honest case's signature on the wire must be the real HMAC.
	if exhibit.Issued.Signature != signDelivery(webhookSecret, webhookIssuedAt, webhookNonce, []byte(webhookBody)) {
		t.Error("issued signature drifted from the real HMAC")
	}
}

func TestWebhookSimulatorReceiverAnswersFastAndRefusesForgery(t *testing.T) {
	run := webhookPayload().Receiver
	if run.AnsweredStatus != 200 || run.AnsweredBody != `{"received":true}` {
		t.Errorf("honest reception = %d %q, want 200 {\"received\":true}", run.AnsweredStatus, run.AnsweredBody)
	}
	if len(run.AsyncBefore2xx) != 2 || len(run.AsyncAfter2xx) != 3 {
		t.Errorf("steps = %d before / %d after, want 2 / 3", len(run.AsyncBefore2xx), len(run.AsyncAfter2xx))
	}
	for i, step := range append(append([]ReceiverStep{}, run.AsyncBefore2xx...), run.AsyncAfter2xx...) {
		if step.Step != i+1 {
			t.Errorf("step %q numbered %d, want %d", step.Title, step.Step, i+1)
		}
	}

	// The front door itself: a tampered body flips the answer to 401 —
	// verified through the same function the honest run went through.
	headerLine := "t=" + webhookIssuedAt + ",n=" + webhookNonce +
		",v1=" + signDelivery(webhookSecret, webhookIssuedAt, webhookNonce, []byte(webhookBody))
	tampered := strings.Replace(webhookBody, "42000", "99000", 1)
	refused := receiveDelivery(webhookSecret, headerLine, parseWebhookClock(clockFresh), []byte(tampered))
	if refused.status != 401 || refused.reason == "" {
		t.Errorf("tampered reception = %d %q, want 401 with a reason", refused.status, refused.reason)
	}
	malformed := receiveDelivery(webhookSecret, "v1=only", parseWebhookClock(clockFresh), []byte(webhookBody))
	if malformed.status != 401 {
		t.Errorf("malformed-header reception = %d, want 401", malformed.status)
	}
}

func TestWebhookSimulatorConfigMatchesTheDiscipline(t *testing.T) {
	cfg := webhookPayload().Config
	if cfg.Algorithm != "HMAC-SHA256" || cfg.SignatureHeader != "X-Guide-Signature" {
		t.Errorf("algorithm/header = %q / %q", cfg.Algorithm, cfg.SignatureHeader)
	}
	if cfg.ToleranceSeconds != 300 || cfg.MaxAttempts != 5 || cfg.BaseDelaySeconds != 1 {
		t.Errorf("knobs drifted: %+v", cfg)
	}
}

func TestWebhookSimulatorPayloadIsPure(t *testing.T) {
	first, second := BuildWebhookSimulatorPayload(), BuildWebhookSimulatorPayload()
	if !reflect.DeepEqual(first, second) {
		t.Error("builder must be pure: two calls returned different payloads")
	}
}
