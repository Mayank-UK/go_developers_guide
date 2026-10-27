package presentation

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"guide/internal/core"
	"guide/internal/domain/demos"
)

// The messaging topic's two demo endpoints, exercised through the same
// middleware chain production runs behind (the family rule that every
// new demo endpoint ships with a Go test). Both runner ports are faked
// here — the go-redis/pgx-backed story runners have their own tests
// against the running stack stores in internal/data/messaging — so the
// happy paths and the honest 503 degradations are pinned without stores.

// fakeMessagingStreamRunner stands in for the story runner: it returns a
// canned run shaped like a real walk, or the failure its field carries.
type fakeMessagingStreamRunner struct {
	run demos.MessagingStreamRun
	err error
}

func (f fakeMessagingStreamRunner) RunStreamStory(_ context.Context, _ string) (demos.MessagingStreamRun, error) {
	return f.run, f.err
}

// fakeMessagingOutboxRunner mirrors the stream fake for the outbox story.
type fakeMessagingOutboxRunner struct {
	run demos.MessagingOutboxRun
	err error
}

func (f fakeMessagingOutboxRunner) RunOutboxStory(_ context.Context, _ string) (demos.MessagingOutboxRun, error) {
	return f.run, f.err
}

// messagingStreamFixtureRun mirrors a real stream story closely enough to
// pin the handler's mapping.
func messagingStreamFixtureRun() demos.MessagingStreamRun {
	return demos.MessagingStreamRun{
		Namespace: "gdg:demo:msg:0123456789ab",
		Stream:    "gdg:demo:msg:0123456789ab:orders",
		Group:     demos.MessagingDemoGroup,
		AppendedIDs: []string{
			"1780000000000-0",
			"1780000000001-0",
			"1780000000002-0",
		},
		ConsumerARead: []demos.MessagingEntryRow{
			{ID: "1780000000000-0", Step: "created"},
			{ID: "1780000000001-0", Step: "paid"},
			{ID: "1780000000002-0", Step: "shipped"},
		},
		AckedByA:  []string{"1780000000000-0", "1780000000002-0"},
		StalledID: "1780000000001-0",
		PendingBefore: []demos.MessagingPendingRow{
			{ID: "1780000000001-0", Consumer: "consumer-a", Deliveries: 1},
		},
		ClaimedByB: []demos.MessagingEntryRow{
			{ID: "1780000000001-0", Step: "paid"},
		},
		GroupDestroyed: true,
		StreamDeleted:  true,
	}
}

// messagingOutboxFixtureRun mirrors a real outbox story.
func messagingOutboxFixtureRun() demos.MessagingOutboxRun {
	return demos.MessagingOutboxRun{
		Namespace: "gdg:demo:msg:0123456789ab",
		Stream:    "gdg:demo:msg:0123456789ab:outbox",
		Recorded: []demos.MessagingRecordedEventRow{
			{ID: "m-aaaaaaaaaaaaaaaa", Kind: "order.created", Payload: "order o-1001 created", Committed: true},
			{ID: "m-bbbbbbbbbbbbbbbb", Kind: "order.paid", Payload: "order o-1001 paid", Committed: true},
		},
		RolledBackEventID: "m-cccccccccccccccc",
		Relay: demos.MessagingRelayRow{Published: []demos.MessagingPublishedRow{
			{OutboxID: "m-aaaaaaaaaaaaaaaa", StreamEntry: "1780000000000-0"},
			{OutboxID: "m-bbbbbbbbbbbbbbbb", StreamEntry: "1780000000001-0"},
		}},
		Consumed: []demos.MessagingDeliveryRow{
			{EntryID: "1780000000000-0", MessageID: "m-aaaaaaaaaaaaaaaa", Payload: "order o-1001 created", FirstTime: true},
			{EntryID: "1780000000001-0", MessageID: "m-bbbbbbbbbbbbbbbb", Payload: "order o-1001 paid", FirstTime: true},
		},
		Redelivered: []demos.MessagingDeliveryRow{
			{EntryID: "1780000000000-0", MessageID: "m-aaaaaaaaaaaaaaaa", Payload: "order o-1001 created", FirstTime: false},
		},
		DuplicatesRefused: 1,
		EventsDeleted:     2,
		OutboxDeleted:     2,
		GroupDestroyed:    true,
		StreamDeleted:     true,
	}
}

func newMessagingDemoServer(t *testing.T) http.Handler {
	t.Helper()
	logger := newTestLogger(&strings.Builder{})
	streamRunner := fakeMessagingStreamRunner{run: messagingStreamFixtureRun()}
	outboxRunner := fakeMessagingOutboxRunner{run: messagingOutboxFixtureRun()}
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/msg/stream", NewMessagingStreamHandler(streamRunner, logger))
	mux.Handle("GET /api/demo/msg/outbox", NewMessagingOutboxHandler(outboxRunner, logger))
	return Chain(
		RequestID,
		RequestLogger(logger),
		Recover(logger),
		EnvelopeMuxErrors(logger),
	)(mux)
}

func TestMessagingStreamDemoServesTheShapedRun(t *testing.T) {
	server := newMessagingDemoServer(t)

	rec := doRequest(server, http.MethodGet, "/api/demo/msg/stream", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content type = %q, want application/json", ct)
	}
	if rec.Header().Get(core.RequestIDHeader) == "" {
		t.Error("every response must carry x-request-id")
	}

	decoded := struct {
		Namespace string `json:"namespace"`
		Stream    string `json:"stream"`
		Beats     []struct {
			Beat     string   `json:"beat"`
			Commands []string `json:"commands"`
		} `json:"beats"`
		Proofs struct {
			Ordering    string `json:"ordering"`
			PendingLeft int64  `json:"pendingLeft"`
		} `json:"proofs"`
		Reasoning string `json:"reasoning"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if !strings.HasPrefix(decoded.Namespace, demos.MessagingDemoKeyPrefix+":") {
		t.Errorf("namespace = %q, want under the messaging prefix (minted per request)", decoded.Namespace)
	}
	if len(decoded.Beats) != 8 {
		t.Errorf("beats = %d, want 8", len(decoded.Beats))
	}
	if !strings.Contains(decoded.Proofs.Ordering, "created → paid → shipped") {
		t.Errorf("ordering proof must carry the sequence: %q", decoded.Proofs.Ordering)
	}
	if decoded.Proofs.PendingLeft != 0 {
		t.Errorf("pendingLeft = %d, want 0", decoded.Proofs.PendingLeft)
	}
	if !strings.Contains(decoded.Reasoning, "consumer GROUP") {
		t.Errorf("reasoning must carry the queue-vs-fanout thesis: %q", decoded.Reasoning)
	}
}

func TestMessagingOutboxDemoServesTheShapedRun(t *testing.T) {
	server := newMessagingDemoServer(t)

	rec := doRequest(server, http.MethodGet, "/api/demo/msg/outbox", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	decoded := struct {
		Stream string `json:"stream"`
		Beats  []struct {
			Beat string `json:"beat"`
		} `json:"beats"`
		Proofs struct {
			Atomicity    string `json:"atomicity"`
			RelayProof   string `json:"relayProof"`
			Idempotency  string `json:"idempotency"`
			EventDeleted int64  `json:"eventsDeleted"`
		} `json:"proofs"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("body not the demo DTO shape: %v\n%s", err, rec.Body.String())
	}
	if !strings.HasSuffix(decoded.Stream, ":outbox") {
		t.Errorf("stream = %q, want the run's outbox stream", decoded.Stream)
	}
	if len(decoded.Beats) != 6 {
		t.Errorf("beats = %d, want 6", len(decoded.Beats))
	}
	if !strings.Contains(decoded.Proofs.Atomicity, "answered 0") {
		t.Errorf("atomicity proof must derive from the rollback's absence: %q", decoded.Proofs.Atomicity)
	}
	if !strings.Contains(decoded.Proofs.Idempotency, "refused 1 time(s)") {
		t.Errorf("idempotency proof must derive from the refused duplicate: %q", decoded.Proofs.Idempotency)
	}
}

func TestMessagingDemosDegradeTo503WhenTheStoresAreUnreachable(t *testing.T) {
	logger := slog.Default()
	mux := http.NewServeMux()
	mux.Handle("GET /api/demo/msg/stream",
		NewMessagingStreamHandler(fakeMessagingStreamRunner{err: errors.New("dial tcp: connection refused")}, logger))
	mux.Handle("GET /api/demo/msg/outbox",
		NewMessagingOutboxHandler(fakeMessagingOutboxRunner{err: errors.New("pool closed")}, logger))
	server := Chain(RequestID, RequestLogger(logger), Recover(logger), EnvelopeMuxErrors(logger))(mux)

	for _, path := range []string{"/api/demo/msg/stream", "/api/demo/msg/outbox"} {
		rec := doRequest(server, http.MethodGet, path, "")
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s status = %d, want 503", path, rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `"code":"service_unavailable"`) {
			t.Errorf("%s 503 body is not the honest envelope: %s", path, body)
		}
		if !strings.Contains(body, "not") || !strings.Contains(body, "reachable right now") {
			t.Errorf("%s 503 body must say what is wrong in plain words: %s", path, body)
		}
		if rid := rec.Header().Get(core.RequestIDHeader); rid == "" {
			t.Errorf("%s 503 must still carry x-request-id", path)
		}
	}
}

func TestMessagingDemosRejectNonGet(t *testing.T) {
	server := newMessagingDemoServer(t)

	for _, path := range []string{"/api/demo/msg/stream", "/api/demo/msg/outbox"} {
		rec := doRequest(server, http.MethodPost, path, "")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("POST %s status = %d, want 405", path, rec.Code)
		}
		if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
			t.Errorf("%s Allow header lost: %q", path, allow)
		}
	}
}
