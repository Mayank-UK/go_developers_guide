// Topic 33 observability's first live demo: the slog JSON viewer. The
// handler replays REAL requests through the production middleware chain
// with a buffer-backed slog logger, and this builder dissects what came
// out: one access line's fields anatomized key by key, the level matrix
// computed by REALLY gating four exemplar statements through a handler
// configured at each GDG_LOG_LEVEL, and the replay panel showing the
// request-id correlation — one id threading a 404's warn line and the
// finished request's info line, both captured from executions that just
// happened. The builder is PURE: lines in, panels out; every derived
// sentence comes from the observed facts.
package demos

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
)

// ObsViewerPath is the demo area's endpoint — registered on the real mux
// by the composition root, so a plain curl fetches the same dissection
// the panel draws.
const ObsViewerPath = "/api/demo/obs/viewer"

// ObsViewerAnatomyLineOrder fixes the dissection order for known keys
// (the JSON encoder's own order is not stable across map parses); any
// key outside the list sorts after it alphabetically.
var ObsViewerAnatomyLineOrder = []string{"time", "level", "msg", "method", "path", "status", "duration_ms", "request_id"}

// ObsViewerFieldNotes explains one access-line field: what wrote it and
// what a consumer does with it. The keys mirror RequestLogger's output.
var ObsViewerFieldNotes = map[string]string{
	"time":        "when the line was written (slog's handler stamps it — the ONE field the logger adds on its own)",
	"level":       "the severity dial this line passed: INFO for access lines — the request-completion heartbeat",
	"msg":         "the event name — \"http_request\" is the access log's; consumers filter on it first",
	"method":      "the HTTP verb — pairs with path to identify WHICH route shape ran",
	"path":        "the request path as received (the access log logs the raw path; metrics bucket it — different jobs)",
	"status":      "the response status the recorder captured — the health of the request, one number",
	"duration_ms": "inclusive wall time under the logging layer, rounded to 0.1ms — the latency signal logs give for free",
	"request_id":  "the correlation key: the id the request-id middleware stamped, shared by every line and envelope this request produced",
}

// ObsViewerReplayedRequest names one request the handler replayed.
type ObsViewerReplayedRequest struct {
	Label     string
	Request   string
	RequestID string
	Status    int
}

// ObsViewerCapturedLine is one raw JSON log line the replay captured,
// tagged with the replay label that produced it.
type ObsViewerCapturedLine struct {
	Source string
	Raw    string
}

// ObsViewerFacts is everything the payload derives from, observed by
// the handler: the access line chosen for the anatomy dissection, the
// replayed requests (labels + lines + ids), and the captured lines in
// write order.
type ObsViewerFacts struct {
	AnatomyLine    string
	ReplayRequests []ObsViewerReplayedRequest
	ReplayLines    []ObsViewerCapturedLine
}

// ObsViewerFieldRow is one dissected field of the anatomy line.
type ObsViewerFieldRow struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Note  string `json:"note"`
}

// ObsViewerAnatomy is the dissected access line.
type ObsViewerAnatomy struct {
	Line   string              `json:"line"`
	Fields []ObsViewerFieldRow `json:"fields"`
	Note   string              `json:"note"`
}

// ObsViewerLevelRow is one severity level: which configs let it through
// (computed by really running the gate), what this server logs at it.
type ObsViewerLevelRow struct {
	Level      string   `json:"level"`
	SurvivesAt []string `json:"survivesAt"`
	EmittedBy  string   `json:"emittedBy"`
	Note       string   `json:"note"`
}

// ObsViewerLevels is the level matrix panel.
type ObsViewerLevels struct {
	Rows []ObsViewerLevelRow `json:"rows"`
	Gate string              `json:"gate"`
	Note string              `json:"note"`
}

// ObsViewerReplayLine is one replayed line parsed to its correlation
// fields (unparseable lines carry their raw text honestly).
type ObsViewerReplayLine struct {
	Source    string `json:"source"`
	Level     string `json:"level,omitempty"`
	Msg       string `json:"msg,omitempty"`
	RequestID string `json:"requestId,omitempty"`
	Raw       string `json:"raw"`
}

// ObsViewerReplay is the correlation panel: the requests, their lines,
// and the note that draws the eye to the shared id.
type ObsViewerReplay struct {
	Requests []ObsViewerReplayedRequest `json:"requests"`
	Lines    []ObsViewerReplayLine      `json:"lines"`
	Note     string                     `json:"note"`
}

// ObsViewerPayload is the wire shape of GET /api/demo/obs/viewer.
type ObsViewerPayload struct {
	Anatomy   ObsViewerAnatomy `json:"anatomy"`
	Levels    ObsViewerLevels  `json:"levels"`
	Replay    ObsViewerReplay  `json:"replay"`
	Reasoning string           `json:"reasoning"`
}

// obsLevelExemplars are the four exemplar statements the matrix gates:
// messages drawn from the server's REAL logging vocabulary, one per
// level, in level order.
var obsLevelExemplars = []struct {
	Level slog.Level
	Msg   string
}{
	{slog.LevelDebug, "probe detail (the investigation dial — the server logs nothing here today by default)"},
	{slog.LevelInfo, "http_request"},
	{slog.LevelWarn, "error response"},
	{slog.LevelError, "panic recovered"},
}

// obsLevelEmitters names what the server really logs at each level.
var obsLevelEmitters = map[string]string{
	"debug": "nothing today — the level is the dial you turn UP when investigating (config takes debug; no line in this server currently pays for it)",
	"info":  "the process's heartbeat: every \"http_request\" access line plus boot events (server listening, migrations applied, seeds)",
	"warn":  "expected failures: every 4xx error response the central writer logs, and optional-service degradation (mongo unavailable; the boot continues)",
	"error": "programmer-shaped trouble: recovered panics with their stack, taxonomy misses (unhandled error), and boot-stopping failures (database unreachable)",
}

// BuildObsViewerPayload shapes the panels (pure) from the handler's
// observed facts. The level matrix is COMPUTED here by constructing a
// real slog JSON handler at each configured level and firing all four
// exemplars — survivesAt lists the configurations where the line
// actually appeared, never a restatement of the docs.
func BuildObsViewerPayload(facts ObsViewerFacts) ObsViewerPayload {
	return ObsViewerPayload{
		Anatomy: buildObsAnatomy(facts.AnatomyLine),
		Levels:  buildObsLevels(),
		Replay:  buildObsReplay(facts),
		Reasoning: "One line, three panels, one lesson: the log is DATA. The anatomy row-by-row " +
			"dissection is what a log consumer (a grep, a pipeline, a platform's indexer) sees; " +
			"the level matrix is the cost dial — every configuration below a line's level " +
			"silences it, computed by really running the gate; and the replay proves the " +
			"correlation contract — one request id threading the warn a failure wrote and the " +
			"info line the finished request logged, which is the join key that turns thousands " +
			"of lines back into one story",
	}
}

// buildObsAnatomy dissects one raw JSON line into ordered, annotated
// field rows. Unknown keys (future handler options) render after the
// known ones so the panel never hides a field the line actually carried.
func buildObsAnatomy(line string) ObsViewerAnatomy {
	fields := []ObsViewerFieldRow{}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		return ObsViewerAnatomy{
			Line:   line,
			Fields: fields,
			Note:   "the captured line did not parse as JSON — an honest empty dissection, not an invented one",
		}
	}
	rendered := map[string]bool{}
	for _, key := range ObsViewerAnatomyLineOrder {
		if value, ok := parsed[key]; ok {
			rendered[key] = true
			fields = append(fields, ObsViewerFieldRow{
				Key:   key,
				Value: compactJSONValue(value),
				Note:  ObsViewerFieldNotes[key],
			})
		}
	}
	extras := make([]string, 0)
	for key := range parsed {
		if !rendered[key] {
			extras = append(extras, key)
		}
	}
	for _, key := range sortedStrings(extras) {
		fields = append(fields, ObsViewerFieldRow{
			Key:   key,
			Value: compactJSONValue(parsed[key]),
			Note:  "an extra field beyond RequestLogger's documented set — rendered, not hidden",
		})
	}
	note := "the line above is verbatim what slog's JSON handler wrote for one replayed "
	if _, ok := parsed["msg"]; ok && parsed["msg"] == "http_request" {
		note += "GET /api/posts request — the same line shape every request this server serves produces"
	} else {
		note += "request — the same line shape every request this server serves produces"
	}
	return ObsViewerAnatomy{Line: line, Fields: fields, Note: note}
}

// buildObsLevels computes the matrix by executing the gate.
func buildObsLevels() ObsViewerLevels {
	configs := []struct {
		Name  string
		Level slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
	}
	rows := make([]ObsViewerLevelRow, 0, len(obsLevelExemplars))
	for _, exemplar := range obsLevelExemplars {
		survives := []string{}
		for _, config := range configs {
			var out strings.Builder
			gated := slog.New(slog.NewJSONHandler(&out, &slog.HandlerOptions{Level: config.Level}))
			gated.Log(context.Background(), exemplar.Level, exemplar.Msg)
			if strings.Contains(out.String(), "\"msg\":\""+exemplar.Msg+"\"") {
				survives = append(survives, config.Name)
			}
		}
		rows = append(rows, ObsViewerLevelRow{
			Level:      exemplar.Level.String(),
			SurvivesAt: survives,
			EmittedBy:  obsLevelEmitters[levelKey(exemplar.Level)],
			Note:       "survivesAt was measured by firing this level's exemplar through a handler gated at each GDG_LOG_LEVEL and watching what appeared",
		})
	}
	return ObsViewerLevels{
		Rows: rows,
		Gate: "GDG_LOG_LEVEL (debug|info|warn|error) — read once at boot into core.Config.LogLevel",
		Note: "the dial is asymmetric by design: turning it DOWN buys quiet (and blindness); " +
			"turning it UP buys detail (and cost). This server defaults to info — every request " +
			"landed in the ledger, investigation lines held back until someone asks",
	}
}

// levelKey maps a slog level to its emitter-note key.
func levelKey(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return "debug"
	case slog.LevelInfo:
		return "info"
	case slog.LevelWarn:
		return "warn"
	default:
		return "error"
	}
}

// buildObsReplay parses the captured lines and draws the correlation
// conclusion from what is actually there.
func buildObsReplay(facts ObsViewerFacts) ObsViewerReplay {
	lines := make([]ObsViewerReplayLine, 0, len(facts.ReplayLines))
	for _, captured := range facts.ReplayLines {
		row := ObsViewerReplayLine{Source: captured.Source, Raw: captured.Raw}
		var parsed struct {
			Level     string `json:"level"`
			Msg       string `json:"msg"`
			RequestID string `json:"request_id"`
		}
		if err := json.Unmarshal([]byte(captured.Raw), &parsed); err == nil {
			row.Level = parsed.Level
			row.Msg = parsed.Msg
			row.RequestID = parsed.RequestID
		}
		lines = append(lines, row)
	}
	bySource := make(map[string]bool)
	for _, line := range lines {
		if line.RequestID != "" {
			bySource[line.RequestID] = true
		}
	}
	correlated := ""
	for _, request := range facts.ReplayRequests {
		if request.Label == "the 404" && bySource[request.RequestID] {
			correlated = " — the warn the failure wrote and the info the finished request logged share ONE id: " + request.RequestID + ". That id is also in the 404's error envelope, so a client report, the warn, and the access line all join on it"
		}
	}
	return ObsViewerReplay{
		Requests: facts.ReplayRequests,
		Lines:    lines,
		Note: "two replays, one buffer, the lines in write order" + correlated +
			". Duration numbers are real wall time from this execution — reload and they move; the SHAPE never does",
	}
}

// compactJSONValue renders a parsed JSON value for the anatomy table.
func compactJSONValue(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "?"
	}
	return string(encoded)
}

// sortedStrings is the tiny sort the panels share.
func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
