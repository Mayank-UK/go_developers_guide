package demos

import (
	"strings"
	"testing"
)

// The slog viewer's builder tests: anatomy parses and orders the real
// line, the level matrix is COMPUTED by the gate (not restated), and
// the replay panel draws the request-id correlation from the captured
// lines.

func viewerFacts() ObsViewerFacts {
	return ObsViewerFacts{
		AnatomyLine: `{"time":"2026-10-27T10:00:00.000000Z","level":"INFO","msg":"http_request",` +
			`"method":"GET","path":"/api/posts","status":200,"duration_ms":1.2,"request_id":"rid-obs-1"}`,
		ReplayRequests: []ObsViewerReplayedRequest{
			{Label: "the 200", Request: "GET /api/posts?limit=2&sort=newest", RequestID: "rid-obs-1", Status: 200},
			{Label: "the 404", Request: "GET /api/posts/obs-viewer-missing", RequestID: "rid-obs-1", Status: 404},
		},
		ReplayLines: []ObsViewerCapturedLine{
			{Source: "the 200 (its access line)", Raw: `{"time":"2026-10-27T10:00:00.100000Z","level":"INFO","msg":"http_request","method":"GET","path":"/api/posts","status":200,"duration_ms":0.8,"request_id":"rid-obs-1"}`},
			{Source: "the 404 (its failure line)", Raw: `{"time":"2026-10-27T10:00:00.200000Z","level":"WARN","msg":"error response","kind":"not_found","status":404,"request_id":"rid-obs-1"}`},
			{Source: "the 404 (its access line)", Raw: `{"time":"2026-10-27T10:00:00.201000Z","level":"INFO","msg":"http_request","method":"GET","path":"/api/posts/obs-viewer-missing","status":404,"duration_ms":0.4,"request_id":"rid-obs-1"}`},
		},
	}
}

func TestObsViewerAnatomyDissectsEveryFieldInOrder(t *testing.T) {
	payload := BuildObsViewerPayload(viewerFacts())
	wantOrder := []string{"time", "level", "msg", "method", "path", "status", "duration_ms", "request_id"}
	if len(payload.Anatomy.Fields) != len(wantOrder) {
		t.Fatalf("anatomy fields = %d, want %d: %+v", len(payload.Anatomy.Fields), len(wantOrder), payload.Anatomy.Fields)
	}
	for i, want := range wantOrder {
		if payload.Anatomy.Fields[i].Key != want {
			t.Errorf("field %d = %q, want %q", i, payload.Anatomy.Fields[i].Key, want)
		}
		if payload.Anatomy.Fields[i].Note == "" {
			t.Errorf("field %q must carry a note", want)
		}
	}
	if payload.Anatomy.Fields[1].Value != `"INFO"` {
		t.Errorf("level value = %q, want quoted INFO", payload.Anatomy.Fields[1].Value)
	}
}

func TestObsViewerAnatomyRendersExtraKeysHonestly(t *testing.T) {
	facts := viewerFacts()
	facts.AnatomyLine = `{"time":"2026-10-27T10:00:00.000000Z","level":"INFO","msg":"http_request","zz_extra":"x","request_id":"rid"}`
	payload := BuildObsViewerPayload(facts)
	keys := make([]string, 0)
	for _, field := range payload.Anatomy.Fields {
		keys = append(keys, field.Key)
	}
	if !strings.Contains(strings.Join(keys, ","), "zz_extra") {
		t.Errorf("extra key must render, not hide: %v", keys)
	}
	// Unparseable anatomy: honest empty, no panic.
	facts.AnatomyLine = "not json"
	if payload = BuildObsViewerPayload(facts); len(payload.Anatomy.Fields) != 0 {
		t.Errorf("unparseable line must dissect to zero fields: %+v", payload.Anatomy.Fields)
	}
}

func TestObsViewerLevelMatrixIsReallyComputed(t *testing.T) {
	payload := BuildObsViewerPayload(viewerFacts())
	if len(payload.Levels.Rows) != 4 {
		t.Fatalf("level rows = %d, want 4", len(payload.Levels.Rows))
	}
	wantSurvives := map[string][]string{
		"DEBUG": {"debug"},
		"INFO":  {"debug", "info"},
		"WARN":  {"debug", "info", "warn"},
		"ERROR": {"debug", "info", "warn", "error"},
	}
	for _, row := range payload.Levels.Rows {
		want := wantSurvives[row.Level]
		if strings.Join(row.SurvivesAt, ",") != strings.Join(want, ",") {
			t.Errorf("level %s survivesAt = %v, want %v (the gate must really run)", row.Level, row.SurvivesAt, want)
		}
		if row.EmittedBy == "" {
			t.Errorf("level %s must name its real emitters", row.Level)
		}
	}
	if !strings.Contains(payload.Levels.Gate, "GDG_LOG_LEVEL") {
		t.Errorf("gate note must name the env dial: %q", payload.Levels.Gate)
	}
}

func TestObsViewerReplayDrawsTheCorrelation(t *testing.T) {
	payload := BuildObsViewerPayload(viewerFacts())
	if len(payload.Replay.Lines) != 3 {
		t.Fatalf("replay lines = %d, want 3", len(payload.Replay.Lines))
	}
	for _, line := range payload.Replay.Lines {
		if line.RequestID != "rid-obs-1" {
			t.Errorf("line %q lost the correlation id: %+v", line.Msg, line)
		}
	}
	if !strings.Contains(payload.Replay.Note, "rid-obs-1") {
		t.Errorf("note must draw the eye to the shared id: %q", payload.Replay.Note)
	}
	warnSeen := false
	for _, line := range payload.Replay.Lines {
		if line.Level == "WARN" && line.Msg == "error response" {
			warnSeen = true
		}
	}
	if !warnSeen {
		t.Error("the 404's warn line must be in the replay")
	}
}
