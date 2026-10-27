package realtime

import (
	"strings"
	"testing"
	"time"
)

// SSE frame rendering and Last-Event-ID parsing, pinned byte-exactly —
// the teaching payload and the live handler both render through these
// functions, so a drifted frame format would surface here first.

func TestFormatEventRendersTheFullFrameByteExactly(t *testing.T) {
	got := FormatEvent(7, "tick", `{"seq":7}`)
	want := "id: 7\nevent: tick\ndata: {\"seq\":7}\n\n"
	if got != want {
		t.Errorf("frame = %q, want %q", got, want)
	}
}

func TestFormatEventRepeatsTheDataFieldPerLine(t *testing.T) {
	got := FormatEvent(3, "", "first\nsecond\nthird")
	want := "id: 3\ndata: first\ndata: second\ndata: third\n\n"
	if got != want {
		t.Errorf("multi-line frame = %q, want %q", got, want)
	}
}

func TestFormatEventOmitsOptionalFields(t *testing.T) {
	if got := FormatEvent(-1, "", "payload"); got != "data: payload\n\n" {
		t.Errorf("bare frame = %q", got)
	}
	if got := FormatEvent(12, "chat", "hi"); !strings.HasPrefix(got, "id: 12\nevent: chat\n") {
		t.Errorf("named frame = %q, want id and event lines first", got)
	}
}

func TestFormatCommentAndRetryFrames(t *testing.T) {
	if got := FormatComment("keepalive"); got != ": keepalive\n\n" {
		t.Errorf("comment frame = %q", got)
	}
	if got := FormatRetry(3000); got != "retry: 3000\n\n" {
		t.Errorf("retry frame = %q", got)
	}
}

func TestParseLastEventIDAcceptsOnlyRealIDs(t *testing.T) {
	cases := []struct {
		raw  string
		want int64
		ok   bool
	}{
		{"41", 41, true},
		{"  41  ", 41, true}, // browsers may pad; trimming is harmless
		{"0", 0, true},
		{"", 0, false},             // first connection: no header, no resume
		{"not-a-number", 0, false}, // garbage: start fresh, never guess
		{"-3", 0, false},           // negative ids do not exist in this stack
	}
	for _, tc := range cases {
		got, ok := ParseLastEventID(tc.raw)
		if ok != tc.ok || got != tc.want {
			t.Errorf("ParseLastEventID(%q) = (%d, %t), want (%d, %t)", tc.raw, got, ok, tc.want, tc.ok)
		}
	}
}

func TestCloseCodeExplainCoversEveryCodeTheStackUses(t *testing.T) {
	for _, code := range []CloseCode{
		CloseNormal, CloseGoingAway, CloseUnsupportedData, ClosePolicyViolation,
		CloseMessageTooLarge, CloseInternalError, CloseSlowConsumer,
	} {
		if explanation := code.Explain(); strings.Contains(explanation, "unmapped") {
			t.Errorf("code %d lacks its own explanation: %q", uint16(code), explanation)
		}
	}
	unknown := CloseCode(4500).Explain()
	if !strings.Contains(unknown, "unmapped") {
		t.Errorf("unknown code explanation = %q, want the honest unmapped answer", unknown)
	}
}

func TestDefaultChatPolicyNumbersAreTheOnesTaught(t *testing.T) {
	policy := DefaultChatPolicy()
	if policy.ReadLimit != 512 || policy.Buffer != 16 || policy.MaxDrops != 8 {
		t.Errorf("policy = %+v, want the published numbers", policy)
	}
	if policy.Heartbeat <= 0 || policy.WriteTimeout <= 0 {
		t.Errorf("policy = %+v, want positive heartbeat and write timeout", policy)
	}
}

var testNow = time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
