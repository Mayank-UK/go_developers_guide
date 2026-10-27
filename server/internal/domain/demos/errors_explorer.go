// Topic 30 error-handling's live demo: the errors explorer. One row per
// failure kind in the core taxonomy — status and default message derived
// from core's own tables (never restated here), each row carrying the
// real code that raises the kind, the log line its handling writes, and
// the trigger that fires it against the running stack (or the honest
// recipe note when the stack cannot produce that failure live). The
// failure-injection half is REAL: the demo area's own probe endpoint
// honors the scenario store's injectFailure toggle — the explorer walks
// it through the same middleware chain production serves, on every
// request, and reports exactly what it answered. The builder below is
// PURE: every derived sentence comes from the walk's observed facts, so
// the payload can never claim a failure the stack did not produce.
package demos

import (
	"errors"
	"fmt"
	"time"

	"guide/internal/core"
)

// ErrorsProbePath is the demo area's own guarded endpoint — the probe
// the explorer walks and the terminal tour curls. It is registered on
// the real mux by the composition root, so a plain curl exercises the
// exact code path the explorer narrates.
const ErrorsProbePath = "/api/demo/errors/probe"

// ErrorsScenarioPath is the scenario store's real endpoints — the
// runtime toggle the probe honors and the panel's scenario row names.
const ErrorsScenarioPath = "/api/demo/scenario"

// ErrorsProbeLatency is the delay the probe sleeps out when the
// scenario's latency injection is armed — bounded well inside the
// server's write timeout, long enough to feel in a terminal tour.
const ErrorsProbeLatency = 1500 * time.Millisecond

// ErrErrorsProbeInjected is the value the probe panics with when the
// scenario's error injection is armed — a deliberate, programmer-bug
// shaped failure. The presentation stack's Recover layer catches it,
// logs it at error with the stack and the request id, and answers the
// generic 500 internal envelope: the panic policy this topic teaches,
// performed live on every armed probe call.
var ErrErrorsProbeInjected = errors.New(
	"errors probe: failure injection armed — a deliberate programmer-bug-shaped panic, " +
		"recovered by the middleware stack",
)

// Trigger mode strings — a row either fires its kind against the
// running stack (live) or carries the recipe that would (recipe).
const (
	// ErrorsTriggerLive names a curl-able trigger.
	ErrorsTriggerLive = "live"
	// ErrorsTriggerRecipe names a trigger whose conditions the running
	// stack honestly does not have.
	ErrorsTriggerRecipe = "recipe"
)

// ErrorsTriggerSpec is how to fire one kind for real — or why only a
// recipe exists. Wire keys mirror the fastapi sibling's panel exactly.
type ErrorsTriggerSpec struct {
	Mode   string `json:"mode"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Body   string `json:"body,omitempty"`
	Prep   string `json:"prep,omitempty"`
	Note   string `json:"note"`
}

// ErrorsKindRow is one failure kind: status, default message, raiser,
// log line, and trigger. Status and message come from core's own
// tables; raised-by and logged-as name the real raise sites.
type ErrorsKindRow struct {
	Kind           string            `json:"kind"`
	HTTPStatus     int               `json:"httpStatus"`
	DefaultMessage string            `json:"defaultMessage"`
	RaisedBy       string            `json:"raisedBy"`
	LoggedAs       string            `json:"loggedAs"`
	Trigger        ErrorsTriggerSpec `json:"trigger"`
}

// ErrorsScenarioRow is the scenario sub-panel: the real GET/PATCH
// endpoints and the three injection modes.
type ErrorsScenarioRow struct {
	GetEndpoint   string   `json:"getEndpoint"`
	PatchEndpoint string   `json:"patchEndpoint"`
	Modes         []string `json:"modes"`
	Note          string   `json:"note"`
}

// ErrorsProbeRow is the probe panel: the endpoint, the armed latency,
// and what each mode answers.
type ErrorsProbeRow struct {
	ProbeEndpoint string `json:"probeEndpoint"`
	LatencyMillis int64  `json:"latencyMillis"`
	HealthyNote   string `json:"healthyNote"`
	InjectedNote  string `json:"injectedNote"`
	LatencyNote   string `json:"latencyNote"`
}

// ErrorsWalk is one live walk of the probe as the explorer observed it:
// the injection mode that was armed, the status and elapsed time the
// probe answered with, the request id threaded through the walk, and
// the body the probe wrote (the healthy row or the error envelope).
type ErrorsWalk struct {
	InjectFailure core.FailureInjection
	Status        int
	Elapsed       time.Duration
	RequestID     string
	Body          string
}

// ErrorsWalkRow is the walk's wire shape inside the payload, with the
// derived note appended.
type ErrorsWalkRow struct {
	InjectFailure string `json:"injectFailure"`
	Status        int    `json:"status"`
	ElapsedMillis int64  `json:"elapsedMillis"`
	RequestID     string `json:"requestID"`
	Body          string `json:"body"`
	Note          string `json:"note"`
}

// ErrorsExplorerPayload is the wire shape of GET /api/demo/errors/explorer.
type ErrorsExplorerPayload struct {
	Kinds        []ErrorsKindRow   `json:"kinds"`
	EnvelopeNote string            `json:"envelopeNote"`
	Scenario     ErrorsScenarioRow `json:"scenario"`
	Probe        ErrorsProbeRow    `json:"probe"`
	Walk         ErrorsWalkRow     `json:"walk"`
	Reasoning    string            `json:"reasoning"`
}

// ErrorsProbeAnswer is the probe's healthy wire shape — what GET
// /api/demo/errors/probe answers when no error injection is armed
// (and, after sleeping, under latency injection).
type ErrorsProbeAnswer struct {
	Status        string `json:"status"`
	InjectFailure string `json:"injectFailure"`
	Detail        string `json:"detail"`
}

// errorsKindOrder is the taxonomy's declared order — the order the
// payload's table walks and the topic's prose teaches.
var errorsKindOrder = []core.Kind{
	core.KindValidationFailed,
	core.KindUnauthorized,
	core.KindForbidden,
	core.KindNotFound,
	core.KindConflict,
	core.KindRateLimited,
	core.KindServiceUnavailable,
	core.KindInternal,
}

// errorsSentinelFor maps a kind onto its package-level sentinel, so the
// table's default message is read from core's own values instead of
// being restated here (the drift the single-source rule prevents).
func errorsSentinelFor(kind core.Kind) *core.Error {
	switch kind {
	case core.KindValidationFailed:
		return core.ErrValidationFailed
	case core.KindUnauthorized:
		return core.ErrUnauthorized
	case core.KindForbidden:
		return core.ErrForbidden
	case core.KindNotFound:
		return core.ErrNotFound
	case core.KindConflict:
		return core.ErrConflict
	case core.KindRateLimited:
		return core.ErrRateLimited
	case core.KindServiceUnavailable:
		return core.ErrServiceUnavailable
	default:
		return core.ErrInternal
	}
}

// errorsKindContext carries the raiser, the log line, and the trigger
// for one kind — the table's teaching half, keyed by kind.
var errorsKindContext = map[core.Kind]struct {
	raisedBy string
	loggedAs string
	trigger  ErrorsTriggerSpec
}{
	core.KindValidationFailed: {
		raisedBy: "the boundary decoders and the store's own guard — the scenario PATCH's " +
			"decodeScenarioPatch rejects a malformed body, and ScenarioStore.Update refuses " +
			"an unknown enum value (internal/presentation/scenario.go, internal/core/scenario.go)",
		loggedAs: "error response · warn — kind, message, status, request id, and the full error chain",
		trigger: ErrorsTriggerSpec{
			Mode:   ErrorsTriggerLive,
			Method: "PATCH",
			Path:   ErrorsScenarioPath,
			Body:   `{"injectFailure": "bogus"}`,
			Note: "an enum value the merged patch refuses — the 422 envelope answers with " +
				"details carrying the value sent and the values expected",
		},
	},
	core.KindUnauthorized: {
		raisedBy: "RequireIdentity — the topic 23 middleware every guarded route wraps: no " +
			"credential carrier at all, a malformed Bearer header, or a credential that " +
			"fails verification (internal/presentation/auth_middleware.go)",
		loggedAs: "error response · warn — one uniform envelope for every failure half",
		trigger: ErrorsTriggerSpec{
			Mode:   ErrorsTriggerLive,
			Method: "GET",
			Path:   "/api/demo/auth/whoami",
			Note: "the guarded whoami with no credential headers at all — the uniform 401 " +
				"envelope, teaching the caller nothing about which half failed",
		},
	},
	core.KindForbidden: {
		raisedBy: "RequirePermission and RequireOwnership — the topic 24 policy gates, " +
			"denying an AUTHENTICATED identity that lacks the permission or the object " +
			"ownership (internal/presentation/authz_middleware.go)",
		loggedAs: "error response · warn — the deny reason rides details",
		trigger: ErrorsTriggerSpec{
			Mode:   ErrorsTriggerRecipe,
			Method: "GET",
			Path:   "/api/demo/authz/guarded",
			Note: "no curl-mintable credential exists in this stack by design — the authz " +
				"matrix story (GET /api/demo/authz/matrix) carries the 403 verdicts live, " +
				"and the topic's Go tests compose the real gates for the wire mapping",
		},
	},
	core.KindNotFound: {
		raisedBy: "use cases and handlers raise it directly — GetPostUseCase for a " +
			"well-formed id that names nothing, the content handler's unknown topic, and " +
			"the mux-error wrapper for the router's own 404",
		loggedAs: "error response · warn — the plain miss, logged like every other kind",
		trigger: ErrorsTriggerSpec{
			Mode:   ErrorsTriggerLive,
			Method: "GET",
			Path:   "/api/posts/no-such-post",
			Note: "a well-formed id that names nothing — the plain 404 envelope through " +
				"the whole chain, envelope-wrapped even when the router itself missed",
		},
	},
	core.KindConflict: {
		raisedBy: "reserved for genuine state conflicts — a duplicate unique key, a lost " +
			"update, a double enqueue; the gRPC surface maps it onto AlreadyExists " +
			"(internal/grpcapi/server.go)",
		loggedAs: "error response · warn — when it fires, it fires like every other kind",
		trigger: ErrorsTriggerSpec{
			Mode:   ErrorsTriggerRecipe,
			Method: "POST",
			Path:   "/api/posts",
			Note: "no live trigger: nothing in the guide's runtime conflicts yet — the kind " +
				"exists so the day a unique violation or duplicate submission surfaces, it " +
				"maps without inventing a new status",
		},
	},
	core.KindRateLimited: {
		raisedBy: "the topic 25 rate-limit middleware — the exhausted budget raises with " +
			"Retry-After riding the response headers " +
			"(internal/presentation/security_ratelimit.go)",
		loggedAs: "error response · warn — kind, status, and the request id of the refused call",
		trigger: ErrorsTriggerSpec{
			Mode:   ErrorsTriggerLive,
			Method: "GET",
			Path:   "/api/demo/security/lab",
			Prep:   "hammer it in a tight loop — the terminal tour's 12-call burst is enough",
			Note: "the route's shared limiter (burst 5, 10/s refill, keyed per client): rapid " +
				"calls spend the budget around the eighth request, the 429 envelope carries " +
				"Retry-After, and the budget self-heals at the refill rate while you watch",
		},
	},
	core.KindServiceUnavailable: {
		raisedBy: "the optional-store adapters' degradation guards — the jobs console and " +
			"the redis playground refuse stale answers when their store is unreachable " +
			"(internal/data/jobs, internal/data/redis)",
		loggedAs: "error response · warn — the adapter names exactly what is unreachable",
		trigger: ErrorsTriggerSpec{
			Mode:   ErrorsTriggerRecipe,
			Method: "GET",
			Path:   "/api/demo/jobs/console",
			Prep:   "docker stop go_developers_guide_redis_dev — and start it again after",
			Note: "needs the optional store down, which the running stack honestly is not: " +
				"stop redis, call the console, watch ONLY that demo degrade while the app " +
				"keeps serving everything else",
		},
	},
	core.KindInternal: {
		raisedBy: "nothing raises internal on purpose — it is what the Recover layer " +
			"answers for a panic and what the central writer answers for an error outside " +
			"the taxonomy",
		loggedAs: "panic recovered · error — the recovered value, the stack, and the request " +
			"id; or unhandled error · error for a taxonomy miss",
		trigger: ErrorsTriggerSpec{
			Mode:   ErrorsTriggerLive,
			Method: "GET",
			Path:   ErrorsProbePath,
			Prep:   `PATCH /api/demo/scenario {"injectFailure": "error"} — then reset it to "none" after`,
			Note: "the scenario's failure injection honored by this demo's own probe: the " +
				"probe panics deliberately, the Recover layer converts it to the generic 500 " +
				"envelope — no stack trace to the client, all of it in the log",
		},
	},
}

// BuildErrorsExplorerPayload shapes the panel (pure) from core's own
// tables plus the live walk the handler observed: statuses come from
// core.HTTPStatusFor, default messages from the kind sentinels, and
// the walk's note derives from what the probe actually answered.
func BuildErrorsExplorerPayload(walk ErrorsWalk) ErrorsExplorerPayload {
	kinds := make([]ErrorsKindRow, 0, len(errorsKindOrder))
	for _, kind := range errorsKindOrder {
		context := errorsKindContext[kind]
		kinds = append(kinds, ErrorsKindRow{
			Kind:           string(kind),
			HTTPStatus:     core.HTTPStatusFor(kind),
			DefaultMessage: errorsSentinelFor(kind).Message,
			RaisedBy:       context.raisedBy,
			LoggedAs:       context.loggedAs,
			Trigger:        context.trigger,
		})
	}
	return ErrorsExplorerPayload{
		Kinds: kinds,
		EnvelopeNote: "every failure this server produces answers the SAME envelope — " +
			`{"error": {code, message, requestId, details?}} — and stamps the x-request-id ` +
			"header on the response, so a client writes its error handling exactly once and " +
			"a human can grep the request id in the server log to find the exact lines that failed",
		Scenario: ErrorsScenarioRow{
			GetEndpoint:   ErrorsScenarioPath,
			PatchEndpoint: ErrorsScenarioPath,
			Modes: []string{
				string(core.FailureNone),
				string(core.FailureError),
				string(core.FailureLatency),
			},
			Note: "the runtime toggle every demo reads — PATCH it, probe it, PATCH it back; " +
				"invalid values answer the 422 envelope (which is itself the validation " +
				"trigger above)",
		},
		Probe: ErrorsProbeRow{
			ProbeEndpoint: ErrorsProbePath,
			LatencyMillis: ErrorsProbeLatency.Milliseconds(),
			HealthyNote: "with injectFailure=none the probe answers 200 {\"status\": " +
				"\"healthy\"} — the round trip is inject, watch the 500, reset, watch the 200",
			InjectedNote: "with injectFailure=error the probe panics deliberately: the Recover " +
				"layer logs the stack with the request id and answers the generic 500 envelope",
			LatencyNote: fmt.Sprintf(
				"with injectFailure=latency the probe sleeps out its %d ms injection before "+
					"answering 200 — latency is a failure mode too, just one the stopwatch "+
					"catches instead of the taxonomy",
				ErrorsProbeLatency.Milliseconds()),
		},
		Walk: ErrorsWalkRow{
			InjectFailure: string(walk.InjectFailure),
			Status:        walk.Status,
			ElapsedMillis: walk.Elapsed.Milliseconds(),
			RequestID:     walk.RequestID,
			Body:          walk.Body,
			Note:          DescribeErrorsWalk(walk),
		},
		Reasoning: "Eight kinds, one envelope, zero exceptions to either: each row fires " +
			"its kind against the real machinery this guide built in earlier topics, and " +
			"the three recipe rows say plainly why their triggers need conditions the " +
			"running stack does not have — the panel never fakes an error it did not " +
			"really produce",
	}
}

// DescribeErrorsWalk derives the walk's note from what the probe
// actually answered — mode and observed status together, so a walk that
// somehow missed its scripted outcome says so instead of claiming it.
func DescribeErrorsWalk(walk ErrorsWalk) string {
	mode := string(walk.InjectFailure)
	switch {
	case walk.InjectFailure == core.FailureNone && walk.Status == 200:
		return fmt.Sprintf(
			"nothing injected: the probe answered %d healthy in %d ms — PATCH the scenario "+
				"to error and probe again to watch the recovered 500 arrive",
			walk.Status, walk.Elapsed.Milliseconds())
	case walk.InjectFailure == core.FailureError && walk.Status == 500:
		return fmt.Sprintf(
			"error injection honored: the probe panicked, the Recover layer answered the "+
				"generic %d envelope, and the same request id (%s) rode the x-request-id "+
				"response header and the envelope's requestId — one id threads the client, "+
				"the envelope, and the panic-recovered log line",
			walk.Status, walk.RequestID)
	case walk.InjectFailure == core.FailureLatency && walk.Status == 200:
		return fmt.Sprintf(
			"latency injection honored: the probe slept out its %d ms injection before "+
				"answering %d — this walk measured %d ms end to end; no status code moves, "+
				"yet the caller's budget is spent all the same",
			ErrorsProbeLatency.Milliseconds(), walk.Status, walk.Elapsed.Milliseconds())
	default:
		return fmt.Sprintf(
			"observed mode=%s status=%d elapsed=%d ms — the walk reports what actually "+
				"happened, never what the mode promised",
			mode, walk.Status, walk.Elapsed.Milliseconds())
	}
}

// BuildErrorsProbeAnswer shapes the probe's healthy answer for the
// scenario mode it served (pure — the handler supplies the snapshot it
// already read; the latency path calls this after sleeping, the error
// path never reaches it).
func BuildErrorsProbeAnswer(state core.ScenarioState) ErrorsProbeAnswer {
	return ErrorsProbeAnswer{
		Status:        "healthy",
		InjectFailure: string(state.InjectFailure),
		Detail: "probe answered normally — PATCH /api/demo/scenario to " +
			`{"injectFailure": "error"} and probe again to watch the 500 envelope ` +
			"arrive without a stack trace in sight",
	}
}
