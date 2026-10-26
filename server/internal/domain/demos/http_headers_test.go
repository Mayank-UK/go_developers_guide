package demos

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// The echo is computed from the facts the handler extracted; these tests
// pin the classifications the http-web topic's prose claims, so a refactor
// that changes what the classifier says fails HERE first, next to the
// claim.

func httpHeadersSampleFacts() HTTPRequestFacts {
	return HTTPRequestFacts{
		Method:    http.MethodGet,
		Path:      "/api/demo/http/headers",
		Query:     "verbose=1",
		Proto:     "HTTP/1.1",
		RequestID: "req-17",
		Header: http.Header{
			"User-Agent":      {"curl/8.7.1"},
			"Accept":          {"application/json"},
			"Accept-Encoding": {"gzip"},
			"Content-Type":    {"application/json"},
			"Origin":          {"http://localhost:5175"},
			"Authorization":   {"Bearer very-secret"},
			"Cookie":          {"session=very-secret"},
			"X-Request-Id":    {"client-42"},
			"X-Custom":        {"teaching-value"},
			"If-None-Match":   {`"abc123"`},
		},
	}
}

func httpHeadersRow(t *testing.T, payload HTTPHeadersPayload, name string) EchoedHeader {
	t.Helper()
	for _, echoed := range payload.Headers {
		if echoed.Name == name {
			return echoed
		}
	}
	t.Fatalf("no echoed header named %q", name)
	return EchoedHeader{}
}

func TestHTTPHeadersEchoClassifiesScopeAndRole(t *testing.T) {
	payload := BuildHTTPHeadersEcho(httpHeadersSampleFacts())
	want := map[string]struct{ scope, role string }{
		"User-Agent":      {ScopeEndToEnd, RoleIdentity},
		"Accept":          {ScopeEndToEnd, RoleNegotiation},
		"Accept-Encoding": {ScopeEndToEnd, RoleNegotiation},
		"Content-Type":    {ScopeEndToEnd, RoleNegotiation},
		"Origin":          {ScopeEndToEnd, RoleCORS},
		"Authorization":   {ScopeEndToEnd, RoleSecurity},
		"Cookie":          {ScopeEndToEnd, RoleSecurity},
		"X-Request-Id":    {ScopeEndToEnd, RoleIdentity},
		"X-Custom":        {ScopeEndToEnd, RoleExtension},
		"If-None-Match":   {ScopeEndToEnd, RoleOther},
	}
	if len(payload.Headers) != len(want) {
		t.Fatalf("echo = %d headers (%v), want %d", len(payload.Headers), payload.Headers, len(want))
	}
	for name, class := range want {
		got := httpHeadersRow(t, payload, name)
		if got.Scope != class.scope || got.Role != class.role {
			t.Errorf("%s classified %s/%s, want %s/%s", name, got.Scope, got.Role, class.scope, class.role)
		}
	}
}

func TestHTTPHeadersEchoRedactsCredentialsButKeepsTheFact(t *testing.T) {
	payload := BuildHTTPHeadersEcho(httpHeadersSampleFacts())
	for _, name := range []string{"Authorization", "Cookie"} {
		got := httpHeadersRow(t, payload, name)
		if got.Value != RedactedHeaderValue {
			t.Errorf("%s echoed %q, want %q", name, got.Value, RedactedHeaderValue)
		}
		if !strings.Contains(got.Note, "withheld") {
			t.Errorf("%s note %q must say the value is withheld", name, got.Note)
		}
	}
}

func TestHTTPHeadersEchoOrdersBaseTeachingHeadersFirstThenSorted(t *testing.T) {
	payload := BuildHTTPHeadersEcho(httpHeadersSampleFacts())
	got := make([]string, 0, len(payload.Headers))
	for _, echoed := range payload.Headers {
		got = append(got, echoed.Name)
	}
	// The base selection in its fixed order, then the leftovers sorted:
	// If-None-Match < X-Custom.
	want := []string{
		"User-Agent", "Accept", "Accept-Encoding", "Content-Type", "Origin",
		"Authorization", "Cookie", "X-Request-Id", "If-None-Match", "X-Custom",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("echo order = %v, want %v", got, want)
	}
}

func TestHTTPHeadersEchoCanonicalizesNamesLikeNetHTTP(t *testing.T) {
	// Lowercase keys — as a raw map caller would build them — come out in
	// net/textproto canonical form, and case-differing duplicates merge
	// (map iteration order decides which value leads, so both joins pass).
	facts := HTTPRequestFacts{
		Method: http.MethodGet,
		Path:   "/",
		Proto:  "HTTP/1.1",
		Header: http.Header{
			"accept":       {"application/json"},
			"ACCEPT":       {"text/plain"},
			"x-request-id": {"raw-1"},
		},
	}
	payload := BuildHTTPHeadersEcho(facts)
	accept := httpHeadersRow(t, payload, "Accept")
	if accept.Value != "application/json, text/plain" && accept.Value != "text/plain, application/json" {
		t.Errorf("merged Accept = %q, want both values joined", accept.Value)
	}
	if id := httpHeadersRow(t, payload, "X-Request-Id"); id.Value != "raw-1" {
		t.Errorf("canonical X-Request-Id = %q, want raw-1", id.Value)
	}
}

func TestHTTPHeadersEchoClassifiesHopByHopAndCarriesTheReferenceList(t *testing.T) {
	facts := HTTPRequestFacts{
		Method: http.MethodGet,
		Path:   "/",
		Proto:  "HTTP/1.1",
		Header: http.Header{
			"Connection":          {"keep-alive"},
			"Upgrade":             {"websocket"},
			"Proxy-Authorization": {"Basic secret"},
		},
	}
	payload := BuildHTTPHeadersEcho(facts)

	connection := httpHeadersRow(t, payload, "Connection")
	if connection.Scope != ScopeHopByHop || connection.Role != RoleTransport {
		t.Errorf("Connection = %s/%s, want hop-by-hop/transport", connection.Scope, connection.Role)
	}
	upgrade := httpHeadersRow(t, payload, "Upgrade")
	if upgrade.Scope != ScopeHopByHop || upgrade.Role != RoleTransport {
		t.Errorf("Upgrade = %s/%s, want hop-by-hop/transport", upgrade.Scope, upgrade.Role)
	}
	// Security outranks transport: the credential redaction matters more.
	proxyAuth := httpHeadersRow(t, payload, "Proxy-Authorization")
	if proxyAuth.Role != RoleSecurity || proxyAuth.Value != RedactedHeaderValue {
		t.Errorf("Proxy-Authorization = %s/%q, want security/redacted", proxyAuth.Role, proxyAuth.Value)
	}

	want := []string{
		"Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization",
		"Te", "Trailer", "Transfer-Encoding", "Upgrade",
	}
	if !reflect.DeepEqual(payload.HopByHop, want) {
		t.Errorf("hop-by-hop reference = %v, want RFC 9110 §7.6.1 + keep-alive, sorted: %v", payload.HopByHop, want)
	}
}

func TestHTTPHeadersEchoCarriesTheRequestFactsAndTruncatesHostileValues(t *testing.T) {
	facts := httpHeadersSampleFacts()
	facts.Header.Set("X-Huge", strings.Repeat("A", MaxEchoValueLength+50))
	payload := BuildHTTPHeadersEcho(facts)

	if payload.Method != http.MethodGet || payload.Path != "/api/demo/http/headers" ||
		payload.Query != "verbose=1" || payload.Proto != "HTTP/1.1" || payload.RequestID != "req-17" {
		t.Errorf("request facts drifted: %+v", payload)
	}
	if payload.ContentType != "application/json" {
		t.Errorf("contentType = %q, want the inbound Content-Type", payload.ContentType)
	}
	huge := httpHeadersRow(t, payload, "X-Huge")
	if len(huge.Value) != MaxEchoValueLength+len("…") {
		t.Errorf("huge value len = %d, want truncated to %d + ellipsis", len(huge.Value), MaxEchoValueLength)
	}
}

func TestHTTPHeadersEchoWireShapeIsDeterministicCamelCase(t *testing.T) {
	// Empty query and content type are OMITTED (never null); empty
	// headers echo as [] not null; keys are camelCase.
	facts := HTTPRequestFacts{Method: http.MethodGet, Path: "/", Proto: "HTTP/1.1"}
	encoded, err := json.Marshal(BuildHTTPHeadersEcho(facts))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(encoded)
	if !json.Valid(encoded) {
		t.Error("payload must encode as valid JSON")
	}
	for _, banned := range []string{`"query":`, `"contentType":`, `null`} {
		if strings.Contains(body, banned) {
			t.Errorf("payload %s must not appear for an absent fact: %s", banned, body)
		}
	}
	if !strings.Contains(body, `"requestID":"`) || !strings.Contains(body, `"hopByHop":[`) {
		t.Errorf("camelCase keys drifted: %s", body)
	}
	if !strings.Contains(body, `"headers":[]`) {
		t.Errorf("empty headers must marshal as []: %s", body)
	}
}

func TestHTTPHeadersEchoBuilderIsPure(t *testing.T) {
	first, second := BuildHTTPHeadersEcho(httpHeadersSampleFacts()), BuildHTTPHeadersEcho(httpHeadersSampleFacts())
	if !reflect.DeepEqual(first, second) {
		t.Error("builder must be pure: two calls returned different payloads")
	}
}
