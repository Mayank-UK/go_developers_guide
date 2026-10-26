package presentation

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"guide/internal/core"
)

// The interfaces-and-composition topic's demo endpoint, exercised through
// the same middleware chain production runs behind (the family rule that
// every new demo endpoint ships with a Go test). The route lives in a test
// mux here; main.go's production wiring is the composition root's call.

func newInterfaceMatrixServer(logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/iface/matrix", NewInterfaceMatrixHandler())
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestInterfaceMatrixDemoServesThePayload(t *testing.T) {
	var buf strings.Builder
	server := newInterfaceMatrixServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodGet, "/api/demo/iface/matrix", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content type = %q, want application/json", ct)
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	// Spot-check the beats the topic's prose promises; the payload
	// builder's own tests pin the deep shape.
	decoded := struct {
		Interfaces []struct {
			Name   string `json:"name"`
			Method string `json:"method"`
		} `json:"interfaces"`
		Rows []struct {
			Expression string `json:"expression"`
			Cells      []struct {
				Iface     string `json:"iface"`
				Satisfied bool   `json:"satisfied"`
			} `json:"cells"`
		} `json:"rows"`
		Embeddings    []json.RawMessage `json:"embeddings"`
		TypeSwitch    []json.RawMessage `json:"typeSwitch"`
		CompileProofs []struct {
			Line string `json:"line"`
			Pins string `json:"pins"`
		} `json:"compileProofs"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if len(decoded.Interfaces) != 5 || decoded.Interfaces[2].Method != "Read([]byte) (int, error)" {
		t.Errorf("interfaces drifted: %+v", decoded.Interfaces)
	}
	if len(decoded.Rows) != 11 || len(decoded.Embeddings) != 3 || len(decoded.TypeSwitch) != 6 {
		t.Errorf(
			"payload faces drifted: rows=%d embeddings=%d switch=%d, want 11/3/6",
			len(decoded.Rows), len(decoded.Embeddings), len(decoded.TypeSwitch),
		)
	}
	// The method-set lesson on the wire: *core.Error is an error; the
	// core.Error VALUE is not.
	verdictOf := func(expression, iface string) *bool {
		for _, row := range decoded.Rows {
			if row.Expression != expression {
				continue
			}
			for _, cell := range row.Cells {
				if cell.Iface == iface {
					sat := cell.Satisfied
					return &sat
				}
			}
		}
		return nil
	}
	if got := verdictOf("*core.Error", "error"); got == nil || !*got {
		t.Errorf("*core.Error × error = %v, want true", got)
	}
	if got := verdictOf("core.Error", "error"); got == nil || *got {
		t.Errorf("core.Error × error = %v, want false (pointer receivers)", got)
	}
	if len(decoded.CompileProofs) == 0 || decoded.CompileProofs[0].Pins == "" {
		t.Errorf("compile proofs drifted: %+v", decoded.CompileProofs)
	}
}

func TestInterfaceMatrixDemoRejectsNonGet(t *testing.T) {
	var buf strings.Builder
	server := newInterfaceMatrixServer(newTestLogger(&buf))

	rec := doRequest(server, http.MethodPost, "/api/demo/iface/matrix", "")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"not_found"`) {
		t.Errorf("405 body is not the envelope: %s", rec.Body.String())
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("Allow header lost: %q", allow)
	}
}
