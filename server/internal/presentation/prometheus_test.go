package presentation

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPrometheusHandlerRouting(t *testing.T) {
	t.Parallel()

	// Route through a real ServeMux so the test also proves the handler is
	// registered with the method+pattern syntax ("GET /api/metrics") — the
	// same idiom the health route's test pins.
	mux := http.NewServeMux()
	mux.Handle("GET /api/metrics", NewPrometheusHandler())

	tests := []struct {
		name        string
		method      string
		target      string
		wantStatus  int
		wantMetrics []string // substrings the 200 body must carry
	}{
		{
			name:        "GET scrapes the default runtime exposition",
			method:      http.MethodGet,
			target:      "/api/metrics",
			wantStatus:  http.StatusOK,
			wantMetrics: []string{"go_goroutines", "go_info"},
		},
		{
			name:       "non-GET method is rejected",
			method:     http.MethodPost,
			target:     "/api/metrics",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "unknown path is not found",
			method:     http.MethodGet,
			target:     "/api/metric",
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
			if len(tt.wantMetrics) == 0 {
				return
			}
			// The exposition format's registered content type — what a
			// Prometheus scraper (and curl) reads unambiguously. client_golang
			// v1.25+ appends an escaping parameter (; escaping=underscores),
			// so pin the stable prefix, not the full parameter list.
			if got, want := rec.Header().Get("Content-Type"), "text/plain; version=0.0.4; charset=utf-8"; !strings.HasPrefix(got, want) {
				t.Errorf("Content-Type = %q, want prefix %q", got, want)
			}
			body := rec.Body.String()
			for _, metric := range tt.wantMetrics {
				// The runtime collector is pure runtime — present on every
				// platform, unlike the process collector's procfs-backed
				// families, so the pin holds on darwin and CI alike.
				if !strings.Contains(body, metric) {
					t.Errorf("body lacks default metric %q:\n%s", metric, body)
				}
			}
		})
	}
}

func TestPrometheusHandlerInstancesAreIndependentlyConstructible(t *testing.T) {
	t.Parallel()

	// The dedicated-registry design means constructing the handler twice
	// is safe — no duplicate-collector panic on a shared global registry.
	// Both instances must serve a complete exposition.
	for i := range 2 {
		rec := httptest.NewRecorder()
		NewPrometheusHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/metrics", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("instance %d: status = %d, want %d", i, rec.Code, http.StatusOK)
		}
		if !strings.Contains(rec.Body.String(), "go_goroutines") {
			t.Fatalf("instance %d: body lacks go_goroutines", i)
		}
	}
}
