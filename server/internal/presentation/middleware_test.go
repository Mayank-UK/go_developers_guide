package presentation

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"guide/internal/content"
	"guide/internal/core"
	postsmemory "guide/internal/data/posts"
	"guide/internal/domain/demos"
	"guide/internal/domain/posts"
)

// newTestLogger captures log records as text for substring assertions.
func newTestLogger(buf *strings.Builder) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, nil))
}

// harnessIntrospector feeds the middleware-test harness's schema-explorer
// route a one-table snapshot: the chain's shape is under test here, not
// the introspector (which the data layer tests against real PostgreSQL).
type harnessIntrospector struct{}

func (harnessIntrospector) Introspect(context.Context) (demos.SchemaSnapshot, error) {
	return demos.SchemaSnapshot{
		Tables:  []string{"posts"},
		Columns: []demos.ColumnRow{{Table: "posts", Name: "id", Type: "uuid"}},
	}, nil
}

// newAPIServer composes the exact chain main.go wires, over a mux serving
// the Phase 1 routes — handler tests exercise the REAL server shape, not
// bare handlers.
func newAPIServer(registry content.Registry, store *core.ScenarioStore, logger *slog.Logger) http.Handler {
	if store == nil {
		store = core.NewScenarioStore()
	}
	mux := http.NewServeMux()
	mux.Handle("GET /api/health", NewHealthHandler("test"))
	mux.Handle("GET /api/content", NewContentListHandler(registry))
	mux.Handle("GET /api/content/{topicId}", NewContentTopicHandler(registry, logger))
	mux.Handle("GET /api/demo/scenario", NewScenarioGetHandler(store))
	mux.Handle("PATCH /api/demo/scenario", NewScenarioPatchHandler(store, logger))
	mux.Handle("GET /api/demo/go/runtime-types", NewRuntimeTypesHandler())
	mux.Handle("GET /api/demo/runtime/concurrency", NewConcurrencyHandler())
	mux.Handle("GET /api/demo/http/headers", NewHTTPHeadersHandler())
	mux.Handle("GET /api/demo/gohttp/app-info", NewAppInfoHandler(core.Config{
		Port:     8100,
		Env:      "test",
		LogLevel: slog.LevelInfo,
	}))
	mux.Handle("GET /api/demo/iface/matrix", NewInterfaceMatrixHandler())
	mux.Handle("GET /api/demo/conc/context", NewContextCancellationHandler())
	mux.Handle("GET /api/demo/conc/race", NewRaceDetectorHandler())
	mux.Handle("GET /api/demo/routing/explorer", NewRoutingExplorerHandler())
	mux.Handle("GET /api/demo/middleware/pipeline", NewMiddlewarePipelineHandler())
	mux.Handle("GET /api/demo/reqres/inspector", NewReqresInspectorHandler())
	mux.Handle("GET /api/demo/reqres/methods", NewReqresMethodsHandler())
	mux.Handle("GET /api/demo/validation/arena", NewValidationArenaHandler())
	mux.Handle("GET /api/demo/api/webhook-simulator", NewWebhookSimulatorHandler())
	mux.Handle("GET /api/demo/api/docs-link", NewAPIDocsLinkHandler(logger))
	mux.Handle("GET /api/openapi.yaml", NewOpenAPISpecHandler())
	mux.Handle("GET /api/demo/api/grpc-console", NewGRPCConsoleHandler())
	// The db demos ride a FAKE introspector here (the real one needs the
	// live PostgreSQL; its own package tests exercise that against a
	// throwaway database) — enough to exercise the full chain shape.
	mux.Handle("GET /api/demo/db/schema-explorer", NewDBSchemaExplorerHandler(harnessIntrospector{}, logger))
	mux.Handle("GET /api/demo/db/uuid", NewDBUUIDHandler(logger))
	// The GraphQL pair mirrors main.go: the endpoint over its own posts
	// stack (the handler tests exercise them through dedicated muxes with
	// seeded stores; here the chain shape is what matters).
	postsRepo := postsmemory.NewRepository()
	postsList := posts.NewListPostsUseCase(postsRepo)
	postsCreate := posts.NewCreatePostUseCase(postsRepo)
	postsGet := posts.NewGetPostUseCase(postsRepo)
	postsReplace := posts.NewReplacePostUseCase(postsRepo)
	postsDelete := posts.NewDeletePostUseCase(postsRepo)
	mux.Handle("GET /api/demo/api/graphql-playground", NewGraphQLPlaygroundHandler(func() posts.PostsRepository {
		return postsmemory.NewRepository()
	}))
	mux.Handle("POST /api/graphql", NewGraphQLHandler(postsList, postsGet, postsCreate, postsReplace, postsDelete, logger))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestRequestIDHonorsInboundHeader(t *testing.T) {
	var seen string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = core.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if seen == "" || rec.Header().Get(core.RequestIDHeader) != seen {
		t.Errorf("inbound absent: context %q, header %q — want the same generated id", seen, rec.Header().Get(core.RequestIDHeader))
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(core.RequestIDHeader, "client-42")
	handler.ServeHTTP(rec, req)
	if seen != "client-42" {
		t.Errorf("context id = %q, want the honored inbound client-42", seen)
	}
	if got := rec.Header().Get(core.RequestIDHeader); got != "client-42" {
		t.Errorf("response header = %q, want client-42 echoed", got)
	}
}

func TestRequestIDWhitespaceOnlyHeaderIsReplaced(t *testing.T) {
	var seen string
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = core.RequestIDFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(core.RequestIDHeader, "   ")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if seen == "" || strings.TrimSpace(seen) != seen || seen == "   " {
		t.Errorf("whitespace-only inbound id must be replaced, got %q", seen)
	}
}

func TestRequestIDCapsOverlongInboundHeader(t *testing.T) {
	var seen string
	handler := RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = core.RequestIDFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(core.RequestIDHeader, strings.Repeat("x", 300))
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if len(seen) != core.MaxInboundRequestIDLength {
		t.Errorf("overlong inbound id capped to %d chars, got %d", core.MaxInboundRequestIDLength, len(seen))
	}
}

func TestChainAppliesFirstArgumentOutermost(t *testing.T) {
	var order []string
	marker := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, "in:"+name)
				next.ServeHTTP(w, r)
				order = append(order, "out:"+name)
			})
		}
	}
	handler := Chain(marker("A"), marker("B"), marker("C"))(http.NotFoundHandler())
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	got := strings.Join(order, ",")
	want := "in:A,in:B,in:C,out:C,out:B,out:A"
	if got != want {
		t.Errorf("chain order = %s, want %s", got, want)
	}
}

func TestRequestLoggerWritesOneLinePerRequest(t *testing.T) {
	var buf strings.Builder
	server := Chain(RequestID, RequestLogger(newTestLogger(&buf)))(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		}))
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set(core.RequestIDHeader, "rid-log")
	server.ServeHTTP(httptest.NewRecorder(), req)

	logged := buf.String()
	for _, want := range []string{"http_request", "method=GET", "path=/api/health", "status=418", "duration_ms=", "request_id=rid-log", "level=INFO"} {
		if !strings.Contains(logged, want) {
			t.Errorf("access log missing %q:\n%s", want, logged)
		}
	}
}

func TestRecoverWrites500EnvelopeAndLogsStack(t *testing.T) {
	var buf strings.Builder
	logger := newTestLogger(&buf)
	server := Chain(RequestID, RequestLogger(logger), Recover(logger))(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("boom: programmer bug")
		}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/demo/panic", nil)
	req.Header.Set(core.RequestIDHeader, "rid-panic")
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"code":"internal"`) || !strings.Contains(body, `"requestId":"rid-panic"`) {
		t.Errorf("panic response is not the internal envelope: %s", body)
	}
	logged := buf.String()
	if !strings.Contains(logged, "panic recovered") || !strings.Contains(logged, "level=ERROR") || !strings.Contains(logged, "goroutine") {
		t.Errorf("panic not logged at error with a stack:\n%s", logged)
	}
	if !strings.Contains(logged, "rid-panic") {
		t.Errorf("panic log missing the request id:\n%s", logged)
	}
}

func TestRecoverLeavesCommittedResponsesAlone(t *testing.T) {
	// A panic after bytes have gone out cannot be replaced by an envelope;
	// the real chain sees the committed response (via the logger's
	// recorder) and logs the panic without writing over what was sent.
	var buf strings.Builder
	logger := newTestLogger(&buf)
	server := Chain(RequestID, RequestLogger(logger), Recover(logger))(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("partial"))
			panic("mid-stream failure")
		}))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK || rec.Body.String() != "partial" {
		t.Errorf("committed response = %d %q, want 200 partial", rec.Code, rec.Body.String())
	}
	if !strings.Contains(buf.String(), "panic recovered") {
		t.Error("mid-stream panic must still be logged")
	}
}

func TestRecoverReraisesErrAbortHandler(t *testing.T) {
	var reraised any
	func() {
		defer func() { reraised = recover() }()
		var buf strings.Builder
		logger := newTestLogger(&buf)
		Recover(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic(http.ErrAbortHandler)
		})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}()
	if reraised != http.ErrAbortHandler {
		t.Errorf("ErrAbortHandler must be re-raised untouched, got %v", reraised)
	}
}

func TestEnvelopeMuxErrorsConvertsCannedReplies(t *testing.T) {
	var buf strings.Builder
	logger := newTestLogger(&buf)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/content", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /api/own-404", func(w http.ResponseWriter, r *http.Request) {
		core.WriteError(w, r, core.NotFound("custom missing"), logger)
	})
	server := EnvelopeMuxErrors(logger)(mux)

	// Unknown path: plain mux 404 → not_found envelope.
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/ghost", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"code":"not_found"`) || !strings.Contains(body, `"message":"Not Found"`) {
		t.Errorf("unknown path body is not the envelope: %s", body)
	}

	// Method mismatch: truthful 405 + Allow preserved, closest kind
	// (not_found, mirroring the fastapi mapping).
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/content", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("Allow header = %q, want it to keep GET", allow)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"code":"not_found"`) || !strings.Contains(body, `"message":"Method Not Allowed"`) {
		t.Errorf("method mismatch body is not the envelope: %s", body)
	}

	// A handler's own 404 envelope is replayed verbatim, never replaced.
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/own-404", nil))
	if body := rec.Body.String(); !strings.Contains(body, "custom missing") {
		t.Errorf("handler's own 404 body was rewritten: %s", body)
	}

	// Normal responses pass straight through.
	rec = httptest.NewRecorder()
	server.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/content", nil))
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Errorf("normal response = %d %q, want untouched 200", rec.Code, rec.Body.String())
	}
}
