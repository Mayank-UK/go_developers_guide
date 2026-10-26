// HTTP-headers demo payload: echo what the server actually received.
//
// GET /api/demo/http/headers serves this payload — the http-web topic's
// inline demo. Unlike the zero-input builders, this one's whole point is
// the REQUEST: the thin handler extracts the facts (method, path, raw
// query, negotiated protocol, request id, headers) and this pure builder
// classifies them — every echoed header gets end-to-end vs hop-by-hop
// scope, a role (negotiation, CORS, security, identity, extension,
// transport, other), and a teaching note when one earns its place. The
// canonical hop-by-hop reference list (RFC 9110 §7.6.1's closed set plus
// HTTP/1.1's historic keep-alive) rides along so the ABSENCE of Connection
// in a browser's echo is itself the lesson: hop-by-hop headers belong to
// one transport connection. Credentials are detected but never echoed.
package demos

import (
	"net/http"
	"net/textproto"
	"slices"
	"sort"
	"strings"
)

// Header scope values — RFC 9110's split between headers meant for the
// final recipient and headers consumed by each transport hop.
const (
	// ScopeEndToEnd marks a header every hop must forward untouched.
	ScopeEndToEnd = "end-to-end"
	// ScopeHopByHop marks a header that belongs to one connection only.
	ScopeHopByHop = "hop-by-hop"
)

// Header role tags — what a header is FOR, the classification this demo
// teaches alongside the scope.
const (
	RoleNegotiation = "negotiation" // Accept*, Content-Type: choosing a representation
	RoleCORS        = "cors"        // Origin, Access-Control-*: cross-origin machinery
	RoleSecurity    = "security"    // Authorization, Cookie: credentials and auth material
	RoleIdentity    = "identity"    // User-Agent, Host, request ids: who is calling
	RoleExtension   = "extension"   // X-*: convention, not standard
	RoleTransport   = "transport"   // hop-by-hop plumbing: connection framing
	RoleOther       = "other"       // standard but none of the above
)

// RedactedHeaderValue is what the echo shows for a credential-bearing
// header: the demo detects Authorization, Cookie, and Proxy-Authorization
// but never repeats their values — an echo that leaks bearer tokens would
// be its own counter-example.
const RedactedHeaderValue = "[value withheld]"

// MaxEchoValueLength caps an echoed header value so a hostile multi-
// kilobyte header cannot bloat the payload; longer values are truncated
// with an ellipsis, mirroring the snippet matcher's short().
const MaxEchoValueLength = 120

// hopByHopHeaders is RFC 9110 §7.6.1's closed set plus HTTP/1.1's historic
// keep-alive. Keys are in net/textproto canonical form — the same form
// net/http's server uses for Request.Header map keys.
var hopByHopHeaders = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailer":             true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

// negotiationHeaders is the content-negotiation family: headers that state
// what representation the client can consume (or, for Content-Type, what it
// is sending).
var negotiationHeaders = map[string]bool{
	"Accept":          true,
	"Accept-Charset":  true,
	"Accept-Encoding": true,
	"Accept-Language": true,
	"Content-Type":    true,
}

// securityHeaders are the credential bearers: their PRESENCE is a fact
// worth echoing; their values never are. Proxy-Authorization is also in
// the hop-by-hop set — security wins the classification, because the
// redaction matters more than the transport role.
var securityHeaders = map[string]bool{
	"Authorization":       true,
	"Cookie":              true,
	"Proxy-Authorization": true,
	"Www-Authenticate":    true,
}

// identityHeaders name the caller or the path the call took.
var identityHeaders = map[string]bool{
	"User-Agent":        true,
	"Host":              true,
	"From":              true,
	"X-Request-Id":      true,
	"Forwarded":         true,
	"X-Forwarded-For":   true,
	"X-Forwarded-Host":  true,
	"X-Forwarded-Proto": true,
}

// baseHeaderSelection is the echo's fixed opening order: the headers with
// a lesson attached, echoed in teaching order before anything else. Names
// are canonical; Host appears here for direct callers even though net/http
// promotes an inbound Host header out of Request.Header.
var baseHeaderSelection = []string{
	"User-Agent",
	"Accept",
	"Accept-Encoding",
	"Accept-Language",
	"Content-Type",
	"Origin",
	"Authorization",
	"Cookie",
	"X-Request-Id",
}

// headerNotes carries a one-line teaching note for the headers whose note
// is worth the pixels; unknown headers get a role-default note or none.
var headerNotes = map[string]string{
	"User-Agent":          "identifies the calling software; log it, rarely branch on it",
	"Accept":              "content negotiation: the media types this client will accept",
	"Accept-Encoding":     "compression codings the client can decode (gzip, br, zstd)",
	"Accept-Language":     "language negotiation; topic 27 turns it into a choice",
	"Content-Type":        "the body's media type — the contract the decoder parses by",
	"Origin":              "CORS material: the scheme://host:port a cross-origin request names",
	"Authorization":       "credentials — detected, value withheld: bearer tokens never echo",
	"Cookie":              "session material — detected, value withheld",
	"X-Request-Id":        "correlation id — this guide's middleware echoes it or mints a fresh one",
	"Host":                "which site the request targets; Go promotes it to Request.Host",
	"Connection":          "hop-by-hop: describes this one transport connection, never forwarded",
	"Keep-Alive":          "HTTP/1.1 persistence tuning — hop-by-hop",
	"Proxy-Authenticate":  "proxy credential challenge — hop-by-hop",
	"Proxy-Authorization": "hop-by-hop credentials — detected, value withheld",
	"Te":                  "declares acceptable trailer sections — hop-by-hop",
	"Trailer":             "headers sent after the body — hop-by-hop",
	"Transfer-Encoding":   "how the body is framed on the wire (chunked); consumed before handlers",
	"Upgrade":             "protocol switch offer (e.g. websocket); meaningful only with Connection: Upgrade",
	"Cache-Control":       "caching directives; topic 21's 304 story starts here",
	"If-None-Match":       "conditional request; topic 21 answers it with 304 Not Modified",
	"Forwarded":           "reverse-proxy trail: the originating peer, added per hop",
	"X-Forwarded-For":     "the client address each proxy saw; trust it only as far as you trust the proxy",
	"X-Forwarded-Proto":   "the scheme the client actually used, added by the proxy",
}

// HTTPRequestFacts is the request side of the echo: everything the thin
// handler extracted from one live request, and nothing else. Plain data in
// — the builder never touches the request itself.
type HTTPRequestFacts struct {
	Method    string
	Path      string
	Query     string // raw query string, no leading '?'
	Proto     string // negotiated protocol, e.g. "HTTP/1.1"
	RequestID string // the middleware's id for this request
	Header    http.Header
}

// EchoedHeader is one inbound header exactly as received (canonical name,
// joined values), with its scope and role classification and an optional
// teaching note.
type EchoedHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Scope string `json:"scope"`
	Role  string `json:"role"`
	Note  string `json:"note,omitempty"`
}

// HTTPHeadersPayload is the wire shape of GET /api/demo/http/headers
// (camelCase JSON) — the http-web topic's inline demo, adapted from the
// fastapi guide's http:headers echo to what this Go server can prove about
// the request it just served.
type HTTPHeadersPayload struct {
	Method      string         `json:"method"`
	Path        string         `json:"path"`
	Query       string         `json:"query,omitempty"`
	Proto       string         `json:"proto"`
	RequestID   string         `json:"requestID"`
	ContentType string         `json:"contentType,omitempty"`
	Headers     []EchoedHeader `json:"headers"`
	HopByHop    []string       `json:"hopByHop"`
}

// canonicalize flattens a header map through net/textproto's canonical
// form — the same normalization net/http's server applies when it parses a
// request — merging duplicate keys that differ only in case.
func canonicalize(header http.Header) http.Header {
	canonical := make(http.Header, len(header))
	for name, values := range header {
		key := textproto.CanonicalMIMEHeaderKey(name)
		canonical[key] = append(canonical[key], values...)
	}
	return canonical
}

// scopeOf classifies one canonical header name against RFC 9110 §7.6.1's
// closed hop-by-hop set.
func scopeOf(name string) string {
	if hopByHopHeaders[name] {
		return ScopeHopByHop
	}
	return ScopeEndToEnd
}

// roleOf classifies one canonical header name. Security outranks transport
// (Proxy-Authorization is hop-by-hop AND a credential), and specific sets
// outrank the X- extension prefix.
func roleOf(name string) string {
	switch {
	case securityHeaders[name]:
		return RoleSecurity
	case hopByHopHeaders[name]:
		return RoleTransport
	case negotiationHeaders[name]:
		return RoleNegotiation
	case name == "Origin" || strings.HasPrefix(name, "Access-Control-"):
		return RoleCORS
	case identityHeaders[name]:
		return RoleIdentity
	case strings.HasPrefix(name, "X-"):
		return RoleExtension
	default:
		return RoleOther
	}
}

// noteOf returns the teaching note for a canonical header name: the
// specific note when one exists, else a role default (transport and
// extension headers always explain themselves), else none.
func noteOf(name, role string) string {
	if note, ok := headerNotes[name]; ok {
		return note
	}
	switch role {
	case RoleTransport:
		return "hop-by-hop: consumed per connection, never forwarded"
	case RoleExtension:
		return "extension header: the x- prefix is convention, not standard"
	default:
		return ""
	}
}

// truncate caps an echoed value at MaxEchoValueLength bytes with an
// ellipsis, mirroring the snippet matcher's short().
func truncate(value string) string {
	if len(value) <= MaxEchoValueLength {
		return value
	}
	return value[:MaxEchoValueLength] + "…"
}

// echoedHeader builds one row: joined values (redacted for credential
// bearers), scope, role, and note.
func echoedHeader(name string, header http.Header) EchoedHeader {
	value := ""
	if !securityHeaders[name] {
		value = truncate(strings.Join(header.Values(name), ", "))
	} else {
		value = RedactedHeaderValue
	}
	return EchoedHeader{
		Name:  name,
		Value: value,
		Scope: scopeOf(name),
		Role:  roleOf(name),
		Note:  noteOf(name, roleOf(name)),
	}
}

// selection orders the echo: the base teaching headers first (only those
// actually received), then every remaining header sorted by name — fully
// deterministic despite map iteration's randomized order.
func selection(received map[string]bool) []string {
	ordered := make([]string, 0, len(received))
	for _, name := range baseHeaderSelection {
		if received[name] {
			ordered = append(ordered, name)
		}
	}
	rest := make([]string, 0, len(received))
	for name := range received {
		if !slices.Contains(baseHeaderSelection, name) {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	return append(ordered, rest...)
}

// BuildHTTPHeadersEcho assembles the echo payload from the facts the
// handler extracted. It is pure: plain data in, payload out, no I/O — same
// input, same payload, every time.
func BuildHTTPHeadersEcho(facts HTTPRequestFacts) HTTPHeadersPayload {
	canonical := canonicalize(facts.Header)
	received := make(map[string]bool, len(canonical))
	for name := range canonical {
		received[name] = true
	}

	echoed := make([]EchoedHeader, 0, len(received))
	for _, name := range selection(received) {
		echoed = append(echoed, echoedHeader(name, canonical))
	}

	hopReference := make([]string, 0, len(hopByHopHeaders))
	for name := range hopByHopHeaders {
		hopReference = append(hopReference, name)
	}
	sort.Strings(hopReference)

	return HTTPHeadersPayload{
		Method:      facts.Method,
		Path:        facts.Path,
		Query:       facts.Query,
		Proto:       facts.Proto,
		RequestID:   facts.RequestID,
		ContentType: canonical.Get("Content-Type"),
		Headers:     echoed,
		HopByHop:    hopReference,
	}
}
