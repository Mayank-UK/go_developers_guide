package realtime

import (
	"strconv"
	"strings"
)

// The server-sent events wire format, rendered byte-exactly. A frame is
// a set of "field: value" lines terminated by a BLANK line ("\n\n") —
// the blank line is the delivery signal, so a frame without it is not
// an event yet. These helpers are the single implementation both the
// live stream handler and the teaching payload quote, so what the topic
// shows and what the endpoint emits cannot drift.

// FormatEvent renders one event frame: an id line when id >= 0, an
// event line when the name is non-empty, one data line PER line of the
// payload (multi-line data repeats the field), and the blank-line
// terminator that actually delivers the event.
func FormatEvent(id int64, event, data string) string {
	var builder strings.Builder
	if id >= 0 {
		builder.WriteString("id: ")
		builder.WriteString(strconv.FormatInt(id, 10))
		builder.WriteByte('\n')
	}
	if event != "" {
		builder.WriteString("event: ")
		builder.WriteString(event)
		builder.WriteByte('\n')
	}
	for _, line := range strings.Split(data, "\n") {
		builder.WriteString("data: ")
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	builder.WriteByte('\n')
	return builder.String()
}

// FormatComment renders a comment frame — a line starting with a colon.
// Browsers ignore comments entirely, which is exactly why they carry
// keepalives: they hold the connection open through proxies without
// delivering anything to the page's JavaScript.
func FormatComment(text string) string {
	return ": " + text + "\n\n"
}

// FormatRetry renders the retry field: how many milliseconds the
// browser should wait before reconnecting a dropped stream. EventSource
// reads it like any field but never surfaces it to page code — the
// reconnection policy lives in the browser, advised by the server.
func FormatRetry(millis int) string {
	return "retry: " + strconv.Itoa(millis) + "\n\n"
}

// ParseLastEventID reads the Last-Event-ID request header: the id of
// the LAST event the browser actually processed, sent automatically on
// every reconnection attempt. Empty (first connection) and unparsable
// values both report ok=false — an unknown id means "start fresh"
// rather than "guess a position", so the server replays nothing and
// continues live.
func ParseLastEventID(raw string) (int64, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || id < 0 {
		return 0, false
	}
	return id, true
}
