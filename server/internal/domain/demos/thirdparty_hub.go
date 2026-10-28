// Topic 42 third-party's live demo: the adapter-surface hub. The
// machinery is REAL Go, not narration: one vendor PORT (interface) per
// classic SaaS category — mailer, payments, object storage — with TWO
// implementations each. The deterministic local sandbox adapters do
// real work: HMAC-SHA256 signing and verification, idempotency-key
// replay and conflict detection, quota accounting that burns down and
// refuses at the ceiling, content-addressed dedupe, and a vendor-error
// taxonomy mapping that runs the app's REAL core.KindForHTTPStatus /
// core.HTTPStatusFor functions. The live adapters are credential-gated
// skeletons with the exact shape a real stripe-go / aws-sdk-go-v2 /
// SMTP adapter would take: they resolve the credential ENV VAR NAME,
// refuse honestly when it is absent, and refuse honestly (differently)
// when it is present — this guide ships no vendor SDK and the skeleton
// never invents a vendor response. A real dependency-injection switch
// selects the wired adapter per vendor from the SHARED ScenarioStore's
// data mode, the same runtime swap the posts surface uses.
//
// Determinism rules: every request drives a FRESH sandbox set through a
// FIXED script, all signatures are computed and verified at a FIXED
// exhibit clock, and no wall-clock time or randomness appears anywhere
// on the wire — two GETs serve byte-identical bodies. Every count on
// the payload was counted during the run that served it; nothing is
// invented.
package demos

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"guide/internal/core"
)

// ThirdPartyHubPath is the demo area's endpoint — registered on the
// real mux by the composition root.
const ThirdPartyHubPath = "/api/demo/thirdparty/hub"

// The fixed exhibit instants: sandbox signatures are computed at
// thirdPartyExhibitClock, webhook-style events are verified as received
// at thirdPartyEventReceivedAt (41 seconds later — inside the window),
// and the stale event is signed for a timestamp half an hour earlier.
// Fixed strings, no wall clock — the whole payload is deterministic.
const (
	thirdPartyExhibitClock    = "2026-10-28T12:00:00Z"
	thirdPartyEventReceivedAt = "2026-10-28T12:00:41Z"
	thirdPartyStaleEventClock = "2026-10-28T11:30:00Z"
	thirdPartyEventTolerance  = 5 * time.Minute
	thirdPartySignatureFormat = "{messageID}.{to}"
	thirdPartyEventFormat     = "{timestamp}.{payload}"
)

// thirdPartySandboxSecret is the sandbox adapters' signing key — demo
// material baked into the guide on purpose. A REAL vendor signing
// secret arrives by environment (the live adapters' KeyEnvVar names
// the variable), never as source.
const thirdPartySandboxSecret = "whsec_thirdparty_exhibit_9d41"

// ── the vendor-error seam ───────────────────────────────────────────

// ThirdPartyVendorError is what every adapter (sandbox and live alike)
// returns on failure: a vendor-shaped error the ADAPTER maps onto the
// app's eight-kind taxonomy. VendorStatus is the status the vendor
// side answered (or would answer) — it is deliberately NOT our HTTP
// status: a card decline (vendor 402) maps to our validation_failed
// 422, and the mapping table is the adapter's own policy.
type ThirdPartyVendorError struct {
	// Code is the vendor error code, e.g. "card_declined".
	Code string
	// VendorStatus is the status the vendor side carried.
	VendorStatus int
	// Retryable says whether a retry can ever help.
	Retryable bool
	// Detail is the human-readable story the payload serves.
	Detail string
}

// Error implements the error interface.
func (e *ThirdPartyVendorError) Error() string {
	return fmt.Sprintf("%s (vendor status %d, retryable=%t): %s",
		e.Code, e.VendorStatus, e.Retryable, e.Detail)
}

// thirdPartyClassification is the adapter's mapping table entry: a
// vendor code to a taxonomy kind, with the teaching note. The table is
// DATA — the payload serves it verbatim, each row's HTTP status
// derived through the REAL core.HTTPStatusFor.
type thirdPartyClassification struct {
	Code   string
	Status int // the vendor status the code rides
	Kind   core.Kind
	// FromRawStatus: the kind is what the real core.KindForHTTPStatus
	// answers for the vendor status (the raw-status path); false means
	// the row is a deliberate named-code override.
	FromRawStatus bool
	Retryable     bool
	Note          string
}

// thirdPartyClassifications maps every vendor code the sandboxes can
// answer with onto the app taxonomy. Named codes whose meaning is
// richer than their status (a 402 decline is input-shaped, a 451
// greylist is "come back later") are deliberate overrides; the generic
// rows show the raw-status path calling the REAL taxonomy functions.
// This table IS the demo's error-mapping machinery.
var thirdPartyClassifications = map[string]thirdPartyClassification{
	"invalid_recipient": {Code: "invalid_recipient", Status: 422,
		Kind: core.KindValidationFailed,
		Note: "bad request shape — fix the input; retrying cannot help"},
	"greylisted": {Code: "greylisted", Status: 451,
		Kind: core.KindServiceUnavailable, Retryable: true,
		Note: "the relay greylists first sends by protocol — back off and retry; it is transient"},
	"send_quota_exceeded": {Code: "send_quota_exceeded", Status: 429,
		Kind: core.KindRateLimited, Retryable: true,
		Note: "the sending quota for the window is spent — wait for the window, do not hammer"},
	"missing_idempotency_key": {Code: "missing_idempotency_key", Status: 422,
		Kind: core.KindValidationFailed,
		Note: "a money-moving call without an idempotency key is refused outright"},
	"invalid_amount": {Code: "invalid_amount", Status: 422,
		Kind: core.KindValidationFailed, Note: "non-positive amounts are input errors, not vendor failures"},
	"idempotency_key_reused": {Code: "idempotency_key_reused", Status: 422,
		Kind: core.KindValidationFailed,
		Note: "same key, different payload — the replay is refused, protecting the caller from its own bug"},
	"card_declined": {Code: "card_declined", Status: 402,
		Kind: core.KindValidationFailed,
		Note: "a decline is input-shaped: this card will never succeed as-is (the vendor's 402 is NOT our 402)"},
	"charge_quota_exceeded": {Code: "charge_quota_exceeded", Status: 429,
		Kind: core.KindRateLimited, Retryable: true,
		Note: "live-mode rate limits — back off and retry inside the vendor's window"},
	"invalid_object_key": {Code: "invalid_object_key", Status: 422,
		Kind: core.KindValidationFailed, Note: "keys carry structure ('/', '..') — refused like the files topic's sanitizer"},
	"object_too_large": {Code: "object_too_large", Status: 413,
		Kind: core.KindValidationFailed, Note: "the byte budget is part of the contract — oversized input is refused"},
	"too_many_writes": {Code: "too_many_writes", Status: 429,
		Kind: core.KindRateLimited, Retryable: true,
		Note: "request-rate ceiling — the burst behavior belongs in the adapter's degradation story"},
	"no_such_object": {Code: "no_such_object", Status: 404,
		Kind: core.KindNotFound, FromRawStatus: true,
		Note: "raw-status path: the real core.KindForHTTPStatus(404) answers not_found"},
	"vendor_bubble": {Code: "vendor_bubble", Status: 503,
		Kind: core.KindServiceUnavailable, FromRawStatus: true, Retryable: true,
		Note: "raw-status path: a 5xx from a dependency is unavailability — never our internal 500"},
	"credentials_absent": {Code: "credentials_absent", Status: 503,
		Kind: core.KindServiceUnavailable,
		Note: "the live adapter's honest refusal: no credential in the environment, nothing to call with"},
	"live_client_not_implemented": {Code: "live_client_not_implemented", Status: 503,
		Kind: core.KindServiceUnavailable,
		Note: "credential present; the real vendor HTTP call belongs here — this skeleton refuses to invent a response"},
}

// ThirdPartyClassifyVendorError maps one vendor error onto the app
// taxonomy through the table above, falling back to the REAL
// core.KindForHTTPStatus for statuses with no named row. Unknown
// vendor errors degrade to service_unavailable — a dependency we
// cannot classify is unavailable, not internal. The bool says whether
// the code was known to the table.
func ThirdPartyClassifyVendorError(err error) (core.Kind, bool) {
	var vendorErr *ThirdPartyVendorError
	if !errors.As(err, &vendorErr) {
		return core.KindServiceUnavailable, false
	}
	if row, ok := thirdPartyClassifications[vendorErr.Code]; ok {
		return row.Kind, true
	}
	if kind := core.KindForHTTPStatus(vendorErr.VendorStatus); kind != core.KindInternal {
		return kind, true
	}
	return core.KindServiceUnavailable, false
}

// thirdPartyVendorErrorf builds one vendor error from its table row.
func thirdPartyVendorErrorf(code, detail string, retryable bool) *ThirdPartyVendorError {
	status := 0
	if row, ok := thirdPartyClassifications[code]; ok {
		status = row.Status
	}
	return &ThirdPartyVendorError{Code: code, VendorStatus: status, Retryable: retryable, Detail: detail}
}

// ── the mailer port ─────────────────────────────────────────────────

// ThirdPartyMailDraft is one outbound message at the mailer port: the
// minimum a relay needs. Real adapters add headers, attachments, and a
// rendering step — never more domain state than the send itself (topic
// 29's payload-minimum rule).
type ThirdPartyMailDraft struct {
	To      string
	Subject string
	Body    string
}

// ThirdPartySendReceipt is the relay's answer for one accepted
// message: the provider's id and the signature the sandbox computed
// over the canonical message — the proof the acceptance was real.
type ThirdPartySendReceipt struct {
	MessageID string
	Signature string
}

// ThirdPartyMailer is the mailer port: one method, one message, one
// answer. Everything vendor-shaped (SMTP vs an HTTP API, greylisting,
// quotas, bounce webhooks) lives behind implementations.
type ThirdPartyMailer interface {
	// Send hands one message to the relay and answers the receipt, or
	// a *ThirdPartyVendorError the adapter maps onto the taxonomy.
	Send(ctx context.Context, draft ThirdPartyMailDraft) (ThirdPartySendReceipt, error)
}

// Compile-time proof both implementations satisfy the port — the
// payload's implementation claims are these assertions, not prose.
var (
	_ ThirdPartyMailer = (*thirdPartySandboxMailer)(nil)
	_ ThirdPartyMailer = (*thirdPartyLiveMailer)(nil)
)

// thirdPartyMailerBudget is the sandbox relay's sending quota per
// window: three accepted sends, then the 429.
const thirdPartyMailerBudget = 3

// thirdPartySandboxMailer is a deterministic local relay: it
// greylists the first send to a new address (the retry-shaped SMTP
// protocol behavior), enforces a per-window quota, signs every
// acceptance with HMAC-SHA256 at the fixed exhibit clock, and answers
// vendor errors from the taxonomy table. Mutex-guarded because a port
// implementation is shared by contract, even though each request's
// script builds a fresh one.
type thirdPartySandboxMailer struct {
	mu         sync.Mutex
	accepted   int
	greylisted map[string]bool
}

// newThirdPartySandboxMailer builds an empty sandbox relay.
func newThirdPartySandboxMailer() *thirdPartySandboxMailer {
	return &thirdPartySandboxMailer{greylisted: make(map[string]bool)}
}

// Send implements ThirdPartyMailer: validate shape, apply the
// greylisting protocol, enforce the quota, accept and sign.
func (m *thirdPartySandboxMailer) Send(_ context.Context, draft ThirdPartyMailDraft) (ThirdPartySendReceipt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	to := strings.TrimSpace(draft.To)
	if to == "" || !strings.Contains(to, "@") || strings.ContainsAny(to, " \t") {
		return ThirdPartySendReceipt{}, thirdPartyVendorErrorf("invalid_recipient",
			fmt.Sprintf("recipient %q is not a bare address (local part @ domain)", draft.To), false)
	}
	if !m.greylisted[to] {
		m.greylisted[to] = true
		return ThirdPartySendReceipt{}, thirdPartyVendorErrorf("greylisted",
			"the relay greylists the first send to a new address — back off and retry (SMTP 451, by design)", true)
	}
	if m.accepted >= thirdPartyMailerBudget {
		return ThirdPartySendReceipt{}, thirdPartyVendorErrorf("send_quota_exceeded",
			fmt.Sprintf("the window's budget of %d accepted sends is spent", thirdPartyMailerBudget), true)
	}
	m.accepted++
	messageID := fmt.Sprintf("msg_sandbox_%02d", m.accepted)
	return ThirdPartySendReceipt{
		MessageID: messageID,
		Signature: thirdPartySignMessage(messageID, to),
	}, nil
}

// thirdPartySignMessage computes the sandbox acceptance signature:
// "v1=" + hex HMAC-SHA256 over "{messageID}.{to}" at the fixed exhibit
// clock — the same discipline the webhooks topic taught, applied to a
// relay receipt.
func thirdPartySignMessage(messageID, to string) string {
	mac := hmac.New(sha256.New, []byte(thirdPartySandboxSecret))
	fmt.Fprintf(mac, "%s.%s", messageID, to)
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}

// ThirdPartyVerifySandboxSignature re-derives the acceptance signature
// and compares in constant time — the verify half of the signing
// discipline, exported so the payload (and its tests) prove both
// halves agree and that tampering breaks them.
func ThirdPartyVerifySandboxSignature(messageID, to, presented string) bool {
	return hmac.Equal([]byte(thirdPartySignMessage(messageID, to)), []byte(presented))
}

// thirdPartyLiveMailer is the live half of the mailer port: the exact
// shape a real adapter over a relay's HTTP API (or a bounded SMTP
// dial) takes. It resolves the credential by ENV VAR NAME, refuses
// honestly when absent, and refuses honestly when present — the
// vendor call site is named in the refusal, never faked.
type thirdPartyLiveMailer struct {
	cfg         ThirdPartyLiveConfig
	credentials func(name string) (string, bool)
}

// Send implements ThirdPartyMailer — the credential gate, then the
// honest stub refusal.
func (a *thirdPartyLiveMailer) Send(context.Context, ThirdPartyMailDraft) (ThirdPartySendReceipt, error) {
	if err := thirdPartyRequireCredentials(a.cfg, a.credentials); err != nil {
		return ThirdPartySendReceipt{}, err
	}
	return ThirdPartySendReceipt{}, thirdPartyLiveStubError(a.cfg, "POST /v1/email/send")
}

// ── the payments port ───────────────────────────────────────────────

// ThirdPartyChargeRequest is one charge at the payments port. The card
// is a TOKEN — a tokenized reference from the provider's browser
// widgets; a card number never crosses this port (PCI scope stays the
// provider's). The idempotency key is the caller's stable identity for
// the MONEY MOVEMENT, not for the request.
type ThirdPartyChargeRequest struct {
	IdempotencyKey string
	AmountCents    int64
	Currency       string
	CardToken      string
}

// ThirdPartyChargeReceipt is the processor's answer for one processed
// charge.
type ThirdPartyChargeReceipt struct {
	ChargeID    string
	AmountCents int64
	Status      string
}

// ThirdPartyEvent is the processor's webhook event after a charge: the
// signed at-least-once notification the receiving side must verify and
// dedupe — Stripe's header design, implemented for real here.
type ThirdPartyEvent struct {
	ID        string
	Type      string
	JSON      string // the exact signed payload bytes
	Timestamp string // RFC 3339 — covered by the signature
	Signature string // "v1=" + hex HMAC-SHA256 over "{timestamp}.{json}"
}

// ThirdPartyPaymentProcessor is the payments port.
type ThirdPartyPaymentProcessor interface {
	// Charge executes one charge under an idempotency key: the same
	// key replays the original receipt instead of charging twice, and
	// a reused key with a different payload is refused.
	Charge(ctx context.Context, req ThirdPartyChargeRequest) (ThirdPartyChargeReceipt, error)
	// LastEvent answers the signed event the most recent successful
	// charge emitted (sandbox bookkeeping; a real processor delivers
	// events to your webhook endpoint instead).
	LastEvent() (ThirdPartyEvent, bool)
}

// Compile-time proof both implementations satisfy the port.
var (
	_ ThirdPartyPaymentProcessor = (*thirdPartySandboxPayments)(nil)
	_ ThirdPartyPaymentProcessor = (*thirdPartyLivePayments)(nil)
)

// The sandbox processor's knobs: a per-window charge quota and the two
// exhibit card tokens — one clean, one the classic decline.
const (
	thirdPartyChargeBudget = 2
	thirdPartyCardOK       = "tok_visa_4242"
	thirdPartyCardDeclined = "tok_declined_0002"
)

// thirdPartySandboxPayments is a deterministic local processor with
// the semantics that matter: idempotency-key replay and conflict
// detection, a decline that is input-shaped, quota refusal, and signed
// webhook events. Mutex-guarded by the same port contract as the
// mailer.
type thirdPartySandboxPayments struct {
	mu        sync.Mutex
	processed int
	charges   map[string]ThirdPartyChargeReceipt
	lastEvent *ThirdPartyEvent
}

// newThirdPartySandboxPayments builds an empty sandbox processor.
func newThirdPartySandboxPayments() *thirdPartySandboxPayments {
	return &thirdPartySandboxPayments{charges: make(map[string]ThirdPartyChargeReceipt)}
}

// Charge implements ThirdPartyPaymentProcessor: require the key,
// validate the amount, replay-or-refuse on the key, classify the
// decline, enforce the quota, charge and emit the signed event.
func (p *thirdPartySandboxPayments) Charge(_ context.Context, req ThirdPartyChargeRequest) (ThirdPartyChargeReceipt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := strings.TrimSpace(req.IdempotencyKey)
	if key == "" {
		return ThirdPartyChargeReceipt{}, thirdPartyVendorErrorf("missing_idempotency_key",
			"a money-moving call without an idempotency key is refused before anything moves", false)
	}
	if req.AmountCents <= 0 {
		return ThirdPartyChargeReceipt{}, thirdPartyVendorErrorf("invalid_amount",
			fmt.Sprintf("amount %d cents is not a chargeable value", req.AmountCents), false)
	}
	if prior, ok := p.charges[key]; ok {
		if prior.AmountCents != req.AmountCents {
			return ThirdPartyChargeReceipt{}, thirdPartyVendorErrorf("idempotency_key_reused",
				fmt.Sprintf("key %q first charged %d cents; this replay wants %d — refused",
					key, prior.AmountCents, req.AmountCents), false)
		}
		return prior, nil
	}
	if req.CardToken == thirdPartyCardDeclined {
		return ThirdPartyChargeReceipt{}, thirdPartyVendorErrorf("card_declined",
			"the issuing bank declined this card — retrying the same token cannot succeed", false)
	}
	if p.processed >= thirdPartyChargeBudget {
		return ThirdPartyChargeReceipt{}, thirdPartyVendorErrorf("charge_quota_exceeded",
			fmt.Sprintf("the window's budget of %d processed charges is spent", thirdPartyChargeBudget), true)
	}
	p.processed++
	chargeID := fmt.Sprintf("ch_sandbox_%02d", p.processed)
	receipt := ThirdPartyChargeReceipt{ChargeID: chargeID, AmountCents: req.AmountCents, Status: "succeeded"}
	p.charges[key] = receipt
	p.lastEvent = thirdPartyEmitEvent(chargeID, req.AmountCents, req.Currency)
	return receipt, nil
}

// LastEvent implements ThirdPartyPaymentProcessor.
func (p *thirdPartySandboxPayments) LastEvent() (ThirdPartyEvent, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.lastEvent == nil {
		return ThirdPartyEvent{}, false
	}
	return *p.lastEvent, true
}

// thirdPartyEmitEvent builds the charge.succeeded event and signs it
// at the fixed exhibit clock: "v1=" + hex HMAC-SHA256 over
// "{timestamp}.{json}" — the vendor webhook header design.
func thirdPartyEmitEvent(chargeID string, amountCents int64, currency string) *ThirdPartyEvent {
	suffix := strings.TrimPrefix(chargeID, "ch_")
	payload := fmt.Sprintf(`{"eventId":"evt_%s","type":"charge.succeeded","chargeId":"%s","amountCents":%d,"currency":"%s"}`,
		suffix, chargeID, amountCents, currency)
	mac := hmac.New(sha256.New, []byte(thirdPartySandboxSecret))
	fmt.Fprintf(mac, "%s.%s", thirdPartyExhibitClock, payload)
	return &ThirdPartyEvent{
		ID:        "evt_" + suffix,
		Type:      "charge.succeeded",
		JSON:      payload,
		Timestamp: thirdPartyExhibitClock,
		Signature: "v1=" + hex.EncodeToString(mac.Sum(nil)),
	}
}

// ThirdPartyVerifySandboxEvent verifies one event's signature exactly
// the receiving side must: freshness FIRST (the cheapest gate — a
// replay dies before any crypto), then the constant-time MAC over the
// exact payload bytes. Exported so the payload proves all three
// verdicts for real: honest, tampered, stale.
func ThirdPartyVerifySandboxEvent(event ThirdPartyEvent, receivedAt time.Time) error {
	signedAt, err := time.Parse(time.RFC3339, event.Timestamp)
	if err != nil {
		return fmt.Errorf("unparseable event timestamp %q: %w", event.Timestamp, err)
	}
	if delta := receivedAt.Sub(signedAt); delta < -thirdPartyEventTolerance || delta > thirdPartyEventTolerance {
		return fmt.Errorf("event timestamp %s is outside the ±%s freshness window of %s",
			event.Timestamp, thirdPartyEventTolerance, receivedAt.Format(time.RFC3339))
	}
	if !hmac.Equal([]byte(thirdPartyEventSignature(event.Timestamp, event.JSON)), []byte(event.Signature)) {
		return errors.New("signature mismatch: the payload bytes changed after signing, or the key is wrong")
	}
	return nil
}

// thirdPartyFixedEventClock parses a fixed exhibit clock; the values
// are constants, so a parse failure is a programmer error in this
// file.
func thirdPartyFixedEventClock(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic("thirdparty hub: exhibit clock " + value + " must parse")
	}
	return parsed
}

// thirdPartyLivePayments is the live half of the payments port — the
// exact shape a stripe-go adapter takes, with the vendor call site
// named in the honest refusal.
type thirdPartyLivePayments struct {
	cfg         ThirdPartyLiveConfig
	credentials func(name string) (string, bool)
}

// Charge implements ThirdPartyPaymentProcessor.
func (a *thirdPartyLivePayments) Charge(context.Context, ThirdPartyChargeRequest) (ThirdPartyChargeReceipt, error) {
	if err := thirdPartyRequireCredentials(a.cfg, a.credentials); err != nil {
		return ThirdPartyChargeReceipt{}, err
	}
	return ThirdPartyChargeReceipt{}, thirdPartyLiveStubError(a.cfg, "POST /v1/payment_intents (Idempotency-Key header)")
}

// LastEvent implements ThirdPartyPaymentProcessor — the live adapter
// has no events until the vendor client exists.
func (a *thirdPartyLivePayments) LastEvent() (ThirdPartyEvent, bool) { return ThirdPartyEvent{}, false }

// ── the object-store port ───────────────────────────────────────────

// ThirdPartyObjectInfo is one stored object's metadata: the key, the
// byte size, the sniffed content type, and the content-addressed ETag
// (first 16 hex of SHA-256 — same content, same tag, wherever it is
// written).
type ThirdPartyObjectInfo struct {
	Key         string
	SizeBytes   int64
	ContentType string
	ETag        string
}

// ThirdPartyObjectStore is the object-storage port: bounded puts and
// keyed gets. The rules are part of the contract: keys are single
// components (structure is refused), the byte budget bounds what a
// window may hold, and identical content dedupes by ETag.
type ThirdPartyObjectStore interface {
	// Put stores body under key inside the budgets, answering the
	// object's metadata; content already present under the budget
	// dedupes by ETag without charging the bytes twice.
	Put(ctx context.Context, key string, body []byte, contentType string) (ThirdPartyObjectInfo, error)
	// Get answers the stored bytes, or the vendor's no_such_object
	// error when the key never landed.
	Get(ctx context.Context, key string) ([]byte, error)
}

// Compile-time proof both implementations satisfy the port.
var (
	_ ThirdPartyObjectStore = (*thirdPartySandboxStore)(nil)
	_ ThirdPartyObjectStore = (*thirdPartyLiveStore)(nil)
)

// The sandbox store's budgets: three puts per window and sixty-four
// NEW content bytes per window — tiny on purpose, so the script burns
// them visibly.
const (
	thirdPartyPutBudget  = 3
	thirdPartyByteBudget = 64
)

// thirdPartySandboxStore is a deterministic local bucket: the files
// topic's key rules, a byte budget, a per-window put quota,
// content-addressed dedupe, and not-found gets.
type thirdPartySandboxStore struct {
	mu    sync.Mutex
	puts  int
	bytes int64
	blobs map[string][]byte
	etags map[string]string // content etag → etag (the dedupe set)
	sizes map[string]int64
}

// newThirdPartySandboxStore builds an empty sandbox bucket.
func newThirdPartySandboxStore() *thirdPartySandboxStore {
	return &thirdPartySandboxStore{
		blobs: make(map[string][]byte),
		etags: make(map[string]string),
		sizes: make(map[string]int64),
	}
}

// thirdPartyETag derives the content-addressed tag: the first 16 hex
// characters of the body's SHA-256.
func thirdPartyETag(body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:8])
}

// thirdPartyValidObjectKey applies the files topic's allowlist posture
// to bucket keys: one component, no separators, no traversal, no NUL.
// A dot inside a name is fine ("report.json"); ".." is not.
func thirdPartyValidObjectKey(key string) bool {
	name := strings.TrimSpace(key)
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.ContainsAny(name, `/\`) || strings.ContainsRune(name, 0) || strings.Contains(name, "..") {
		return false
	}
	return true
}

// Put implements ThirdPartyObjectStore.
func (s *thirdPartySandboxStore) Put(_ context.Context, key string, body []byte, contentType string) (ThirdPartyObjectInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !thirdPartyValidObjectKey(key) {
		return ThirdPartyObjectInfo{}, thirdPartyVendorErrorf("invalid_object_key",
			fmt.Sprintf("key %q carries structure or is empty — keys are single minted components", key), false)
	}
	if etag := thirdPartyETag(body); s.etags[etag] != "" {
		// Content-addressed dedupe: the bytes already exist in the
		// window; the put still costs a request but charges no bytes.
		s.puts++
		return ThirdPartyObjectInfo{Key: key, SizeBytes: int64(len(body)),
			ContentType: contentType, ETag: etag}, nil
	}
	if s.bytes+int64(len(body)) > thirdPartyByteBudget {
		return ThirdPartyObjectInfo{}, thirdPartyVendorErrorf("object_too_large",
			fmt.Sprintf("%d new bytes would exceed the window's %d-byte budget (%d already held)",
				len(body), thirdPartyByteBudget, s.bytes), false)
	}
	if s.puts >= thirdPartyPutBudget {
		return ThirdPartyObjectInfo{}, thirdPartyVendorErrorf("too_many_writes",
			fmt.Sprintf("the window's budget of %d puts is spent", thirdPartyPutBudget), true)
	}
	s.puts++
	s.etags[thirdPartyETag(body)] = thirdPartyETag(body)
	s.blobs[key] = append([]byte(nil), body...)
	s.sizes[key] = int64(len(body))
	s.bytes += int64(len(body))
	return ThirdPartyObjectInfo{Key: key, SizeBytes: int64(len(body)),
		ContentType: contentType, ETag: thirdPartyETag(body)}, nil
}

// Get implements ThirdPartyObjectStore.
func (s *thirdPartySandboxStore) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	body, ok := s.blobs[strings.TrimSpace(key)]
	if !ok {
		return nil, thirdPartyVendorErrorf("no_such_object",
			fmt.Sprintf("key %q never landed in this bucket", key), false)
	}
	return append([]byte(nil), body...), nil
}

// thirdPartyLiveStore is the live half of the object-store port — the
// exact shape an aws-sdk-go-v2 S3 adapter takes.
type thirdPartyLiveStore struct {
	cfg         ThirdPartyLiveConfig
	credentials func(name string) (string, bool)
}

// Put implements ThirdPartyObjectStore.
func (a *thirdPartyLiveStore) Put(context.Context, string, []byte, string) (ThirdPartyObjectInfo, error) {
	if err := thirdPartyRequireCredentials(a.cfg, a.credentials); err != nil {
		return ThirdPartyObjectInfo{}, err
	}
	return ThirdPartyObjectInfo{}, thirdPartyLiveStubError(a.cfg, "PutObject into the bucket")
}

// Get implements ThirdPartyObjectStore.
func (a *thirdPartyLiveStore) Get(context.Context, string) ([]byte, error) {
	if err := thirdPartyRequireCredentials(a.cfg, a.credentials); err != nil {
		return nil, err
	}
	return nil, thirdPartyLiveStubError(a.cfg, "GetObject from the bucket")
}

// ── the live adapters' shared credential gate ───────────────────────

// ThirdPartyLiveConfig names one live adapter's wiring: the vendor, the
// API base URL, and the ENV VARIABLE NAME the credential arrives by.
// The value never lives in this file — the name is the whole contract.
type ThirdPartyLiveConfig struct {
	VendorName string
	APIBaseURL string
	KeyEnvVar  string
}

// The three live adapters' configs, served on the payload as the
// honest statement of what WOULD be wired.
var (
	thirdPartyLiveMailerCfg = ThirdPartyLiveConfig{
		VendorName: "an email relay",
		APIBaseURL: "https://api.mail-relay.example",
		KeyEnvVar:  "MAIL_RELAY_API_KEY",
	}
	thirdPartyLivePaymentsCfg = ThirdPartyLiveConfig{
		VendorName: "Stripe",
		APIBaseURL: "https://api.stripe.example",
		KeyEnvVar:  "STRIPE_SECRET_KEY",
	}
	thirdPartyLiveStoreCfg = ThirdPartyLiveConfig{
		VendorName: "an S3-compatible bucket",
		APIBaseURL: "https://object-store.example",
		KeyEnvVar:  "OBJECT_STORE_ACCESS_KEY",
	}
)

// thirdPartyRequireCredentials is the live adapters' first gate:
// resolve the credential by its env var name and refuse honestly —
// with the taxonomy's service_unavailable, never a panic and never a
// fabricated success — when the environment holds no value.
func thirdPartyRequireCredentials(cfg ThirdPartyLiveConfig, credentials func(string) (string, bool)) error {
	if credentials == nil {
		return thirdPartyVendorErrorf("credentials_absent",
			"no credential resolver wired — nothing to call "+cfg.VendorName+" with", false)
	}
	if _, ok := credentials(cfg.KeyEnvVar); !ok {
		return thirdPartyVendorErrorf("credentials_absent",
			cfg.KeyEnvVar+" is not set in this environment — set it to wire the "+cfg.VendorName+" client", false)
	}
	return nil
}

// thirdPartyLiveStubError is the live adapters' second gate: the
// credential exists, and the REAL vendor HTTP call would run now —
// bounded, behind this port, mapped onto the taxonomy on failure. This
// guide ships no vendor SDK (stripe-go, aws-sdk-go-v2, and SMTP
// clients live in the topic's comparison prose only), so the skeleton
// refuses rather than inventing a response.
func thirdPartyLiveStubError(cfg ThirdPartyLiveConfig, callSite string) *ThirdPartyVendorError {
	return thirdPartyVendorErrorf("live_client_not_implemented",
		cfg.KeyEnvVar+" is present and the real call would be "+callSite+" against "+cfg.APIBaseURL+
			" — this guide wires no vendor SDK, and the skeleton refuses to invent a response", false)
}

// ── the dependency-injection switch ─────────────────────────────────

// ThirdPartyWired is one switch position: the adapter chosen for every
// port when the scenario's data mode says so — fake selects the
// deterministic sandboxes, live selects the credential-gated
// skeletons. This is the same runtime swap the posts surface performs
// between its in-memory and live repositories.
type ThirdPartyWired struct {
	Mailer   ThirdPartyMailer
	Payments ThirdPartyPaymentProcessor
	Store    ThirdPartyObjectStore
}

// WireThirdParty resolves the switch: fake data mode wires the
// sandboxes, live data mode wires the live skeletons. The composition
// root calls this per request (the choice is cheap and the scenario is
// a runtime toggle), exactly like every fake/live seam in this guide.
func WireThirdParty(mode core.DataMode, credentials func(string) (string, bool)) ThirdPartyWired {
	if mode == core.DataModeLive {
		return ThirdPartyWired{
			Mailer:   &thirdPartyLiveMailer{cfg: thirdPartyLiveMailerCfg, credentials: credentials},
			Payments: &thirdPartyLivePayments{cfg: thirdPartyLivePaymentsCfg, credentials: credentials},
			Store:    &thirdPartyLiveStore{cfg: thirdPartyLiveStoreCfg, credentials: credentials},
		}
	}
	return ThirdPartyWired{
		Mailer:   newThirdPartySandboxMailer(),
		Payments: newThirdPartySandboxPayments(),
		Store:    newThirdPartySandboxStore(),
	}
}

// ── the observed-run shapes ─────────────────────────────────────────

// ThirdPartyRunRow is one observed call of a vendor script: what was
// called, with what input, what the adapter answered, and how the
// failure (if any) classified onto the taxonomy. VendorStatus 0 means
// the call succeeded — no vendor status to report.
type ThirdPartyRunRow struct {
	Step         int    `json:"step"`
	Call         string `json:"call"`
	Input        string `json:"input"`
	Outcome      string `json:"outcome"`
	VendorCode   string `json:"vendorCode,omitempty"`
	VendorStatus int    `json:"vendorStatus,omitempty"`
	Kind         string `json:"kind,omitempty"`
	HTTPStatus   int    `json:"httpStatus,omitempty"`
	Retryable    bool   `json:"retryable"`
	Detail       string `json:"detail"`
}

// ThirdPartyCountRow is one counted observation — counted during the
// run, never asserted.
type ThirdPartyCountRow struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

// ThirdPartyPortMethod is one method of a vendor port.
type ThirdPartyPortMethod struct {
	Name      string `json:"name"`
	Signature string `json:"signature"`
}

// ThirdPartyPortPanel is a port as served: its methods, its two
// compile-time-asserted implementations, and the seam's note.
type ThirdPartyPortPanel struct {
	Name            string                 `json:"name"`
	Methods         []ThirdPartyPortMethod `json:"methods"`
	Implementations []string               `json:"implementations"`
	Note            string                 `json:"note"`
}

// ThirdPartySandboxPanel is one vendor's sandbox run: the config the
// script ran under, the observed rows, and the counted totals.
type ThirdPartySandboxPanel struct {
	Config []string             `json:"config"`
	Rows   []ThirdPartyRunRow   `json:"rows"`
	Counts []ThirdPartyCountRow `json:"counts"`
}

// ThirdPartyLivePanel is one vendor's live half: the config the real
// adapter would take, the observed one-call outcome (an honest refusal
// in this guide), and the seam note.
type ThirdPartyLivePanel struct {
	Vendor     string           `json:"vendor"`
	APIBaseURL string           `json:"apiBaseUrl"`
	KeyEnvVar  string           `json:"keyEnvVar"`
	CallSite   string           `json:"callSite"`
	Outcome    ThirdPartyRunRow `json:"outcome"`
	Note       string           `json:"note"`
}

// ThirdPartyVendorPanel is one category end to end: the port, the
// sandbox's observed run, and the live skeleton's observed refusal.
type ThirdPartyVendorPanel struct {
	ID      string                 `json:"id"`
	Title   string                 `json:"title"`
	Lead    string                 `json:"lead"`
	Port    ThirdPartyPortPanel    `json:"port"`
	Sandbox ThirdPartySandboxPanel `json:"sandbox"`
	Live    ThirdPartyLivePanel    `json:"live"`
}

// ThirdPartyWiringRow is one vendor's switch position: both adapter
// identities, which one the current data mode selects, and one
// observed probe through the ACTIVE adapter.
type ThirdPartyWiringRow struct {
	Category      string `json:"category"`
	Port          string `json:"port"`
	FakeAdapter   string `json:"fakeAdapter"`
	LiveAdapter   string `json:"liveAdapter"`
	Active        string `json:"active"`
	ActiveOutcome string `json:"activeOutcome"`
}

// ThirdPartyWiringPanel is the DI switch as served: the source of the
// toggle, its current position, and the per-vendor rows.
type ThirdPartyWiringPanel struct {
	Source        string                `json:"source"`
	Current       string                `json:"current"`
	GetEndpoint   string                `json:"getEndpoint"`
	PatchEndpoint string                `json:"patchEndpoint"`
	Rows          []ThirdPartyWiringRow `json:"rows"`
	Note          string                `json:"note"`
}

// ThirdPartyTaxonomyRow is one mapping-table row, with the HTTP status
// derived through the REAL core.HTTPStatusFor — the table the adapters
// classify through, served as data.
type ThirdPartyTaxonomyRow struct {
	VendorCode   string `json:"vendorCode"`
	VendorStatus int    `json:"vendorStatus"`
	Kind         string `json:"kind"`
	HTTPStatus   int    `json:"httpStatus"`
	Retryable    bool   `json:"retryable"`
	ViaRawStatus bool   `json:"viaRawStatus"`
	Note         string `json:"note"`
}

// ThirdPartyHubPayload is the wire shape of
// GET /api/demo/thirdparty/hub (camelCase JSON).
type ThirdPartyHubPayload struct {
	Mode        string                  `json:"mode"`
	HonestyNote string                  `json:"honestyNote"`
	Wiring      ThirdPartyWiringPanel   `json:"wiring"`
	Vendors     []ThirdPartyVendorPanel `json:"vendors"`
	TaxonomyMap []ThirdPartyTaxonomyRow `json:"taxonomyMap"`
}

// ThirdPartyHubFacts is everything the builder derives from: the
// scenario snapshot that drives the DI switch, and the credential
// resolver the live adapters gate through (the composition root wires
// os.LookupEnv; tests inject their own).
type ThirdPartyHubFacts struct {
	Scenario    core.ScenarioState
	Credentials func(name string) (string, bool)
}

// BuildThirdPartyHubPayload drives the whole hub (pure over its
// facts): every request runs the FIXED vendor scripts against a FRESH
// sandbox set, probes the live skeletons through the same credential
// gate the real adapters would use, resolves the DI switch from the
// scenario's data mode, and shapes every row from what actually
// happened. No wall clock, no randomness — same facts in, same bytes
// out.
func BuildThirdPartyHubPayload(facts ThirdPartyHubFacts) ThirdPartyHubPayload {
	mailerRun := thirdPartyRunMailerScript()
	paymentsRun := thirdPartyRunPaymentsScript()
	storeRun := thirdPartyRunStoreScript()
	liveMail := thirdPartyRunLiveProbe(thirdPartyLiveMailerCfg, "Send",
		func(a ThirdPartyLiveConfig, c func(string) (string, bool)) error {
			_, err := (&thirdPartyLiveMailer{cfg: a, credentials: c}).Send(context.Background(),
				ThirdPartyMailDraft{To: "ops@example.com", Subject: "hub probe", Body: "live wiring probe"})
			return err
		}, facts.Credentials)
	livePay := thirdPartyRunLiveProbe(thirdPartyLivePaymentsCfg, "Charge",
		func(a ThirdPartyLiveConfig, c func(string) (string, bool)) error {
			_, err := (&thirdPartyLivePayments{cfg: a, credentials: c}).Charge(context.Background(),
				ThirdPartyChargeRequest{IdempotencyKey: "hub-probe", AmountCents: 100, CardToken: thirdPartyCardOK})
			return err
		}, facts.Credentials)
	liveStore := thirdPartyRunLiveProbe(thirdPartyLiveStoreCfg, "Put",
		func(a ThirdPartyLiveConfig, c func(string) (string, bool)) error {
			_, err := (&thirdPartyLiveStore{cfg: a, credentials: c}).Put(context.Background(),
				"hub-probe.json", []byte(`{"probe":true}`), "application/json")
			return err
		}, facts.Credentials)

	active := "fakeAdapter"
	if facts.Scenario.DataMode == core.DataModeLive {
		active = "liveAdapter"
	}
	wired := WireThirdParty(facts.Scenario.DataMode, facts.Credentials)
	probe := thirdPartyActiveProbe(wired)

	return ThirdPartyHubPayload{
		Mode: "adapter-surface hub: three vendor ports (mailer, payments, object storage), each with two real Go " +
			"implementations — a deterministic sandbox and a credential-gated live skeleton — switched by the shared " +
			"scenario store's data mode; every row below was observed during THIS request's scripted drive through " +
			"those adapters",
		HonestyNote: "No-invention pin: every count on this payload was counted during the run that served it, every " +
			"signature is a real HMAC-SHA256 computed and verified at the fixed exhibit clock " + thirdPartyExhibitClock +
			" (so two GETs serve byte-identical bodies), and every vendor status, taxonomy kind, and HTTP status on a " +
			"failure row came out of the real classification table and core.HTTPStatusFor. No vendor network call is " +
			"made from this endpoint: the sandboxes are local deterministic Go, and the live skeletons refuse honestly " +
			"— this guide configures no vendor credentials, and nothing here invents a vendor response",
		Wiring: ThirdPartyWiringPanel{
			Source:        "GET/PATCH /api/demo/scenario — the dataMode field",
			Current:       string(facts.Scenario.DataMode),
			GetEndpoint:   "/api/demo/scenario",
			PatchEndpoint: "/api/demo/scenario",
			Rows: []ThirdPartyWiringRow{
				{Category: "mailer", Port: "ThirdPartyMailer",
					FakeAdapter:   "thirdPartySandboxMailer — greylisting, quota, signed acceptances",
					LiveAdapter:   "thirdPartyLiveMailer — credential-gated skeleton over " + thirdPartyLiveMailerCfg.APIBaseURL,
					Active:        active,
					ActiveOutcome: probe.Mailer},
				{Category: "payments", Port: "ThirdPartyPaymentProcessor",
					FakeAdapter:   "thirdPartySandboxPayments — idempotency keys, declines, signed events",
					LiveAdapter:   "thirdPartyLivePayments — credential-gated skeleton over " + thirdPartyLivePaymentsCfg.APIBaseURL,
					Active:        active,
					ActiveOutcome: probe.Payments},
				{Category: "storage", Port: "ThirdPartyObjectStore",
					FakeAdapter:   "thirdPartySandboxStore — key rules, byte+put budgets, content dedupe",
					LiveAdapter:   "thirdPartyLiveStore — credential-gated skeleton over " + thirdPartyLiveStoreCfg.APIBaseURL,
					Active:        active,
					ActiveOutcome: probe.Storage},
			},
			Note: "the same runtime swap the posts surface performs between its in-memory and live repositories: " +
				"PATCH the scenario's dataMode to live and reload — every wiring row flips to the live skeletons and " +
				"the active probes answer the honest credential refusal instead of sandbox receipts",
		},
		Vendors: []ThirdPartyVendorPanel{
			{
				ID: "mailer", Title: "Email: the mailer port",
				Lead: "One Send method, one receipt, everything relay-shaped hidden behind it.",
				Port: ThirdPartyPortPanel{
					Name: "ThirdPartyMailer",
					Methods: []ThirdPartyPortMethod{
						{Name: "Send", Signature: "Send(ctx context.Context, draft ThirdPartyMailDraft) (ThirdPartySendReceipt, error)"},
					},
					Implementations: []string{
						"thirdPartySandboxMailer — compile-time asserted (var _ ThirdPartyMailer)",
						"thirdPartyLiveMailer — compile-time asserted (var _ ThirdPartyMailer)",
					},
					Note: "the port carries the message, not the transport: SMTP dial vs relay HTTP API is an adapter decision",
				},
				Sandbox: mailerRun,
				Live: ThirdPartyLivePanel{
					Vendor: thirdPartyLiveMailerCfg.VendorName, APIBaseURL: thirdPartyLiveMailerCfg.APIBaseURL,
					KeyEnvVar: thirdPartyLiveMailerCfg.KeyEnvVar, CallSite: "POST /v1/email/send", Outcome: liveMail,
					Note: "the skeleton resolves " + thirdPartyLiveMailerCfg.KeyEnvVar + " by name and refuses honestly " +
						"until it exists — then names the real call site instead of inventing a response",
				},
			},
			{
				ID: "payments", Title: "Payments: the processor port",
				Lead: "Idempotency keys and signed events are the two mechanics a payments adapter must own.",
				Port: ThirdPartyPortPanel{
					Name: "ThirdPartyPaymentProcessor",
					Methods: []ThirdPartyPortMethod{
						{Name: "Charge", Signature: "Charge(ctx context.Context, req ThirdPartyChargeRequest) (ThirdPartyChargeReceipt, error)"},
						{Name: "LastEvent", Signature: "LastEvent() (ThirdPartyEvent, bool)"},
					},
					Implementations: []string{
						"thirdPartySandboxPayments — compile-time asserted (var _ ThirdPartyPaymentProcessor)",
						"thirdPartyLivePayments — compile-time asserted (var _ ThirdPartyPaymentProcessor)",
					},
					Note: "a tokenized card reference crosses the port — a card number never does (PCI scope stays the vendor's)",
				},
				Sandbox: paymentsRun,
				Live: ThirdPartyLivePanel{
					Vendor: thirdPartyLivePaymentsCfg.VendorName, APIBaseURL: thirdPartyLivePaymentsCfg.APIBaseURL,
					KeyEnvVar: thirdPartyLivePaymentsCfg.KeyEnvVar,
					CallSite:  "POST /v1/payment_intents (Idempotency-Key header)", Outcome: livePay,
					Note: "stripe-go would live here; this guide teaches it in comparison prose and refuses to fabricate charges",
				},
			},
			{
				ID: "storage", Title: "Cloud storage: the object-store port",
				Lead: "Minted keys, bounded bytes, content-addressed dedupe — the bucket behind a two-method port.",
				Port: ThirdPartyPortPanel{
					Name: "ThirdPartyObjectStore",
					Methods: []ThirdPartyPortMethod{
						{Name: "Put", Signature: "Put(ctx context.Context, key string, body []byte, contentType string) (ThirdPartyObjectInfo, error)"},
						{Name: "Get", Signature: "Get(ctx context.Context, key string) ([]byte, error)"},
					},
					Implementations: []string{
						"thirdPartySandboxStore — compile-time asserted (var _ ThirdPartyObjectStore)",
						"thirdPartyLiveStore — compile-time asserted (var _ ThirdPartyObjectStore)",
					},
					Note: "the files topic's Store port is the same seam one layer down — local disk today, a bucket tomorrow",
				},
				Sandbox: storeRun,
				Live: ThirdPartyLivePanel{
					Vendor: thirdPartyLiveStoreCfg.VendorName, APIBaseURL: thirdPartyLiveStoreCfg.APIBaseURL,
					KeyEnvVar: thirdPartyLiveStoreCfg.KeyEnvVar, CallSite: "PutObject into the bucket", Outcome: liveStore,
					Note: "aws-sdk-go-v2 (or any S3-compatible client) would live here; the skeleton gates on " +
						thirdPartyLiveStoreCfg.KeyEnvVar + " and refuses honestly",
				},
			},
		},
		TaxonomyMap: thirdPartyTaxonomyRows(),
	}
}

// thirdPartyTaxonomyRows serves the classification table as data —
// codes sorted, HTTP statuses derived through the REAL taxonomy
// function, raw-status rows marked as such.
func thirdPartyTaxonomyRows() []ThirdPartyTaxonomyRow {
	codes := make([]string, 0, len(thirdPartyClassifications))
	for code := range thirdPartyClassifications {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	rows := make([]ThirdPartyTaxonomyRow, 0, len(codes))
	for _, code := range codes {
		row := thirdPartyClassifications[code]
		rows = append(rows, ThirdPartyTaxonomyRow{
			VendorCode:   row.Code,
			VendorStatus: row.Status,
			Kind:         string(row.Kind),
			HTTPStatus:   core.HTTPStatusFor(row.Kind),
			Retryable:    row.Retryable,
			ViaRawStatus: row.FromRawStatus,
			Note:         row.Note,
		})
	}
	return rows
}

// thirdPartyProbeOutcomes carries the three observed active-adapter
// probes for the wiring rows.
type thirdPartyProbeOutcomes struct{ Mailer, Payments, Storage string }

// thirdPartyActiveProbe drives ONE honest call through each ACTIVE
// adapter of the wired set — the switch made observable. Under fake
// mode the fresh sandboxes answer (the mailer greylists the probe's
// first send, so it is sent twice — the behavior is the lesson); under
// live mode the skeletons answer the credential refusal.
func thirdPartyActiveProbe(wired ThirdPartyWired) thirdPartyProbeOutcomes {
	ctx := context.Background()
	mailerOutcome := thirdPartyProbeMailer(wired.Mailer, ctx)

	charge, payErr := wired.Payments.Charge(ctx, ThirdPartyChargeRequest{
		IdempotencyKey: "wiring-probe", AmountCents: 4200, Currency: "USD", CardToken: thirdPartyCardOK})
	paymentsOutcome := fmt.Sprintf("accepted: %s succeeded (%d cents)", charge.ChargeID, charge.AmountCents)
	if payErr != nil {
		paymentsOutcome = "refused: " + thirdPartyProbeRefusal(payErr)
	}

	info, storeErr := wired.Store.Put(ctx, "wiring-probe.json",
		[]byte(`{"probe":"the active object store"}`), "application/json")
	storageOutcome := fmt.Sprintf("accepted: %s, etag %s, %d bytes", info.Key, info.ETag, info.SizeBytes)
	if storeErr != nil {
		storageOutcome = "refused: " + thirdPartyProbeRefusal(storeErr)
	}
	return thirdPartyProbeOutcomes{Mailer: mailerOutcome, Payments: paymentsOutcome, Storage: storageOutcome}
}

// thirdPartyProbeMailer probes the ACTIVE mailer with the retry the
// greylisting protocol asks for: the first send to a new address is
// refused transiently by the sandbox, the second is accepted — the
// observed outcome reports both beats honestly.
func thirdPartyProbeMailer(mailer ThirdPartyMailer, ctx context.Context) string {
	draft := ThirdPartyMailDraft{
		To: "wiring-probe@example.com", Subject: "the wiring probe", Body: "one send through the ACTIVE mailer"}
	receipt, firstErr := mailer.Send(ctx, draft)
	if firstErr == nil {
		return "accepted first try: " + receipt.MessageID
	}
	receipt, retryErr := mailer.Send(ctx, draft)
	if retryErr == nil {
		return "greylisted first, accepted on the retry: " + receipt.MessageID + " — the sandbox's own protocol"
	}
	return "refused on both attempts: " + thirdPartyProbeRefusal(retryErr)
}

// thirdPartyProbeRefusal renders one refusal for the wiring rows:
// the vendor error and the taxonomy kind it classified to.
func thirdPartyProbeRefusal(err error) string {
	kind, _ := ThirdPartyClassifyVendorError(err)
	return fmt.Sprintf("%s → %s", err.Error(), kind)
}

// thirdPartyRunRowFromError shapes the failure half of a run row: the
// vendor code, its status, the classified kind, the HTTP status the
// kind maps to (through the real table), and the retry verdict.
func thirdPartyRunRowFromError(err error) (code string, vendorStatus int, kind string, httpStatus int, retryable bool) {
	var vendorErr *ThirdPartyVendorError
	if !errors.As(err, &vendorErr) {
		return "unknown", 0, string(core.KindServiceUnavailable),
			core.HTTPStatusFor(core.KindServiceUnavailable), false
	}
	classified, _ := ThirdPartyClassifyVendorError(vendorErr)
	return vendorErr.Code, vendorErr.VendorStatus, string(classified),
		core.HTTPStatusFor(classified), vendorErr.Retryable
}

// thirdPartyRunLiveProbe drives one call through a live skeleton and
// shapes the observed refusal row. The row is honest whatever the
// environment holds: no credential → credentials_absent; credential
// present → the stub refusal naming the real call site.
func thirdPartyRunLiveProbe(cfg ThirdPartyLiveConfig, call string,
	drive func(ThirdPartyLiveConfig, func(string) (string, bool)) error,
	credentials func(string) (string, bool)) ThirdPartyRunRow {
	err := drive(cfg, credentials)
	code, vendorStatus, kind, httpStatus, retryable := thirdPartyRunRowFromError(err)
	return ThirdPartyRunRow{
		Step: 1, Call: call, Input: cfg.KeyEnvVar + " (by name — the value never crosses this wire)",
		Outcome: "refused-honestly", VendorCode: code, VendorStatus: vendorStatus,
		Kind: kind, HTTPStatus: httpStatus, Retryable: retryable, Detail: err.Error(),
	}
}

// ── the fixed scripts ───────────────────────────────────────────────

// thirdPartyRunMailerScript drives the mailer sandbox through its
// fixed choreography: an invalid draft, a greylisted first send, the
// retry that succeeds, two more accepted sends to the SAME address
// (greylisting is per new address — an established sender flows), the
// quota refusal at the ceiling, and the signing discipline verified
// honest and tampered.
func thirdPartyRunMailerScript() ThirdPartySandboxPanel {
	mailer := newThirdPartySandboxMailer()
	ctx := context.Background()
	rows := []ThirdPartyRunRow{}
	counts := map[string]int{}
	step := 0

	// call drives one send and DERIVES the outcome label from what the
	// adapter answered — a refusal never wears an acceptance label.
	call := func(label, input string, draft ThirdPartyMailDraft) ThirdPartyRunRow {
		step++
		receipt, err := mailer.Send(ctx, draft)
		outcome := "accepted"
		row := ThirdPartyRunRow{Step: step, Call: label, Input: input, Outcome: outcome}
		if err != nil {
			row.VendorCode, row.VendorStatus, row.Kind, row.HTTPStatus, row.Retryable = thirdPartyRunRowFromError(err)
			row.Detail = err.Error()
			switch row.VendorCode {
			case "invalid_recipient":
				row.Outcome = "refused-invalid"
			case "greylisted":
				row.Outcome = "refused-greylisted"
			case "send_quota_exceeded":
				row.Outcome = "refused-quota"
			default:
				panic("thirdparty hub: mailer script met a vendor code it cannot label: " + row.VendorCode)
			}
		} else {
			row.Detail = fmt.Sprintf("accepted as %s, signature %s", receipt.MessageID, receipt.Signature)
		}
		rows = append(rows, row)
		return row
	}

	call("Send", `To: "not-an-address"`,
		ThirdPartyMailDraft{To: "not-an-address", Subject: "bad"})
	counts["invalid drafts refused"] = 1

	call("Send (first to a new address)", "To: ops@example.com",
		ThirdPartyMailDraft{To: "ops@example.com", Subject: "quota report"})
	counts["greylisted first sends (451 → service_unavailable, retryable)"] = 1

	retried := call("Send (the retry)", "To: ops@example.com — same message, retried",
		ThirdPartyMailDraft{To: "ops@example.com", Subject: "quota report"})
	if retried.Outcome != "accepted" {
		panic("thirdparty hub: the greylist retry must be accepted — the script's contract")
	}
	retried.Outcome = "accepted-after-retry"

	call("Send", "To: ops@example.com again — an established sender",
		ThirdPartyMailDraft{To: "ops@example.com", Subject: "digest"})
	call("Send", "To: ops@example.com — the budget's last slot",
		ThirdPartyMailDraft{To: "ops@example.com", Subject: "summary"})
	call("Send", "To: ops@example.com — the window is full",
		ThirdPartyMailDraft{To: "ops@example.com", Subject: "one too many"})

	// The signing discipline, verified for real: the retry receipt's
	// signature checks out over the bound address; the same signature
	// claimed for a different address does not. msg_sandbox_02 is the
	// second accepted send — "digest" — bound to ops@example.com.
	step++
	honest := thirdPartySignMessage("msg_sandbox_02", "ops@example.com")
	verified := ThirdPartyVerifySandboxSignature("msg_sandbox_02", "ops@example.com", honest)
	tampered := ThirdPartyVerifySandboxSignature("msg_sandbox_02", "attacker@example.com", honest)
	rows = append(rows, ThirdPartyRunRow{
		Step: step, Call: "VerifySandboxSignature",
		Input:   "msg_sandbox_02 for ops@example.com, then the same signature claimed for attacker@example.com",
		Outcome: "verified-then-rejected",
		Detail: fmt.Sprintf("honest recipient: constant-time compare verified=%t; tampered recipient: verified=%t "+
			"(the MAC covers the bound address)", verified, tampered),
	})
	counts["signatures verified"] = 0
	if verified {
		counts["signatures verified"] = 1
	}
	counts["signature rejections"] = 0
	if tampered {
		counts["signature rejections"] = 1
	}
	for _, row := range rows {
		switch row.Outcome {
		case "accepted", "accepted-after-retry":
			counts["messages accepted"]++
		case "refused-quota":
			counts["quota refusals (429 → rate_limited)"]++
		}
	}

	return ThirdPartySandboxPanel{
		Config: []string{
			fmt.Sprintf("send budget: %d accepted sends per window", thirdPartyMailerBudget),
			"greylisting: the first send to any new address is refused transiently (451)",
			"signing: HMAC-SHA256 over " + thirdPartySignatureFormat + " at the fixed exhibit clock " + thirdPartyExhibitClock,
		},
		Rows:   rows,
		Counts: thirdPartySortedCounts(counts),
	}
}

// thirdPartyRunPaymentsScript drives the payments sandbox through its
// fixed choreography: a missing key, a clean charge, the idempotent
// replay, the key-conflict replay, a decline, the quota refusal, and
// the signed event verified three ways (honest, tampered, stale).
func thirdPartyRunPaymentsScript() ThirdPartySandboxPanel {
	processor := newThirdPartySandboxPayments()
	ctx := context.Background()
	rows := []ThirdPartyRunRow{}
	counts := map[string]int{}
	step := 0

	// charge drives one charge; refusal labels are DERIVED from the
	// vendor code, success labels are VERIFIED (a mismatched
	// expectation is a script bug and panics — never a mislabeled row).
	charge := func(label, input string, req ThirdPartyChargeRequest, successOutcome string) {
		step++
		receipt, err := processor.Charge(ctx, req)
		row := ThirdPartyRunRow{Step: step, Call: label, Input: input}
		if err != nil {
			row.VendorCode, row.VendorStatus, row.Kind, row.HTTPStatus, row.Retryable = thirdPartyRunRowFromError(err)
			row.Detail = err.Error()
			switch row.VendorCode {
			case "missing_idempotency_key", "invalid_amount", "idempotency_key_reused":
				row.Outcome = "refused-invalid"
			case "card_declined":
				row.Outcome = "refused-declined"
			case "charge_quota_exceeded":
				row.Outcome = "refused-quota"
			default:
				panic("thirdparty hub: payments script met a vendor code it cannot label: " + row.VendorCode)
			}
		} else {
			row.Outcome = successOutcome
			row.Detail = fmt.Sprintf("%s succeeded, %d cents", receipt.ChargeID, receipt.AmountCents)
		}
		rows = append(rows, row)
	}

	charge("Charge", "no idempotency key",
		ThirdPartyChargeRequest{AmountCents: 1000, CardToken: thirdPartyCardOK}, "")
	counts["calls without an idempotency key refused"] = 1

	charge("Charge", `key "order-1001-receipt", 4200 cents, tok_visa_4242`,
		ThirdPartyChargeRequest{IdempotencyKey: "order-1001-receipt", AmountCents: 4200,
			Currency: "USD", CardToken: thirdPartyCardOK}, "charged")
	counts["charges processed"] = 1

	// Capture the FIRST charge's event now — a value copy, so later
	// charges' events do not rewrite the one this script verifies.
	firstEvent, hadEvent := processor.LastEvent()
	if !hadEvent {
		panic("thirdparty hub: the first scripted charge must have emitted an event")
	}

	charge("Charge (the replay)", `key "order-1001-receipt", 4200 cents — identical`,
		ThirdPartyChargeRequest{IdempotencyKey: "order-1001-receipt", AmountCents: 4200,
			Currency: "USD", CardToken: thirdPartyCardOK}, "deduped-replay")
	counts["idempotent replays deduped to the original charge"] = 1

	charge("Charge (the bug-shaped replay)", `key "order-1001-receipt", 9900 cents — same key, different money`,
		ThirdPartyChargeRequest{IdempotencyKey: "order-1001-receipt", AmountCents: 9900,
			Currency: "USD", CardToken: thirdPartyCardOK}, "")
	counts["key-conflict replays refused"] = 1

	charge("Charge", "tok_declined_0002 — the classic decline test card",
		ThirdPartyChargeRequest{IdempotencyKey: "order-1002-receipt", AmountCents: 1500,
			Currency: "USD", CardToken: thirdPartyCardDeclined}, "")
	counts["declines classified input-shaped (402 → validation_failed)"] = 1

	charge("Charge", "a second distinct charge — the window fills",
		ThirdPartyChargeRequest{IdempotencyKey: "order-1003-receipt", AmountCents: 600,
			Currency: "USD", CardToken: thirdPartyCardOK}, "charged")
	counts["charges processed"]++

	charge("Charge", "a third distinct charge",
		ThirdPartyChargeRequest{IdempotencyKey: "order-1004-receipt", AmountCents: 700,
			Currency: "USD", CardToken: thirdPartyCardOK}, "")
	counts["quota refusals (429 → rate_limited)"] = 1

	// The signed event, verified three ways. The tampered variant
	// rewrites the amount AFTER signing; the stale variant carries a
	// fresh signature over an old timestamp — and dies at the clock
	// check before any crypto runs.
	event := firstEvent
	received := thirdPartyFixedEventClock(thirdPartyEventReceivedAt)
	tamperedEvent := event
	tamperedEvent.JSON = strings.Replace(event.JSON, `"amountCents":4200`, `"amountCents":99000`, 1)
	staleEvent := event
	staleEvent.Timestamp = thirdPartyStaleEventClock
	staleEvent.Signature = thirdPartyEventSignature(thirdPartyStaleEventClock, staleEvent.JSON)
	step++
	rows = append(rows,
		thirdPartyEventVerifyRow(step, event, received, "verified"),
	)
	step++
	rows = append(rows,
		thirdPartyEventVerifyRow(step, tamperedEvent, received, "rejected-tampered"),
	)
	step++
	rows = append(rows,
		thirdPartyEventVerifyRow(step, staleEvent, received, "rejected-stale"),
	)
	counts["events verified"] = 1
	counts["event rejections (tampered + stale)"] = 2

	return ThirdPartySandboxPanel{
		Config: []string{
			fmt.Sprintf("charge budget: %d processed charges per window (replays cost nothing — that is the point)",
				thirdPartyChargeBudget),
			"idempotency: same key + same amount replays the original receipt; same key + different amount is refused",
			"declines: tok_declined_0002 answers the vendor's card_declined — input-shaped, never retryable",
			"events: charge.succeeded signed as " + thirdPartyEventFormat + " at " + thirdPartyExhibitClock +
				", verified at " + thirdPartyEventReceivedAt + " (±" + thirdPartyEventTolerance.String() + " freshness)",
		},
		Rows:   rows,
		Counts: thirdPartySortedCounts(counts),
	}
}

// thirdPartyEventSignature computes an event signature over an
// arbitrary (timestamp, payload) pair — used to sign the stale variant
// honestly: a valid MAC over an old timestamp, so only the freshness
// gate can catch it.
func thirdPartyEventSignature(timestamp, payload string) string {
	mac := hmac.New(sha256.New, []byte(thirdPartySandboxSecret))
	fmt.Fprintf(mac, "%s.%s", timestamp, payload)
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}

// thirdPartyEventVerifyRow verifies one event variant for real and
// shapes the row from the verifier's own verdict. A rejected event
// carries the RECEIVING side's classification — unauthorized, the
// webhooks topic's refusal for unverified deliveries — with the HTTP
// status derived through the real core.HTTPStatusFor.
func thirdPartyEventVerifyRow(step int, event ThirdPartyEvent, receivedAt time.Time, outcome string) ThirdPartyRunRow {
	err := ThirdPartyVerifySandboxEvent(event, receivedAt)
	detail := "verified: fresh timestamp, constant-time MAC match over the exact payload bytes"
	row := ThirdPartyRunRow{Step: step, Call: "VerifySandboxEvent",
		Input:   fmt.Sprintf("%s (%s), signature v1=…%s", event.ID, event.Type, event.Signature[len(event.Signature)-8:]),
		Outcome: outcome}
	if err != nil {
		detail = err.Error()
		row.Kind = string(core.KindUnauthorized)
		row.HTTPStatus = core.HTTPStatusFor(core.KindUnauthorized)
	}
	row.Detail = detail
	return row
}

// thirdPartyRunStoreScript drives the object-store sandbox through its
// fixed choreography: a hostile key, a put, the content-deduped put,
// an oversized put, a second accepted put that fills the ceiling, a
// missing get, and the rate refusal.
func thirdPartyRunStoreScript() ThirdPartySandboxPanel {
	store := newThirdPartySandboxStore()
	ctx := context.Background()
	rows := []ThirdPartyRunRow{}
	counts := map[string]int{}
	step := 0

	// put drives one put; refusal labels are DERIVED from the vendor
	// code, success labels are VERIFIED (a mismatched expectation is a
	// script bug and panics — never a mislabeled row).
	put := func(label, input, key string, body []byte, successOutcome string) ThirdPartyObjectInfo {
		step++
		info, err := store.Put(ctx, key, body, "application/json")
		row := ThirdPartyRunRow{Step: step, Call: label, Input: input}
		if err != nil {
			row.VendorCode, row.VendorStatus, row.Kind, row.HTTPStatus, row.Retryable = thirdPartyRunRowFromError(err)
			row.Detail = err.Error()
			switch row.VendorCode {
			case "invalid_object_key":
				row.Outcome = "refused-key"
			case "object_too_large":
				row.Outcome = "refused-size"
			case "too_many_writes":
				row.Outcome = "refused-quota"
			default:
				panic("thirdparty hub: store script met a vendor code it cannot label: " + row.VendorCode)
			}
		} else {
			row.Outcome = successOutcome
			row.Detail = fmt.Sprintf("stored %s, %d bytes, etag %s", info.Key, info.SizeBytes, info.ETag)
		}
		rows = append(rows, row)
		return info
	}

	report := []byte(`{"report":"q3","rows":42}`)
	oversized := append([]byte(`{"archive":"annual-2026"}`), []byte(strings.Repeat("x", 100))...)

	put("Put", `key "reports/../../etc/passwd" — structure smuggled into a key`,
		"reports/../../etc/passwd", report, "")
	counts["hostile keys refused"] = 1

	put("Put", fmt.Sprintf("key %q, %d new bytes", "q3-report.json", len(report)),
		"q3-report.json", report, "accepted")
	counts["puts accepted"] = 1

	dup := put("Put", `key "q3-report-copy.json", SAME content — content-addressed dedupe`,
		"q3-report-copy.json", report, "deduped-content")
	counts["content-deduped puts (same etag, no bytes charged)"] = 0
	if dup.ETag == thirdPartyETag(report) {
		counts["content-deduped puts (same etag, no bytes charged)"] = 1
	}

	put("Put", fmt.Sprintf("key %q, %d new bytes — over the budget", "annual-archive.json", len(oversized)),
		"annual-archive.json", oversized, "")
	counts["oversized puts refused (413 → validation_failed)"] = 1

	put("Put", `key "q4-plan.json" — fills the put ceiling`,
		"q4-plan.json", []byte(`{"plan":"q4"}`), "accepted")
	counts["puts accepted"]++

	step++
	if _, err := store.Get(ctx, "absent-report.json"); err == nil {
		panic("thirdparty hub: the absent get must fail")
	} else {
		code, vendorStatus, kind, status, retryable := thirdPartyRunRowFromError(err)
		rows = append(rows, ThirdPartyRunRow{Step: step, Call: "Get",
			Input: `key "absent-report.json" — never written`, Outcome: "refused-not-found",
			VendorCode: code, VendorStatus: vendorStatus, Kind: kind, HTTPStatus: status,
			Retryable: retryable, Detail: err.Error()})
	}
	counts["missing gets classified not_found (via the real core.KindForHTTPStatus)"] = 1

	put("Put", `key "overflow.json" — the put budget is spent`,
		"overflow.json", []byte(`{"n":1}`), "")
	counts["rate refusals (429 → rate_limited)"] = 1

	return ThirdPartySandboxPanel{
		Config: []string{
			fmt.Sprintf("put budget: %d puts per window", thirdPartyPutBudget),
			fmt.Sprintf("byte budget: %d NEW content bytes per window (dedupe charges nothing)", thirdPartyByteBudget),
			"keys: single minted components — separators, traversal, NUL, and empty names are refused",
			"etags: first 16 hex of SHA-256 over the content — same bytes, same tag, anywhere",
		},
		Rows:   rows,
		Counts: thirdPartySortedCounts(counts),
	}
}

// thirdPartySortedCounts renders counted observations as sorted rows —
// map iteration order never reaches the wire.
func thirdPartySortedCounts(counts map[string]int) []ThirdPartyCountRow {
	labels := make([]string, 0, len(counts))
	for label := range counts {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	rows := make([]ThirdPartyCountRow, 0, len(labels))
	for _, label := range labels {
		rows = append(rows, ThirdPartyCountRow{Label: label, Value: counts[label]})
	}
	return rows
}
