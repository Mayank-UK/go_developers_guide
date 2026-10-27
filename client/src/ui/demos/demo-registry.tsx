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
};
