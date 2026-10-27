package demos

import (
	"encoding/json"
	"strings"
	"testing"

	"guide/internal/domain/realtime"
)

// The realtime story builders — prefixed test names (wave rule). The
// pins: story shape on the wire, non-nil slices, the DERIVED bytes
// matching the real realtime package's own output (a drifted sentence
// or frame fails here), determinism across two builds, and the chat
// choreography's arithmetic being the hub's real answer.

func TestRealtimeTickerPayloadHasTheStoryShape(t *testing.T) {
	payload := BuildRealtimeTickerPayload()
	if payload.Mechanism != "sse-ticker" {
		t.Errorf("mechanism = %q", payload.Mechanism)
	}
	if len(payload.Beats) != 6 {
		t.Errorf("beats = %d, want 6", len(payload.Beats))
	}
	if len(payload.Proofs) == 0 || payload.Reasoning == "" {
		t.Error("payload lacks proofs/reasoning")
	}
	for _, beat := range payload.Beats {
		if beat.Facts == nil {
			t.Errorf("beat %q facts marshaled null", beat.Beat)
		}
		if beat.Verdict != RealtimeVerdictOK &&
			beat.Verdict != RealtimeVerdictRejected && beat.Verdict != RealtimeVerdictInfo {
			t.Errorf("beat %q verdict = %q", beat.Beat, beat.Verdict)
		}
	}
}

func TestRealtimeTickerPayloadQuotesTheRealFormatter(t *testing.T) {
	payload := BuildRealtimeTickerPayload()
	// The frame fact must be EXACTLY what FormatEvent emits for the
	// same message — the story cannot drift from the wire.
	sampleJSON, err := json.Marshal(realtime.Message{
		Seq: 41, Kind: "tick", From: "feed", Body: "tick", At: realtimeSampleAt,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	wantFrame := realtime.FormatEvent(41, "tick", string(sampleJSON))
	var frameFact *string
	for _, beat := range payload.Beats {
		if beat.Beat == "the frame anatomy" {
			frameFact = &beat.Facts[0].Value
		}
	}
	if frameFact == nil {
		t.Fatal("frame anatomy beat missing")
	}
	if *frameFact != wantFrame {
		t.Errorf("story frame = %q, want the formatter's own %q", *frameFact, wantFrame)
	}
	// The resume beat's arithmetic: After(38) on a 41-tick feed is
	// exactly three frames, 39..41 — stated with the real seqs.
	var resumeBeat *string
	for _, beat := range payload.Beats {
		if beat.Beat == "Last-Event-ID resume" {
			resumeBeat = &beat.Facts[0].Value
		}
	}
	if resumeBeat == nil || !strings.Contains(*resumeBeat, "39, 40, 41") {
		t.Errorf("resume fact = %v, want seqs 39, 40, 41", resumeBeat)
	}
	// The live endpoint pointer matches the real route the handlers own.
	found := false
	for _, beat := range payload.Beats {
		for _, fact := range beat.Facts {
			if strings.Contains(fact.Value, RealtimeTickerLivePath) {
				found = true
			}
		}
	}
	if !found {
		t.Error("payload never points at the live stream endpoint")
	}
}

func TestRealtimeChatPayloadChoreographyIsTheHubsAnswer(t *testing.T) {
	payload := BuildRealtimeChatPayload()
	if payload.Mechanism != "ws-chat" || len(payload.Beats) != 6 {
		t.Fatalf("mechanism/beats = %q/%d", payload.Mechanism, len(payload.Beats))
	}
	var publish4, evictionClose string
	for _, beat := range payload.Beats {
		if beat.Beat == "the slow consumer" {
			for _, fact := range beat.Facts {
				if strings.HasPrefix(fact.Label, "publish #4") {
					publish4 = fact.Value
				}
				if fact.Label == "the eviction close" {
					evictionClose = fact.Value
				}
			}
		}
	}
	if publish4 == "" || evictionClose == "" {
		t.Fatal("slow-consumer beat missing its publish #4 / eviction-close facts")
	}
	// Buffer 2 + budget 2: publish #4 evicts. The hub said so; the
	// payload must quote exactly that, with the close code.
	if !strings.Contains(publish4, "evicted: [stalled]") {
		t.Errorf("publish #4 fact = %q, want the stalled eviction", publish4)
	}
	if !strings.Contains(evictionClose, "1013") {
		t.Errorf("eviction close fact = %q, want code 1013", evictionClose)
	}
	// The close-code table covers every code the domain defines, with
	// the domain's own explanations.
	var table []RealtimeStoryFact
	for _, beat := range payload.Beats {
		if beat.Beat == "ping/pong and the close codes" {
			table = beat.Facts
		}
	}
	if len(table) != 7 {
		t.Fatalf("close-code table = %d facts, want 7", len(table))
	}
	// The codes in order, each with the domain's own explanation.
	for i, want := range []string{"1000", "1001", "1003", "1008", "1009", "1011", "1013"} {
		if table[i].Label != want {
			t.Errorf("close-code %d = %q, want %q", i, table[i].Label, want)
		}
	}
	if !strings.Contains(table[6].Value, "stopped reading") {
		t.Errorf("slow-consumer explanation = %q", table[6].Value)
	}
	// The live room endpoint pointer matches the real route.
	if !strings.Contains(payload.Beats[5].Facts[0].Value, RealtimeChatLivePath) {
		t.Errorf("live endpoint fact = %+v", payload.Beats[5].Facts[0])
	}
}

func TestRealtimeStoryPayloadsMarshalWithNonNilSlices(t *testing.T) {
	for _, build := range []func() RealtimeStoryPayload{
		BuildRealtimeTickerPayload, BuildRealtimeChatPayload,
	} {
		encoded, err := json.Marshal(build())
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if strings.Contains(string(encoded), ":null") {
			t.Error("payload marshals a null slice")
		}
	}
}

func TestRealtimeStoryPayloadsAreDeterministic(t *testing.T) {
	firstTicker, secondTicker := BuildRealtimeTickerPayload(), BuildRealtimeTickerPayload()
	firstChat, secondChat := BuildRealtimeChatPayload(), BuildRealtimeChatPayload()
	a, _ := json.Marshal(firstTicker)
	b, _ := json.Marshal(secondTicker)
	if string(a) != string(b) {
		t.Error("ticker story is not byte-stable across builds")
	}
	c, _ := json.Marshal(firstChat)
	d, _ := json.Marshal(secondChat)
	if string(c) != string(d) {
		t.Error("chat story is not byte-stable across builds")
	}
}
