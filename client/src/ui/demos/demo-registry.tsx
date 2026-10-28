import { lazy, type ComponentType, type LazyExoticComponent } from 'react';

/**
 * Demo registry — the client-side half of the content API's demo-block
 * contract. Served content names a demo by the demo block's id; this record is
 * the only place that binds an id to an implementation. Demos are lazy-loaded
 * so a topic's chunk only pays for the demos it actually renders, and an
 * unknown id degrades to a "not wired yet" panel instead of crashing the
 * screen (see DemoHost).
 *
 * The registry grows one lazy entry per landed demo; every demo block id
 * served before its client half lands renders the friendly not-wired-yet
 * card (see DemoHost).
 *
 * ── HOW A DEMO LANDS ────────────────────────────────────────────────────────
 * 1. Server side: a pure payload builder in `internal/domain/demos/` + a thin
 *    handler in `internal/presentation/demo/<area>.go` (area module), so the
 *    demo endpoint is `GET /api/demo/<area>/<name>` with a typed response.
 * 2. Client data path: `src/domain/demo/<name>-repository.ts` (port) +
 *    `src/data/demo/<name>-{dto,api,repository-impl}.ts` + a use case in
 *    `src/domain/demo/`, wired in the container under its own tokens.
 * 3. Create `src/ui/demos/<area>/<name>_demo.tsx` — a DEFAULT-exported
 *    component taking no props (it resolves its own dependencies through the
 *    container, like every view) — with loading + error states and a
 *    fixture-driven spec (happy + failure).
 * 4. Add one entry here:
 *
 *      const MyDemo = lazy(() => import('./<area>/<name>_demo'));
 *      export const demoRegistry = { ..., '<area>:<name>': MyDemo };
 *
 * Nothing else changes: the content's demo-block id starts resolving through
 * DemoHost automatically. Demo ids follow the server's `<area>:<name>`
 * convention (e.g. `go:app-info`, `runtime:goroutines`).
 */

export type DemoComponentType = ComponentType;

export type DemoRegistry = Readonly<
  Record<string, LazyExoticComponent<DemoComponentType>>
>;

const RuntimeTypesDemo = lazy(() => import('./go/runtime_types_demo'));
const ConcurrencyDemo = lazy(() => import('./runtime/concurrency_demo'));
const HeadersDemo = lazy(() => import('./http/headers_demo'));
const AppInfoDemo = lazy(() => import('./gohttp/app_info_demo'));
const InterfaceMatrixDemo = lazy(() => import('./iface/matrix_demo'));
const ContextCancellationDemo = lazy(() => import('./conc/context_demo'));
const RaceDetectorDemo = lazy(() => import('./conc/race_demo'));
const RoutingExplorerDemo = lazy(() => import('./routing/explorer_demo'));
const MiddlewarePipelineDemo = lazy(() => import('./middleware/pipeline_demo'));
const ReqresInspectorDemo = lazy(() => import('./reqres/inspector_demo'));
const ReqresMethodsDemo = lazy(() => import('./reqres/methods_demo'));
const ValidationArenaDemo = lazy(() => import('./validation/arena_demo'));
const PostsWorkbenchDemo = lazy(() => import('./api/posts_workbench_demo'));
const WebhookSimulatorDemo = lazy(() => import('./api/webhook_simulator_demo'));
const DocsLinkDemo = lazy(() => import('./api/docs_link_demo'));
const GraphQLPlaygroundDemo = lazy(() => import('./api/graphql_playground_demo'));
const GRPCConsoleDemo = lazy(() => import('./api/grpc_console_demo'));
const SchemaExplorerDemo = lazy(() => import('./db/schema_explorer_demo'));
const UUIDLabDemo = lazy(() => import('./db/uuid_demo'));
const SQLSafetyDemo = lazy(() => import('./db/sql_safety_demo'));
const MigrationsDemo = lazy(() => import('./db/migrations_demo'));
const SQLLabDemo = lazy(() => import('./db/sql_lab_demo'));
const SearchDemo = lazy(() => import('./db/search_demo'));
const RaceDemo = lazy(() => import('./db/race_demo'));
const AuditDemo = lazy(() => import('./db/audit_demo'));
const ORMProbeDemo = lazy(() => import('./db/orm_probe_demo'));
const MongoConsoleDemo = lazy(() => import('./db/mongo_console_demo'));
const RedisPlaygroundDemo = lazy(() => import('./redis/playground_demo'));
const CachingLabDemo = lazy(() => import('./caching/lab_demo'));
const MessagingStreamDemo = lazy(() => import('./messaging/stream_demo'));
const MessagingOutboxDemo = lazy(() => import('./messaging/outbox_demo'));
// The authentication topic's seven stories share one wire shape, so the
// client ships one shared view (auth/auth_story_view) with seven thin
// named wrappers — one lazy entry each, same as every demo.
const AuthSessionsDemo = lazy(() => import('./auth/sessions_demo'));
const AuthAPIKeysDemo = lazy(() => import('./auth/api_keys_demo'));
const AuthCompareDemo = lazy(() => import('./auth/compare_demo'));
const AuthJWTDemo = lazy(() => import('./auth/jwt_demo'));
const AuthOAuthDemo = lazy(() => import('./auth/oauth_demo'));
const AuthHashingDemo = lazy(() => import('./auth/hashing_demo'));
const AuthRefreshDemo = lazy(() => import('./auth/refresh_demo'));
const AuthzMatrixDemo = lazy(() => import('./authz/matrix_demo'));
// The security topic's two stories share the auth stories' wire shape,
// so they ride the same shared-view pattern.
const SecurityLabDemo = lazy(() => import('./security/lab_demo'));
const SecurityPIIDemo = lazy(() => import('./security/pii_demo'));
const FilesWorkbenchDemo = lazy(() => import('./files/workbench_demo'));
const I18nNegotiatorDemo = lazy(() => import('./i18n/negotiator_demo'));
// The realtime demos hold LIVE connections (EventSource + WebSocket) —
// the lazy chunk pays for itself only when the topic is opened.
const RealtimeTickerDemo = lazy(() => import('./realtime/ticker_demo'));
const RealtimeChatDemo = lazy(() => import('./realtime/chat_demo'));
const JobsConsoleDemo = lazy(() => import('./jobs/console_demo'));
// The error-handling explorer walks the scenario-honoring probe live —
// the topic's failure-injection demo, one lazy entry like every other.
const ErrorsExplorerDemo = lazy(() => import('./errors/explorer_demo'));
// The dependency manifest puts go.mod beside the serving binary's own
// build info and runs the real govulncheck when it can — one lazy
// entry like every other.
const DepsManifestDemo = lazy(() => import('./deps/manifest_demo'));
// The architecture tracer walks one real posts request through the
// layers with recorders at the seams — the timeline the topic draws,
// one lazy entry like every other.
const ArchTracerDemo = lazy(() => import('./arch/tracer_demo'));
// The observability topic's pair: the slog viewer (one real log line
// dissected, the level gate computed, the request-id correlation
// replayed) and the trace round-trip (your traceparent in, a span
// tree out) — one lazy entry each, same as every other.
const ObsViewerDemo = lazy(() => import('./obs/viewer_demo'));
const ObsTraceDemo = lazy(() => import('./obs/trace_demo'));
// The testing topic's runner: a real, bounded `go test -json` run over
// a curated set of the repo's packages, executed once at boot and
// served as a parsed timeline — one lazy entry like every other.
const TestingRunnerDemo = lazy(() => import('./testing/runner_demo'));
// The debugging topic's stall: a real wedge per request — one worker
// of a bounded set stuck on a channel receive — captured from the
// process's own goroutine profile and released before the response
// returns; one lazy entry like every other.
const DebugStallDemo = lazy(() => import('./debug/stall_demo'));
// The environments topic's inspector: this process's resolved config,
// derived at request time — every variable with its
// environment-vs-default attribution, credentials redacted by a
// table-tested policy, the twelve-factor scorecard, and the
// ScenarioStore's current state as the live flag panel.
const EnvInspectorDemo = lazy(() => import('./env/inspector_demo'));
// The deployment topic's stack viewer: the repo's real deploy/
// artifacts — the prod compose file and every k8s manifest — parsed
// live by the server and served with per-artifact provenance: services
// and their rails, the probe contract against the real endpoints, the
// rollout knobs, and the placeholder-only secret scan.
const DeployStackDemo = lazy(() => import('./deploy/stack_demo'));
// The performance topic's bench: this repo's own curated benchmarks
// (gzip levels × sizes, JSON marshal versus streaming encode, the
// in-memory repository's List) executed once at boot under a hard
// process-group-killing timeout, served with the exact command,
// benchtime, count, and age — both count samples side by side, never
// averaged, beside the compiler's escape verdicts verbatim.
const PerfBenchDemo = lazy(() => import('./perf/bench_demo'));
// The reliability topic's breaker: a REAL process-lifetime circuit
// breaker (closed/open/half-open, failure threshold, cooldown,
// single-probe semantics) protecting a dependency that honors the
// shared scenario panel's failure injection and respects context
// deadlines — its walk and retry timelines served with measured
// timings, its state persisting across requests so PATCHing the
// scenario trips and recovers it live.
const ReliabilityBreakerDemo = lazy(() => import('./reliability/breaker_demo'));
// The scaling topic's statelessness round: three in-process replicas
// behind a real round-robin balancer with an affinity table, plus the
// shared store standing for externalized state — one session walked
// through both placements with every number measured (distribution
// rows, swap before-counts, the accumulated counter). The round is
// deterministic, so the replay button re-runs an identical
// choreography: the lesson replays, the numbers do not drift.
const ScalingStatelessDemo = lazy(() => import('./scaling/stateless_demo'));
// The best-practices topic's self-audit: a REAL boot-computed checklist —
// live probes through the real handler chain, real tool runs (gofmt, the
// snippet ratchet, a -race round), real file reads — served from the
// snapshot with honest fails where a check cannot run and manual
// verdicts where none can honestly judge; the repo's deliberate not-dos
// ride the same payload.
const BestChecklistDemo = lazy(() => import('./best/checklist_demo'));
// The third-party topic's adapter-surface hub: three vendor ports
// (mailer, payments, object storage) each driven through a fixed
// script against a deterministic sandbox adapter — signing,
// idempotency keys, quota accounting, error-taxonomy mapping —
// beside its credential-gated live skeleton's honest refusal, the
// DI switch's current position with per-vendor wiring rows, and the
// vendor-error taxonomy map with HTTP statuses derived through the
// server's real core.HTTPStatusFor; no vendor network call is ever
// made, and the sandbox runs are deterministic, so replay re-serves
// identical vendor panels.
const ThirdPartyHubDemo = lazy(() => import('./thirdparty/hub_demo'));
// The advanced-architecture topic's pair, riding the REAL distributed
// patterns from the server's internal/domain/patterns: the saga — three
// observed runs of the compensation machinery (success, injected
// failure with its idempotent re-compensation replay, mid-saga
// cancellation whose undo pass survives the dead context) — and the
// event store — one inventory stream driven through rebuild-by-replay →
// decide → append-with-expected-version, a typed optimistic-concurrency
// conflict with its recovery, and deterministic rebuild proofs. Both
// endpoints are stateless and clock-free, so replay re-serves
// byte-identical ledgers; one lazy entry each, same as every demo.
const ArchSagaDemo = lazy(() => import('./arch/saga_demo'));
const ArchEventstoreDemo = lazy(() => import('./arch/eventstore_demo'));

export const demoRegistry: DemoRegistry = {
  'go:runtime-types': RuntimeTypesDemo,
  'runtime:concurrency': ConcurrencyDemo,
  'http:headers': HeadersDemo,
  'gohttp:app-info': AppInfoDemo,
  'iface:matrix': InterfaceMatrixDemo,
  'conc:context': ContextCancellationDemo,
  'conc:race': RaceDetectorDemo,
  'routing:explorer': RoutingExplorerDemo,
  'middleware:pipeline': MiddlewarePipelineDemo,
  'reqres:inspector': ReqresInspectorDemo,
  'reqres:methods': ReqresMethodsDemo,
  'validation:arena': ValidationArenaDemo,
  'api:posts-workbench': PostsWorkbenchDemo,
  'api:webhook-simulator': WebhookSimulatorDemo,
  'api:docs-link': DocsLinkDemo,
  'api:graphql-playground': GraphQLPlaygroundDemo,
  'api:grpc-console': GRPCConsoleDemo,
  'db:schema-explorer': SchemaExplorerDemo,
  'db:uuid': UUIDLabDemo,
  'db:sql-safety': SQLSafetyDemo,
  'db:migrations': MigrationsDemo,
  'db:sql-lab': SQLLabDemo,
  'db:search': SearchDemo,
  'db:race': RaceDemo,
  'db:audit': AuditDemo,
  'db:orm-probe': ORMProbeDemo,
  'db:mongo-console': MongoConsoleDemo,
  'redis:playground': RedisPlaygroundDemo,
  'caching:lab': CachingLabDemo,
  'msg:stream': MessagingStreamDemo,
  'msg:outbox': MessagingOutboxDemo,
  'auth:sessions': AuthSessionsDemo,
  'auth:api-keys': AuthAPIKeysDemo,
  'auth:compare': AuthCompareDemo,
  'auth:jwt': AuthJWTDemo,
  'auth:oauth': AuthOAuthDemo,
  'auth:hashing': AuthHashingDemo,
  'auth:refresh': AuthRefreshDemo,
  'authz:matrix': AuthzMatrixDemo,
  'security:lab': SecurityLabDemo,
  'security:pii': SecurityPIIDemo,
  'files:workbench': FilesWorkbenchDemo,
  'i18n:negotiator': I18nNegotiatorDemo,
  'realtime:ticker': RealtimeTickerDemo,
  'realtime:chat': RealtimeChatDemo,
  'jobs:console': JobsConsoleDemo,
  'errors:explorer': ErrorsExplorerDemo,
  'deps:manifest': DepsManifestDemo,
  'arch:tracer': ArchTracerDemo,
  'obs:viewer': ObsViewerDemo,
  'obs:trace': ObsTraceDemo,
  'testing:runner': TestingRunnerDemo,
  'debug:stall': DebugStallDemo,
  'env:inspector': EnvInspectorDemo,
  'deploy:stack': DeployStackDemo,
  'perf:bench': PerfBenchDemo,
  'reliability:breaker': ReliabilityBreakerDemo,
  'scaling:stateless': ScalingStatelessDemo,
  'best:checklist': BestChecklistDemo,
  'thirdparty:hub': ThirdPartyHubDemo,
  'arch:saga': ArchSagaDemo,
  'arch:eventstore': ArchEventstoreDemo,
};
