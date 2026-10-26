package presentation

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandlerRouting(t *testing.T) {
	t.Parallel()

	// Route through a real ServeMux so the test also proves the handler is
	// registered with the method+pattern syntax ("GET /api/health").
	mux := http.NewServeMux()
	mux.Handle("GET /api/health", NewHealthHandler("dev"))

	tests := []struct {
		name       string
		method     string
		target     string
		wantStatus int
		wantBody   string // exact body, checked on 200 only
	}{
		{
			name:       "GET reports liveness",
			method:     http.MethodGet,
			target:     "/api/health",
			wantStatus: http.StatusOK,
			wantBody:   "{\"status\":\"ok\",\"env\":\"dev\"}\n",
		},
		{
			name:       "non-GET method is rejected",
			method:     http.MethodPost,
			target:     "/api/health",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "unknown path is not found",
			method:     http.MethodGet,
			target:     "/api/healthz",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.target, nil))

			if rec.Code != tt.wantStatus {
				t.Fatalf("%s %s: status = %d, want %d", tt.method, tt.target, rec.Code, tt.wantStatus)
			}
			if tt.wantBody == "" {
				return
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want %q", got, "application/json")
			}
			if got := rec.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
		})
	}
}

func TestHealthHandlerReportsBootEnv(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	NewHealthHandler("prod").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	want := "{\"status\":\"ok\",\"env\":\"prod\"}\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}
