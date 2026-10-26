package core

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestLogger captures log records as text for substring assertions.
func newTestLogger(buf *strings.Builder) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, nil))
}

func requestWithID(id string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/content/nope", nil)
	if id != "" {
		r = r.WithContext(WithRequestID(r.Context(), id))
	}
	return r
}

func decodeEnvelope(t *testing.T, body string) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("envelope body is not JSON: %v\nbody: %s", err, body)
	}
	return decoded
}

func TestWriteErrorEachKindProducesStatusBodyAndHeader(t *testing.T) {
	cases := []struct {
		kind    Kind
		status  int
		message string
	}{
		{KindValidationFailed, 422, "Validation failed."},
		{KindUnauthorized, 401, "Authentication required."},
		{KindForbidden, 403, "Access forbidden."},
		{KindNotFound, 404, "Topic 'nope' not found"},
		{KindConflict, 409, "Resource conflict."},
		{KindRateLimited, 429, "Too many requests."},
		{KindServiceUnavailable, 503, "Service unavailable."},
		{KindInternal, 500, "Internal server error."},
	}
	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteError(rec, requestWithID("rid-1"), New(tc.kind, tc.message), nil)

			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}
			if got := rec.Header().Get(RequestIDHeader); got != "rid-1" {
				t.Errorf("x-request-id header = %q, want rid-1", got)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("content type = %q, want application/json", got)
			}

			decoded := decodeEnvelope(t, rec.Body.String())
			inner, ok := decoded["error"].(map[string]any)
			if !ok {
				t.Fatalf("envelope has no error object: %v", decoded)
			}
			if inner["code"] != string(tc.kind) {
				t.Errorf("code = %v, want %s", inner["code"], tc.kind)
			}
			if inner["message"] != tc.message {
				t.Errorf("message = %v, want %q", inner["message"], tc.message)
			}
			if inner["requestId"] != "rid-1" {
				t.Errorf("requestId = %v, want rid-1", inner["requestId"])
			}
			if _, hasDetails := inner["details"]; hasDetails {
				t.Error("details must be absent when unset, never null")
			}
		})
	}
}

func TestWriteErrorExactWireBody(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, requestWithID("rid-2"), ValidationFailed("Invalid scenario patch.", map[string]string{"body": "bad token"}), nil)

	got := strings.TrimSuffix(rec.Body.String(), "\n")
	want := `{"error":{"code":"validation_failed","message":"Invalid scenario patch.",` +
		`"requestId":"rid-2","details":{"body":"bad token"}}}`
	if got != want {
		t.Errorf("wire body =\n got %s\nwant %s", got, want)
	}
}

func TestWriteErrorTaxonomyMissIsGeneric500(t *testing.T) {
	rec := httptest.NewRecorder()
	underlying := errors.New("database exploded with secret details")
	WriteError(rec, requestWithID("rid-3"), underlying, nil)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "database exploded") {
		t.Errorf("internal details leaked into the envelope: %s", body)
	}
	decoded := decodeEnvelope(t, body)
	inner := decoded["error"].(map[string]any)
	if inner["code"] != string(KindInternal) || inner["message"] != "Internal server error." {
		t.Errorf("taxonomy miss must degrade to the generic internal envelope, got %v", inner)
	}
	if inner["requestId"] != "rid-3" {
		t.Errorf("requestId = %v, want rid-3", inner["requestId"])
	}
}

func TestWriteErrorUnknownKindIsATaxonomyMiss(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, requestWithID("rid-4"), New(Kind("made_up"), "should never happen"), nil)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, "made_up") {
		t.Errorf("unknown kind leaked into the envelope: %s", body)
	}
}

func TestWriteErrorWithoutRequestIDUsesEmptyString(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, requestWithID(""), NotFound("gone"), nil)
	decoded := decodeEnvelope(t, rec.Body.String())
	if inner := decoded["error"].(map[string]any); inner["requestId"] != "" {
		t.Errorf("requestId = %v, want the honest empty string", inner["requestId"])
	}
}

func TestWriteErrorLogsThroughGivenLogger(t *testing.T) {
	var buf strings.Builder
	logger := newTestLogger(&buf)

	rec := httptest.NewRecorder()
	WriteError(rec, requestWithID("rid-5"), NotFound("gone"), logger)
	if !strings.Contains(buf.String(), "error response") || !strings.Contains(buf.String(), "not_found") {
		t.Errorf("taxonomy hit not logged at warn: %s", buf.String())
	}

	buf.Reset()
	rec = httptest.NewRecorder()
	WriteError(rec, requestWithID("rid-6"), errors.New("plain failure"), logger)
	logged := buf.String()
	if !strings.Contains(logged, "unhandled error") || !strings.Contains(logged, "plain failure") {
		t.Errorf("taxonomy miss not logged at error with the underlying cause: %s", logged)
	}
	if !strings.Contains(logged, "level=ERROR") {
		t.Errorf("taxonomy miss must log at error level: %s", logged)
	}
}
