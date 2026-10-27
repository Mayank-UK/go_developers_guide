package obs

import (
	"context"
	"strings"
	"testing"
)

// The W3C trace-context parser is table-tested against the spec's own
// edge cases: the reserved version, wrong widths, non-hex digits, the
// all-zero ids, future-version trailing fields, and case normalization.

func TestParseTraceparentAcceptsTheCanonicalForm(t *testing.T) {
	tc, err := ParseTraceparent("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	if err != nil {
		t.Fatalf("canonical header must parse: %v", err)
	}
	if tc.Version != "00" || tc.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" ||
		tc.ParentID != "00f067aa0ba902b7" || tc.Flags != "01" {
		t.Errorf("parsed fields drifted: %+v", tc)
	}
	if got, want := tc.String(), "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if !tc.Sampled() {
		t.Error("flags 01 must report sampled")
	}
}

func TestParseTraceparentNormalizesUppercaseHex(t *testing.T) {
	tc, err := ParseTraceparent("00-4BF92F3577B34DA6A3CE929D0E0E4736-00F067AA0BA902B7-01")
	if err != nil {
		t.Fatalf("uppercase hex must parse (spec ABNF allows it): %v", err)
	}
	if tc.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" || tc.ParentID != "00f067aa0ba902b7" {
		t.Errorf("normalization drifted: %+v", tc)
	}
}

func TestParseTraceparentRejectsMalformedHeaders(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   string
	}{
		{"empty", "", "empty header"},
		{"whitespace only", "   ", "empty header"},
		{"too few fields", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7", "3 field(s)"},
		{"reserved version", "ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", "reserved"},
		{"unknown version width", "0-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", "2 hex digits"},
		{"non-hex version", "0x-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", "not hexadecimal"},
		{"short trace id", "00-4bf92f3577b34da6-00f067aa0ba902b7-01", "32 hex digits"},
		{"long parent id", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b799-01", "16 hex digits"},
		{"non-hex trace id", "00-4bf92f3577b34da6a3ce929d0e0e473g-00f067aa0ba902b7-01", "not hexadecimal"},
		{"all-zero trace id", "00-00000000000000000000000000000000-00f067aa0ba902b7-01", "all zeros"},
		{"all-zero parent id", "00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01", "all zeros"},
		{"bad flags width", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-1", "2 hex digits"},
		{"non-hex flags", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-0x", "not hexadecimal"},
		{"version 00 with extra field", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01-extra", "exactly four fields"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseTraceparent(tc.header)
			if err == nil {
				t.Fatalf("header %q must be rejected", tc.header)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q must name %q", err.Error(), tc.want)
			}
		})
	}
}

func TestParseTraceparentAllowsFutureVersionTrailingFields(t *testing.T) {
	tc, err := ParseTraceparent("01-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01-until-the-spec-says")
	if err != nil {
		t.Fatalf("future versions may append fields: %v", err)
	}
	if tc.Version != "01" {
		t.Errorf("version = %q, want 01", tc.Version)
	}
}

func TestChildAndRootRoundTrip(t *testing.T) {
	root := NewRoot()
	if root.Version != "00" || len(root.TraceID) != 32 || len(root.ParentID) != 16 || root.Flags != "01" {
		t.Errorf("root shape drifted: %+v", root)
	}
	child := root.Child()
	if child.TraceID != root.TraceID {
		t.Error("child must keep the trace id")
	}
	if child.ParentID == root.ParentID || len(child.ParentID) != 16 {
		t.Error("child must mint a fresh 16-hex span id")
	}
	// New ids must not collide with the reserved all-zero value.
	if strings.Trim(child.TraceID, "0") == "" || strings.Trim(child.ParentID, "0") == "" {
		t.Error("minted ids must never be all zeros")
	}
	// Not sampled when the flag bit is clear.
	if (&TraceContext{Flags: "00"}).Sampled() {
		t.Error("flags 00 must not report sampled")
	}
}

func TestTraceContextPropagation(t *testing.T) {
	ctx := context.Background()
	if _, ok := TraceContextFromContext(ctx); ok {
		t.Error("bare context must carry no trace context")
	}
	root := NewRoot()
	traced := WithTraceContext(ctx, root)
	got, ok := TraceContextFromContext(traced)
	if !ok || got.TraceID != root.TraceID {
		t.Errorf("round trip drifted: %+v ok=%v", got, ok)
	}
}

func TestTracestateRoundTrip(t *testing.T) {
	members, err := ParseTracestate("consign=poor; rw=2213; opaque=value")
	if err != nil {
		t.Fatalf("valid tracestate must parse: %v", err)
	}
	if len(members) != 3 || members[0].Key != "consign" || members[2].Value != "value" {
		t.Errorf("members drifted: %+v", members)
	}
	if got, want := FormatTracestate(members), "consign=poor;rw=2213;opaque=value"; got != want {
		t.Errorf("re-serialized = %q, want %q", got, want)
	}
}

func TestParseTracestateRejectsMalformedLists(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   string
	}{
		{"empty", "", "empty header"},
		{"member without value", "consign", "key=value"},
		{"empty key", "=poor", "key=value"},
		{"empty value", "consign=", "key=value"},
		{"empty member", "consign=poor;;rw=2", "empty list member"},
		{"duplicate key", "rw=1;rw=2", "duplicate key"},
		{"control char in value", "rw=bad\x01value", "invalid character"},
		{"semicolon smuggled", "rw=a;b", "key=value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseTracestate(tc.header)
			if err == nil {
				t.Fatalf("tracestate %q must be rejected", tc.header)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q must name %q", err.Error(), tc.want)
			}
		})
	}
	// The 33rd member trips the member-count bound.
	var tooMany strings.Builder
	for i := 0; i < 33; i++ {
		if i > 0 {
			tooMany.WriteString(";")
		}
		tooMany.WriteString("k" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + "=v")
	}
	if _, err := ParseTracestate(tooMany.String()); err == nil || !strings.Contains(err.Error(), "32-member") {
		t.Errorf("33 members must trip the bound: %v", err)
	}
}
