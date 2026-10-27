// Topic 28 realtime's two demos share one payload shape — the story
// discipline the family established (beats in execution order with
// verdicts and facts, derived proofs, closing reasoning). This file
// holds the shared wire shape plus the realtime:ticker builder; every
// frame, header value, and resume number the sentences quote is
// DERIVED by calling the real realtime package — the payload cannot
// claim bytes the live endpoint does not emit.
package demos

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"guide/internal/domain/realtime"
)

// Verdict colors for the realtime demos — the same three values every
// story demo serves (aliased so realtime code reads in its own voice).
const (
	RealtimeVerdictOK       = SecurityVerdictOK
	RealtimeVerdictRejected = SecurityVerdictRejected
	RealtimeVerdictInfo     = SecurityVerdictInfo
)

// RealtimeStoryFact is one label/value pair — frame bytes, header
// values, close-code meanings. Values are pre-rendered strings so the
// client never formats.
type RealtimeStoryFact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// RealtimeStoryBeat is one beat as the payload serves it.
type RealtimeStoryBeat struct {
	Beat    string              `json:"beat"`
	Detail  string              `json:"detail"`
	Verdict string              `json:"verdict"`
	Facts   []RealtimeStoryFact `json:"facts"`
}

// RealtimeStoryPayload is the wire shape both realtime story endpoints
// serve. Every slice is non-nil so the wire renders [], never null.
type RealtimeStoryPayload struct {
	Mechanism string              `json:"mechanism"`
	Beats     []RealtimeStoryBeat `json:"beats"`
	Proofs    []RealtimeStoryFact `json:"proofs"`
	Reasoning string              `json:"reasoning"`
}

// newRealtimeStoryPayload assembles a story payload with the wire rules
// applied: non-nil facts and proofs so the JSON renders [], never null.
func newRealtimeStoryPayload(mechanism string, beats []RealtimeStoryBeat, proofs []RealtimeStoryFact, reasoning string) RealtimeStoryPayload {
	for i := range beats {
		if beats[i].Facts == nil {
			beats[i].Facts = []RealtimeStoryFact{}
		}
	}
	if proofs == nil {
		proofs = []RealtimeStoryFact{}
	}
	return RealtimeStoryPayload{Mechanism: mechanism, Beats: beats, Proofs: proofs, Reasoning: reasoning}
}

// realtimeSampleAt is the fixed clock every derived byte in the ticker
// story uses — the payload is byte-stable across fetches.
var realtimeSampleAt = time.Date(2026, 10, 27, 9, 30, 0, 0, time.UTC)

// RealtimeTickerLivePath is the LIVE SSE stream endpoint the story
// points at (the client's live panel and the terminal curl tour hit it
// directly — a JSON story cannot carry a stream).
const RealtimeTickerLivePath = "/api/demo/realtime/ticker/stream"

// RealtimeChatLivePath is the LIVE WebSocket room endpoint.
const RealtimeChatLivePath = "/api/demo/realtime/chat/ws"

// BuildRealtimeTickerPayload shapes the realtime:ticker story: the SSE
// frame anatomy rendered by the real formatter, the response contract
// the live handler stamps, the resume math off a real replay log, the
// keepalive bytes, and the reconnection honesty — plus the pointer to
// the live stream running beside it.
func BuildRealtimeTickerPayload() RealtimeStoryPayload {
	// A real tick message, marshaled and framed exactly as the live
	// endpoint frames it. json.Marshal of this struct cannot fail
	// (strings, an int64, a time.Time).
	sample := realtime.Message{Seq: 41, Kind: "tick", From: "feed", Body: "tick", At: realtimeSampleAt}
	sampleJSON, err := json.Marshal(sample)
	if err != nil {
		// Structurally impossible for this struct; refusing loudly is
		// the honest shape if it ever changes.
		panic(fmt.Sprintf("marshal tick message: %v", err))
	}
	frame := realtime.FormatEvent(sample.Seq, sample.Kind, string(sampleJSON))
	multiline := realtime.FormatEvent(3, "", "first line\nsecond line")
	retryFrame := realtime.FormatRetry(3000)
	keepaliveFrame := realtime.FormatComment("keepalive")

	// The resume math off a real feed: 41 ticks retained, a client that
	// last processed 38 misses exactly 39, 40, 41.
	hub, err := realtime.NewHub(realtime.DefaultHubPolicy())
	if err != nil {
		panic(fmt.Sprintf("story hub: %v", err))
	}
	feed, err := realtime.NewTickerFeed(hub, "ticker", 64, realtime.FeedOptions{
		Interval: time.Second, // the live cadence; TickOnce never waits
		Now:      func() time.Time { return realtimeSampleAt },
	})
	if err != nil {
		panic(fmt.Sprintf("story feed: %v", err))
	}
	for i := 0; i < 41; i++ {
		feed.TickOnce()
	}
	replay := feed.ReplayAfter(38)
	replayedSeqs := make([]string, 0, len(replay))
	for _, msg := range replay {
		replayedSeqs = append(replayedSeqs, fmt.Sprintf("%d", msg.Seq))
	}

	beats := []RealtimeStoryBeat{
		{
			Beat: "the frame anatomy", Verdict: RealtimeVerdictOK,
			Detail: "an SSE event is field lines ended by ONE blank line — the blank line is the delivery signal, so an unfinished frame is not an event yet. The live stream emits exactly these bytes for every tick (a JSON message in the data field, the room's sequence number in id, the event name in event); multi-line data repeats the data field per line:",
			Facts: []RealtimeStoryFact{
				{Label: "a real tick frame", Value: frame},
				{Label: "multi-line data", Value: multiline},
			},
		},
		{
			Beat: "the response contract", Verdict: RealtimeVerdictOK,
			Detail: "the stream response is not JSON: the content type says event stream, no-cache tells intermediaries never to serve a stale copy, X-Accel-Buffering: no tells nginx-style proxies to pass bytes through instead of batching them, and the first bytes on the wire advise the browser's reconnection pace. Every live connection this endpoint serves stamps all four:",
			Facts: []RealtimeStoryFact{
				{Label: "Content-Type", Value: realtime.SSEContentType},
				{Label: "Cache-Control", Value: realtime.SSEStreamRequestHeaders["Cache-Control"]},
				{Label: "X-Accel-Buffering", Value: realtime.SSEStreamRequestHeaders["X-Accel-Buffering"] + " — buffering would break the stream's whole point"},
				{Label: "reconnection advice", Value: retryFrame + "EventSource reads it; page code never manages the reconnect"},
			},
		},
		{
			Beat: "the live stream", Verdict: RealtimeVerdictInfo,
			Detail: fmt.Sprintf("the stream itself runs beside this story at %s — one shared feed, one tick per second, every connected client seeing the SAME sequence numbers. The live panel in this demo page is an EventSource pointed at it right now; from a terminal it is curl -N (no buffering) and Ctrl-C to hang up. Each frame's id advances by one: that id is what makes resume possible.", RealtimeTickerLivePath),
			Facts: []RealtimeStoryFact{
				{Label: "endpoint", Value: "GET " + RealtimeTickerLivePath},
				{Label: "cadence", Value: "one tick per second — the ticks themselves are the liveness proof"},
				{Label: "terminal", Value: "curl -N localhost:8100" + RealtimeTickerLivePath},
			},
		},
		{
			Beat: "Last-Event-ID resume", Verdict: RealtimeVerdictOK,
			Detail: "when the connection drops, EventSource reconnects ON ITS OWN and sends the id of the last event the page processed as the Last-Event-ID header. The server replays every retained tick strictly newer than that id, then continues live. The math below is a real replay run through this exact code: a client that last processed 38 gets exactly three frames back, then live:",
			Facts: []RealtimeStoryFact{
				{Label: "Last-Event-ID: 38", Value: fmt.Sprintf("%d frames replayed — seqs %s — then live", len(replay), strings.Join(replayedSeqs, ", "))},
				{Label: "retention", Value: fmt.Sprintf("the newest %d ticks; ticks older than that are overwritten, and the ids make the gap visible", 64)},
				{Label: "first connection", Value: "no header, no replay — ParseLastEventID says not-ok and the stream starts fresh at the live edge"},
			},
		},
		{
			Beat: "keepalive", Verdict: RealtimeVerdictOK,
			Detail: "an idle stream looks dead to every proxy with a read timeout, so the handler writes a comment frame when nothing else has gone out for a while. Browsers ignore lines starting with a colon completely — no event reaches the page — but the bytes keep the path warm. This feed's ticks arrive every second, so in practice the ticks ARE the keepalive; the comment fires only if the feed ever stalls:",
			Facts: []RealtimeStoryFact{
				{Label: "the keepalive bytes", Value: keepaliveFrame},
				{Label: "who sees it", Value: "nobody — comment frames are invisible to EventSource by design"},
			},
		},
		{
			Beat: "at-least-once honesty", Verdict: RealtimeVerdictInfo,
			Detail: "resume is at-least-once, not exactly-once: a browser that reconnected BEFORE the page processed the last delivered frame replays it again, so consumers may see an id twice. The seq in every frame is the defense — drop anything ≤ the highest seq you have handled and the duplication is harmless. Promising exactly-once over a reconnecting stream would be lying; ids make the honesty cheap:",
			Facts: []RealtimeStoryFact{
				{Label: "per connection", Value: "at-least-once — a frame may be redelivered after a reconnect"},
				{Label: "per id", Value: "at-most-once — dedupe on seq makes redelivery harmless"},
			},
		},
	}
	return newRealtimeStoryPayload("sse-ticker",
		beats,
		[]RealtimeStoryFact{
			{Label: "the wire format", Value: "field lines (id, event, data) + one blank line — rendered by realtime.FormatEvent, the one implementation both the stream and this story call"},
			{Label: "one-way only", Value: "SSE is server→client over plain HTTP — chat, presence, and client pushes need WebSockets (this topic's second demo)"},
			{Label: "the browser contract", Value: "EventSource auto-reconnects with Last-Event-ID and honors retry: — the page never manages the socket"},
			{Label: "the resume math", Value: fmt.Sprintf("ReplayAfter(38) → %s, computed live by the payload builder from a real feed", strings.Join(replayedSeqs, ", "))},
		},
		"Every byte above is derived: the frame is FormatEvent's own output for a real message, the headers are the ones the live handler stamps, and the resume numbers come from a replay log fed 41 real ticks. That is the whole SSE bargain — plain HTTP, a tiny text format, one direction, and reconnection semantics the BROWSER owns: the server's only duties are flushing each frame as it is written, numbering events so resume is possible, retaining recent history so reconnects lose nothing, and keeping the line warm. Anything bidirectional needs the protocol upgrade this topic's second half installs.")
}
