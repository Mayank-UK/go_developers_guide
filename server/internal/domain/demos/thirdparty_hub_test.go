package demos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"guide/internal/core"
)

// The third-party hub's machinery, tested directly: the fixed scripts
// observe what they claim to observe, the vendor-error taxonomy
// mapping runs the real core functions, the live skeletons refuse
// honestly both with and without credentials, the DI switch wires by
// data mode, the shared-sandbox state is race-clean, and the whole
// payload is a deterministic function of its facts — no wall clock, no
// randomness, no invented numbers.

// thirdPartyAbsentCredentials is the guide's honest runtime resolver:
// no vendor credential is configured anywhere.
func thirdPartyAbsentCredentials(string) (string, bool) { return "", false }

// thirdPartyPresentCredentials simulates an operator who set every
// key — the skeletons must then refuse with the OTHER honest error.
func thirdPartyPresentCredentials(name string) (string, bool) {
	return "sk_fake_" + name, true
}

func thirdPartyHubFacts(mode core.DataMode, credentials func(string) (string, bool)) ThirdPartyHubFacts {
	return ThirdPartyHubFacts{
		Scenario:    core.ScenarioState{DataMode: mode, InjectFailure: core.FailureNone},
		Credentials: credentials,
	}
}

func TestThirdPartyHubPayloadIsDeterministicFromItsFacts(t *testing.T) {
	first, err := json.Marshal(BuildThirdPartyHubPayload(thirdPartyHubFacts(core.DataModeFake, thirdPartyAbsentCredentials)))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	second, err := json.Marshal(BuildThirdPartyHubPayload(thirdPartyHubFacts(core.DataModeFake, thirdPartyAbsentCredentials)))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("same facts must serve byte-identical payloads (fixed scripts, fixed exhibit clock)")
	}
}

// thirdPartyCountValue finds one counted observation by exact label.
func thirdPartyCountValue(t *testing.T, panel ThirdPartySandboxPanel, label string) int {
	t.Helper()
	for _, row := range panel.Counts {
		if row.Label == label {
			return row.Value
		}
	}
	t.Fatalf("count %q missing from panel; counts = %+v", label, panel.Counts)
	return 0
}

func TestThirdPartyHubMailerScriptObservesThePinnedChoreography(t *testing.T) {
	panel := thirdPartyRunMailerScript()
	for label, want := range map[string]int{
		"messages accepted":      3,
		"invalid drafts refused": 1,
		"greylisted first sends (451 → service_unavailable, retryable)": 1,
		"quota refusals (429 → rate_limited)":                           1,
		"signatures verified":                                           1,
		"signature rejections":                                          0,
	} {
		if got := thirdPartyCountValue(t, panel, label); got != want {
			t.Errorf("mailer count %q = %d, want %d", label, got, want)
		}
	}
	// The quota refusal must be the LAST send in the script — the
	// burn-down is the story.
	last := panel.Rows[len(panel.Rows)-2] // the final row is the signature verification
	if last.Outcome != "refused-quota" {
		t.Errorf("the scripted burn-down's last send = %q, want refused-quota", last.Outcome)
	}
}

func TestThirdPartyHubPaymentsScriptObservesThePinnedChoreography(t *testing.T) {
	panel := thirdPartyRunPaymentsScript()
	for label, want := range map[string]int{
		"charges processed":                                          2,
		"calls without an idempotency key refused":                   1,
		"idempotent replays deduped to the original charge":          1,
		"key-conflict replays refused":                               1,
		"declines classified input-shaped (402 → validation_failed)": 1,
		"quota refusals (429 → rate_limited)":                        1,
		"events verified":                                            1,
		"event rejections (tampered + stale)":                        2,
	} {
		if got := thirdPartyCountValue(t, panel, label); got != want {
			t.Errorf("payments count %q = %d, want %d", label, got, want)
		}
	}
}

func TestThirdPartyHubStoreScriptObservesThePinnedChoreography(t *testing.T) {
	panel := thirdPartyRunStoreScript()
	for label, want := range map[string]int{
		"puts accepted": 2,
		"content-deduped puts (same etag, no bytes charged)":                      1,
		"hostile keys refused":                                                    1,
		"oversized puts refused (413 → validation_failed)":                        1,
		"rate refusals (429 → rate_limited)":                                      1,
		"missing gets classified not_found (via the real core.KindForHTTPStatus)": 1,
	} {
		if got := thirdPartyCountValue(t, panel, label); got != want {
			t.Errorf("store count %q = %d, want %d", label, got, want)
		}
	}
}

// TestThirdPartyHubFailureRowsCarryTheRealTaxonomy pins the mapping:
// every refused row's kind came out of the classification table, and
// every HTTP status equals core.HTTPStatusFor(kind) — the REAL
// function, re-derived here rather than trusted from the wire.
func TestThirdPartyHubFailureRowsCarryTheRealTaxonomy(t *testing.T) {
	payload := BuildThirdPartyHubPayload(thirdPartyHubFacts(core.DataModeFake, thirdPartyAbsentCredentials))
	seen := map[string]string{} // vendorCode → kind, for the specific pins below
	for _, vendor := range payload.Vendors {
		for _, row := range vendor.Sandbox.Rows {
			if !strings.HasPrefix(row.Outcome, "refused") && !strings.HasPrefix(row.Outcome, "rejected") {
				continue
			}
			if row.Kind == "" || row.HTTPStatus == 0 {
				t.Fatalf("refused row %q (%s) lacks taxonomy fields: %+v", row.Call, vendor.ID, row)
			}
			if row.HTTPStatus != core.HTTPStatusFor(core.Kind(row.Kind)) {
				t.Errorf("row %q: httpStatus %d, want core.HTTPStatusFor(%s) = %d",
					row.Call, row.HTTPStatus, row.Kind, core.HTTPStatusFor(core.Kind(row.Kind)))
			}
			if row.VendorCode != "" {
				seen[row.VendorCode] = row.Kind
			}
		}
	}
	for code, wantKind := range map[string]string{
		"invalid_recipient":      string(core.KindValidationFailed),
		"greylisted":             string(core.KindServiceUnavailable),
		"send_quota_exceeded":    string(core.KindRateLimited),
		"card_declined":          string(core.KindValidationFailed),
		"idempotency_key_reused": string(core.KindValidationFailed),
		"charge_quota_exceeded":  string(core.KindRateLimited),
		"invalid_object_key":     string(core.KindValidationFailed),
		"object_too_large":       string(core.KindValidationFailed),
		"no_such_object":         string(core.KindNotFound),
		"too_many_writes":        string(core.KindRateLimited),
	} {
		if seen[code] != wantKind {
			t.Errorf("vendor code %q classified %q, want %q", code, seen[code], wantKind)
		}
	}
	// The vendor's 402 decline must NOT surface as our 402 anywhere.
	for _, vendor := range payload.Vendors {
		for _, row := range vendor.Sandbox.Rows {
			if row.VendorStatus == 402 && row.HTTPStatus == 402 {
				t.Error("a card decline (vendor 402) must map onto the taxonomy, never pass through as our 402")
			}
		}
	}
}

func TestThirdPartyHubPaymentsIdempotencySemantics(t *testing.T) {
	processor := newThirdPartySandboxPayments()
	ctx := context.Background()
	base := ThirdPartyChargeRequest{IdempotencyKey: "order-42", AmountCents: 4200, Currency: "USD", CardToken: thirdPartyCardOK}

	first, err := processor.Charge(ctx, base)
	if err != nil {
		t.Fatalf("first charge: %v", err)
	}
	replay, err := processor.Charge(ctx, base)
	if err != nil {
		t.Fatalf("replay must succeed: %v", err)
	}
	if replay.ChargeID != first.ChargeID {
		t.Errorf("replay answered %q, want the original %q — idempotency replays, never re-charges",
			replay.ChargeID, first.ChargeID)
	}

	conflicting := base
	conflicting.AmountCents = 9900
	if _, err := processor.Charge(ctx, conflicting); err == nil {
		t.Fatal("same key + different amount must be refused")
	} else {
		var vendorErr *ThirdPartyVendorError
		if !errors.As(err, &vendorErr) || vendorErr.Code != "idempotency_key_reused" {
			t.Errorf("conflict refusal = %v, want idempotency_key_reused", err)
		}
	}
}

func TestThirdPartyHubLiveAdaptersRefuseHonestly(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name  string
		drive func(credentials func(string) (string, bool)) error
	}{
		{"mailer", func(c func(string) (string, bool)) error {
			_, err := (&thirdPartyLiveMailer{cfg: thirdPartyLiveMailerCfg, credentials: c}).Send(ctx,
				ThirdPartyMailDraft{To: "ops@example.com", Subject: "probe", Body: "body"})
			return err
		}},
		{"payments", func(c func(string) (string, bool)) error {
			_, err := (&thirdPartyLivePayments{cfg: thirdPartyLivePaymentsCfg, credentials: c}).Charge(ctx,
				ThirdPartyChargeRequest{IdempotencyKey: "probe", AmountCents: 100, CardToken: thirdPartyCardOK})
			return err
		}},
		{"storage", func(c func(string) (string, bool)) error {
			_, err := (&thirdPartyLiveStore{cfg: thirdPartyLiveStoreCfg, credentials: c}).Put(ctx,
				"probe.json", []byte("{}"), "application/json")
			return err
		}},
	}
	for _, tc := range cases {
		err := tc.drive(thirdPartyAbsentCredentials)
		var vendorErr *ThirdPartyVendorError
		if !errors.As(err, &vendorErr) || vendorErr.Code != "credentials_absent" {
			t.Errorf("%s without a credential: %v, want credentials_absent", tc.name, err)
			continue
		}
		if kind, _ := ThirdPartyClassifyVendorError(err); kind != core.KindServiceUnavailable {
			t.Errorf("%s credentials_absent classified %q, want service_unavailable", tc.name, kind)
		}

		err = tc.drive(thirdPartyPresentCredentials)
		if !errors.As(err, &vendorErr) || vendorErr.Code != "live_client_not_implemented" {
			t.Errorf("%s with a credential: %v, want live_client_not_implemented", tc.name, err)
			continue
		}
		if !strings.Contains(vendorErr.Detail, thirdPartyLiveStubMarker) &&
			!strings.Contains(vendorErr.Detail, "refuses to invent a response") {
			t.Errorf("%s stub refusal must name the honest reason, got %q", tc.name, vendorErr.Detail)
		}
	}
}

// thirdPartyLiveStubMarker is a stable fragment of the stub refusal's
// honest wording.
const thirdPartyLiveStubMarker = "the real call would be"

func TestThirdPartyHubSwitchWiresByDataMode(t *testing.T) {
	fake := BuildThirdPartyHubPayload(thirdPartyHubFacts(core.DataModeFake, thirdPartyAbsentCredentials))
	if fake.Wiring.Current != "fake" {
		t.Errorf("fake wiring.Current = %q, want fake", fake.Wiring.Current)
	}
	for _, row := range fake.Wiring.Rows {
		if row.Active != "fakeAdapter" {
			t.Errorf("fake mode row %q active = %q, want fakeAdapter", row.Category, row.Active)
		}
		if !strings.Contains(row.ActiveOutcome, "accepted") {
			t.Errorf("fake mode row %q outcome should be an acceptance: %q", row.Category, row.ActiveOutcome)
		}
	}

	live := BuildThirdPartyHubPayload(thirdPartyHubFacts(core.DataModeLive, thirdPartyAbsentCredentials))
	if live.Wiring.Current != "live" {
		t.Errorf("live wiring.Current = %q, want live", live.Wiring.Current)
	}
	for _, row := range live.Wiring.Rows {
		if row.Active != "liveAdapter" {
			t.Errorf("live mode row %q active = %q, want liveAdapter", row.Category, row.Active)
		}
		if !strings.Contains(live.Vendors[0].Live.Outcome.Detail, "MAIL_RELAY_API_KEY") {
			t.Errorf("the live panel must name the credential env var, got %q", live.Vendors[0].Live.Outcome.Detail)
		}
		if !strings.Contains(row.ActiveOutcome, "credentials_absent") {
			t.Errorf("live mode row %q outcome should be the honest credential refusal: %q",
				row.Category, row.ActiveOutcome)
		}
	}
}

func TestThirdPartyHubSignatureAndEventVerification(t *testing.T) {
	signature := thirdPartySignMessage("msg_sandbox_01", "ops@example.com")
	if !ThirdPartyVerifySandboxSignature("msg_sandbox_01", "ops@example.com", signature) {
		t.Error("the honest signature must verify")
	}
	if ThirdPartyVerifySandboxSignature("msg_sandbox_01", "attacker@example.com", signature) {
		t.Error("a signature claimed for a different address must fail")
	}

	_, had := newThirdPartySandboxPayments().LastEvent()
	if had {
		t.Fatal("a fresh processor has no event")
	}
	processor := newThirdPartySandboxPayments()
	if _, err := processor.Charge(context.Background(), ThirdPartyChargeRequest{
		IdempotencyKey: "k", AmountCents: 4200, Currency: "USD", CardToken: thirdPartyCardOK}); err != nil {
		t.Fatalf("charge: %v", err)
	}
	event, had := processor.LastEvent()
	if !had {
		t.Fatal("a successful charge must emit an event")
	}
	received := thirdPartyFixedEventClock(thirdPartyEventReceivedAt)
	if err := ThirdPartyVerifySandboxEvent(event, received); err != nil {
		t.Errorf("the honest event must verify: %v", err)
	}
	stale := event
	stale.Timestamp = thirdPartyStaleEventClock
	stale.Signature = thirdPartyEventSignature(stale.Timestamp, stale.JSON)
	if err := ThirdPartyVerifySandboxEvent(stale, received); err == nil ||
		!strings.Contains(err.Error(), "freshness window") {
		t.Errorf("a validly-signed stale event must die at the clock check, got %v", err)
	}
	tampered := event
	tampered.JSON = strings.Replace(event.JSON, "4200", "99000", 1)
	if err := ThirdPartyVerifySandboxEvent(tampered, received); err == nil ||
		!strings.Contains(err.Error(), "signature mismatch") {
		t.Errorf("a tampered payload must fail the MAC, got %v", err)
	}
}

func TestThirdPartyHubStoreDedupeAndByteAccounting(t *testing.T) {
	store := newThirdPartySandboxStore()
	ctx := context.Background()
	body := []byte(`{"report":"q3","rows":42}`)

	first, err := store.Put(ctx, "q3-report.json", body, "application/json")
	if err != nil {
		t.Fatalf("first put: %v", err)
	}
	dup, err := store.Put(ctx, "q3-report-copy.json", body, "application/json")
	if err != nil {
		t.Fatalf("dedupe put: %v", err)
	}
	if dup.ETag != first.ETag {
		t.Errorf("identical content must share the etag: %q vs %q", dup.ETag, first.ETag)
	}

	// The dedupe charged no bytes: the next put's budget error (if the
	// byte budget were double-charged it would fire here) still reports
	// only the first body's bytes as held.
	big := []byte(strings.Repeat("y", int(thirdPartyByteBudget)-len(body)+1))
	_, err = store.Put(ctx, "one-byte-over.json", big, "application/json")
	if err == nil {
		t.Fatal("a body one byte over the remaining budget must be refused")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("%d already held", len(body))) {
		t.Errorf("the refusal must report the held bytes honestly: %v", err)
	}
}

func TestThirdPartyHubObjectKeyRules(t *testing.T) {
	for _, valid := range []string{"q3-report.json", "a", "archive-2026.tar"} {
		if !thirdPartyValidObjectKey(valid) {
			t.Errorf("key %q must be accepted (dots inside a component are fine)", valid)
		}
	}
	for _, invalid := range []string{"", " ", ".", "..", "a/b", `a\b`, "a..b", "reports/../../etc/passwd"} {
		if thirdPartyValidObjectKey(invalid) {
			t.Errorf("key %q must be refused", invalid)
		}
	}
}

func TestThirdPartyHubSandboxStateIsRaceClean(t *testing.T) {
	mailer := newThirdPartySandboxMailer()
	payments := newThirdPartySandboxPayments()
	store := newThirdPartySandboxStore()
	ctx := context.Background()

	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			addr := fmt.Sprintf("worker-%d@example.com", worker)
			for round := 0; round < 8; round++ {
				_, _ = mailer.Send(ctx, ThirdPartyMailDraft{To: addr, Subject: "race", Body: "x"})
				_, _ = payments.Charge(ctx, ThirdPartyChargeRequest{
					IdempotencyKey: fmt.Sprintf("race-%d-%d", worker, round), AmountCents: 100,
					Currency: "USD", CardToken: thirdPartyCardOK})
				_, _ = store.Put(ctx, fmt.Sprintf("race-%d-%d.json", worker, round),
					[]byte(`{"round":1}`), "application/json")
				_, _ = store.Get(ctx, "race-missing.json")
				_, _ = payments.LastEvent()
			}
		}(worker)
	}
	wg.Wait()
}

// TestThirdPartyHubInventsNoNumbers pins the honesty claim: every
// count on the payload is reproducible (two independent runs serve
// identical counts), and every taxonomy row's HTTP status is the real
// core.HTTPStatusFor of its kind. No timing values exist on the wire
// at all.
func TestThirdPartyHubInventsNoNumbers(t *testing.T) {
	first := BuildThirdPartyHubPayload(thirdPartyHubFacts(core.DataModeFake, thirdPartyAbsentCredentials))
	second := BuildThirdPartyHubPayload(thirdPartyHubFacts(core.DataModeFake, thirdPartyAbsentCredentials))
	for i, vendor := range first.Vendors {
		for j, got := range vendor.Sandbox.Counts {
			want := second.Vendors[i].Sandbox.Counts[j]
			if got != want {
				t.Errorf("count drifted between runs: %+v vs %+v", got, want)
			}
		}
	}
	for _, row := range first.TaxonomyMap {
		if row.HTTPStatus != core.HTTPStatusFor(core.Kind(row.Kind)) {
			t.Errorf("taxonomy row %q: %d, want core.HTTPStatusFor(%s)", row.VendorCode, row.HTTPStatus, row.Kind)
		}
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, timingField := range []string{"elapsedMs", "durationMs", "totalMs", "\"at\":", "latencyMs"} {
		if strings.Contains(string(encoded), timingField) {
			t.Errorf("the hub payload must not carry timing fields, found %q", timingField)
		}
	}
}

// TestThirdPartyHubUnknownVendorErrorsDegradeToUnavailable pins the
// classify fallback: an unclassified vendor error is a dependency
// problem, never our internal 500.
func TestThirdPartyHubUnknownVendorErrorsDegradeToUnavailable(t *testing.T) {
	unknown := &ThirdPartyVendorError{Code: "brand_new_outage", VendorStatus: 599, Retryable: true}
	if kind, known := ThirdPartyClassifyVendorError(unknown); kind != core.KindServiceUnavailable || known {
		t.Errorf("unknown vendor error classified (%q, known=%t), want (service_unavailable, false)", kind, known)
	}
	plain := fmt.Errorf("a totally ordinary error")
	if kind, known := ThirdPartyClassifyVendorError(plain); kind != core.KindServiceUnavailable || known {
		t.Errorf("non-vendor error classified (%q, known=%t), want (service_unavailable, false)", kind, known)
	}
	if kind, _ := ThirdPartyClassifyVendorError(&ThirdPartyVendorError{Code: "raw-530", VendorStatus: 530}); kind != core.KindServiceUnavailable {
		t.Errorf("unnamed 5xx must classify service_unavailable via the raw path, got %q", kind)
	}
	if kind, _ := ThirdPartyClassifyVendorError(&ThirdPartyVendorError{Code: "raw-418", VendorStatus: 418}); kind != core.KindNotFound {
		t.Errorf("unnamed 4xx must classify via the real core.KindForHTTPStatus, got %q", kind)
	}
}
