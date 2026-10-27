// Package obs is the observability domain: the W3C trace-context model
// (traceparent/tracestate parsing, validation, and child serialization),
// an in-process span-tree recorder built on top of it, a bounded-timeout
// dependency-readiness check, and a Prometheus-text-exposition metrics
// registry with bounded label sets. Everything here is stdlib-only by
// the guide's dependency discipline — the OpenTelemetry SDK and the
// Prometheus client library are taught in the observability topic as
// comparison prose, never as imports: what this package implements is
// the real protocol surface both sides of those libraries agree on.
package obs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// TraceparentHeader is the W3C trace-context propagation header: the
// one header a traced caller sends and a traced server echoes, carrying
// version, trace id, parent span id, and flags.
const TraceparentHeader = "traceparent"

// TracestateHeader carries the companion vendor list ("consign=...;rw=...")
// alongside traceparent. It is propagated opaquely-but-validated: members
// survive the round trip in order, which is the whole contract.
const TracestateHeader = "tracestate"

// TraceContext is one parsed traceparent value: which version wrote it,
// the trace id both sides share, the parent (remote) span id, and the
// two-hex-digit flags (bit 0 = sampled). Ids are normalized to lowercase
// on parse; String serializes the canonical lowercase form.
type TraceContext struct {
	Version    string
	TraceID    string
	ParentID   string
	Flags      string
	Tracestate []TraceStateMember
}

// TraceStateMember is one "key=value" vendor entry of the tracestate list.
type TraceStateMember struct {
	Key   string
	Value string
}

// ParseTraceparent parses and validates a traceparent header value per the
// W3C trace-context rules this guide teaches: exactly
// version-traceid-parentid-flags for version 00 (future versions may append
// fields, which are ignored, not rejected), lowercase-or-uppercase hex
// normalized to lowercase, the reserved version ff refused, and the
// all-zero trace id and parent id refused (the spec reserves them as
// "invalid"). A malformed header returns an error naming the rule it broke
// — callers treat that as "proceed under a fresh root", never as a failed
// request.
func ParseTraceparent(header string) (TraceContext, error) {
	value := strings.TrimSpace(header)
	if value == "" {
		return TraceContext{}, fmt.Errorf("traceparent: empty header")
	}
	parts := strings.Split(value, "-")
	if len(parts) < 4 {
		return TraceContext{}, fmt.Errorf(
			"traceparent %q: want version-traceid-parentid-flags, got %d field(s)", value, len(parts))
	}

	version, err := parseHexField(value, parts[0], 2, "version")
	if err != nil {
		return TraceContext{}, err
	}
	if version == "ff" {
		return TraceContext{}, fmt.Errorf("traceparent %q: version ff is reserved", value)
	}
	if version == "00" && len(parts) != 4 {
		return TraceContext{}, fmt.Errorf(
			"traceparent %q: version 00 must carry exactly four fields, got %d", value, len(parts))
	}

	traceID, err := parseHexField(value, parts[1], 32, "trace-id")
	if err != nil {
		return TraceContext{}, err
	}
	if traceID == strings.Repeat("0", 32) {
		return TraceContext{}, fmt.Errorf("traceparent %q: trace-id must not be all zeros", value)
	}

	parentID, err := parseHexField(value, parts[2], 16, "parent-id")
	if err != nil {
		return TraceContext{}, err
	}
	if parentID == strings.Repeat("0", 16) {
		return TraceContext{}, fmt.Errorf("traceparent %q: parent-id must not be all zeros", value)
	}

	flags, err := parseHexField(value, parts[3], 2, "flags")
	if err != nil {
		return TraceContext{}, err
	}

	return TraceContext{Version: version, TraceID: traceID, ParentID: parentID, Flags: flags}, nil
}

// parseHexField validates one lowercase-normalized hex field of the exact
// width the spec pins, wrapping the failure with the offending value.
func parseHexField(header, field string, width int, name string) (string, error) {
	if len(field) != width {
		return "", fmt.Errorf("traceparent %q: %s must be %d hex digits, got %d", header, name, width, len(field))
	}
	for i := 0; i < len(field); i++ {
		c := field[i]
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') && !(c >= 'A' && c <= 'F') {
			return "", fmt.Errorf("traceparent %q: %s %q is not hexadecimal", header, name, field)
		}
	}
	return strings.ToLower(field), nil
}

// ParseTracestate parses the companion tracestate header: up to 32
// "key=value" members separated by ";", order preserved, keys unique.
// The grammar enforced here is the teaching subset of the W3C ABNF
// (documented printable keys and values, no delimiters inside members);
// the OpenTelemetry SDK implements the full grammar, and the comparison
// prose in the observability topic says exactly that.
func ParseTracestate(header string) ([]TraceStateMember, error) {
	value := strings.TrimSpace(header)
	if value == "" {
		return nil, fmt.Errorf("tracestate: empty header")
	}
	raw := strings.Split(value, ";")
	if len(raw) > 32 {
		return nil, fmt.Errorf("tracestate: %d members exceeds the 32-member limit", len(raw))
	}
	seen := make(map[string]bool, len(raw))
	members := make([]TraceStateMember, 0, len(raw))
	for _, member := range raw {
		member = strings.TrimSpace(member)
		if member == "" {
			return nil, fmt.Errorf("tracestate %q: empty list member", value)
		}
		key, val, ok := strings.Cut(member, "=")
		if !ok || key == "" || val == "" || strings.Contains(key, "=") {
			return nil, fmt.Errorf("tracestate %q: member %q is not key=value", value, member)
		}
		if err := validateTraceStateToken(key, "key"); err != nil {
			return nil, fmt.Errorf("tracestate %q: %w", value, err)
		}
		if err := validateTraceStateToken(val, "value"); err != nil {
			return nil, fmt.Errorf("tracestate %q: %w", value, err)
		}
		if seen[key] {
			return nil, fmt.Errorf("tracestate %q: duplicate key %q", value, key)
		}
		seen[key] = true
		members = append(members, TraceStateMember{Key: key, Value: val})
	}
	return members, nil
}

// validateTraceStateToken enforces the shared token grammar: 1–256
// printable ASCII characters with the list delimiters (';' and '=')
// excluded so a member can never smuggle structure.
func validateTraceStateToken(token, name string) error {
	if len(token) > 256 {
		return fmt.Errorf("member %s %q exceeds 256 characters", name, token)
	}
	for i := 0; i < len(token); i++ {
		c := token[i]
		if c <= ' ' || c >= 0x7f || c == ';' || c == '=' {
			return fmt.Errorf("member %s %q contains the invalid character %q", name, token, c)
		}
	}
	return nil
}

// FormatTracestate serializes members back to the wire form, order
// preserved — the round trip that makes tracestate a propagation header
// and not a parser's dead end.
func FormatTracestate(members []TraceStateMember) string {
	parts := make([]string, 0, len(members))
	for _, member := range members {
		parts = append(parts, member.Key+"="+member.Value)
	}
	return strings.Join(parts, ";")
}

// NewTraceID mints a fresh 32-hex-digit trace id (16 random bytes).
func NewTraceID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// crypto/rand does not fail on the supported platforms; a constant
		// keeps the process serving if it ever does (same posture as
		// core.NewRequestID).
		return strings.Repeat("1", 32)
	}
	return hex.EncodeToString(buf[:])
}

// NewSpanID mints a fresh 16-hex-digit span id (8 random bytes).
func NewSpanID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return strings.Repeat("2", 16)
	}
	return hex.EncodeToString(buf[:])
}

// NewRoot starts a brand-new trace: fresh trace and span ids, version 00,
// sampled flag set — what a server mints when no inbound context arrived
// (or the inbound one was malformed and rejected).
func NewRoot() TraceContext {
	return TraceContext{Version: "00", TraceID: NewTraceID(), ParentID: NewSpanID(), Flags: "01"}
}

// Child derives the context for the next span under this one: same trace,
// fresh span id, flags carried. The server's outbound header carries the
// child so the caller can connect the response to the work it caused.
func (tc TraceContext) Child() TraceContext {
	return TraceContext{
		Version:    "00",
		TraceID:    tc.TraceID,
		ParentID:   NewSpanID(),
		Flags:      tc.Flags,
		Tracestate: tc.Tracestate,
	}
}

// Sampled reports whether the sampled flag bit is set — the hint that
// says this trace is worth the cost of recording.
func (tc TraceContext) Sampled() bool {
	if len(tc.Flags) != 2 {
		return false
	}
	// The sampled bit is bit 0 of the flag BYTE — the lowest bit of the
	// second hex digit: odd digits have it set.
	switch tc.Flags[1] {
	case '1', '3', '5', '7', '9', 'b', 'd', 'f':
		return true
	default:
		return false
	}
}

// String serializes the canonical wire form, lowercase, four fields.
func (tc TraceContext) String() string {
	return tc.Version + "-" + tc.TraceID + "-" + tc.ParentID + "-" + tc.Flags
}

// traceContextKey is the unexported context key type: no other package
// can collide with or forge the ambient trace context.
type traceContextKey struct{}

// WithTraceContext returns a context carrying the trace context, the way
// the request-id middleware carries the request id: once per request, at
// the edge, read everywhere beneath.
func WithTraceContext(ctx context.Context, tc TraceContext) context.Context {
	return context.WithValue(ctx, traceContextKey{}, tc)
}

// TraceContextFromContext returns the ambient trace context, or false when
// the request was never traced (no middleware, or a caller outside HTTP).
func TraceContextFromContext(ctx context.Context) (TraceContext, bool) {
	if ctx == nil {
		return TraceContext{}, false
	}
	tc, ok := ctx.Value(traceContextKey{}).(TraceContext)
	return tc, ok
}
