import { type AppConfig } from './config/app-config';
import { ContentApi } from '../data/content/content-api';
import { ContentRepositoryImpl } from '../data/content/content-repository-impl';
import type { ContentRepository } from '../domain/content/content-repository';
import { ListContentUseCase } from '../domain/content/list-content-usecase';
import { GetTopicDetailUseCase } from '../domain/content/get-topic-detail-usecase';
import { HealthApi } from '../data/health/health-api';
import { HealthRepositoryImpl } from '../data/health/health-repository-impl';
import type { HealthRepository } from '../domain/health/health-repository';
import { GetHealthUseCase } from '../domain/health/get-health-usecase';
import { ScenarioApi } from '../data/scenario/scenario-api';
import { ScenarioRepositoryImpl } from '../data/scenario/scenario-repository-impl';
import type { ScenarioRepository } from '../domain/scenario/scenario-repository';
import { GetScenarioUseCase } from '../domain/scenario/get-scenario-usecase';
import { PatchScenarioUseCase } from '../domain/scenario/patch-scenario-usecase';
import { RuntimeTypesApi } from '../data/demo/runtime-types-api';
import { RuntimeTypesRepositoryImpl } from '../data/demo/runtime-types-repository-impl';
import type { RuntimeTypesRepository } from '../domain/demo/runtime-types-repository';
import { GetRuntimeTypesDemoUseCase } from '../domain/demo/get-runtime-types-demo-usecase';
import { ConcurrencyApi } from '../data/demo/concurrency-api';
import { ConcurrencyRepositoryImpl } from '../data/demo/concurrency-repository-impl';
import type { ConcurrencyRepository } from '../domain/demo/concurrency-repository';
import { GetConcurrencyDemoUseCase } from '../domain/demo/get-concurrency-demo-usecase';
import { HttpHeadersApi } from '../data/demo/http-headers-api';
import { HttpHeadersRepositoryImpl } from '../data/demo/http-headers-repository-impl';
import type { HttpHeadersRepository } from '../domain/demo/http-headers-repository';
import {
  GetHttpHeadersDemoUseCase,
  HTTP_HEADERS_DEMO_USE_CASE,
} from '../domain/demo/get-http-headers-demo-usecase';
import { AppInfoApi } from '../data/demo/app-info-api';
import { AppInfoRepositoryImpl } from '../data/demo/app-info-repository-impl';
import type { AppInfoRepository } from '../domain/demo/app-info-repository';
import {
  GetAppInfoDemoUseCase,
  GetAppInfoDemoUseCaseToken,
} from '../domain/demo/get-app-info-demo-usecase';
import { InterfaceMatrixApi } from '../data/demo/interface-matrix-api';
import { InterfaceMatrixRepositoryImpl } from '../data/demo/interface-matrix-repository-impl';
import type { InterfaceMatrixRepository } from '../domain/demo/interface-matrix-repository';
import {
  GetInterfaceMatrixDemoUseCase,
  INTERFACE_MATRIX_DEMO_USE_CASE,
} from '../domain/demo/get-interface-matrix-demo-usecase';
import { ContextCancellationApi } from '../data/demo/context-cancellation-api';
import { ContextCancellationRepositoryImpl } from '../data/demo/context-cancellation-repository-impl';
import type { ContextCancellationRepository } from '../domain/demo/context-cancellation-repository';
import { GetContextCancellationDemoUseCase } from '../domain/demo/get-context-cancellation-demo-usecase';
import { RaceDetectorApi } from '../data/demo/race-detector-api';
import { RaceDetectorRepositoryImpl } from '../data/demo/race-detector-repository-impl';
import type { RaceDetectorRepository } from '../domain/demo/race-detector-repository';
import { GetRaceDetectorDemoUseCase } from '../domain/demo/get-race-detector-demo-usecase';
import { RoutingExplorerApi } from '../data/demo/routing-explorer-api';
import { RoutingExplorerRepositoryImpl } from '../data/demo/routing-explorer-repository-impl';
import type { RoutingExplorerRepository } from '../domain/demo/routing-explorer-repository';
import {
  GetRoutingExplorerDemoUseCase,
  ROUTING_EXPLORER_DEMO_USE_CASE,
} from '../domain/demo/get-routing-explorer-demo-usecase';
import { MiddlewarePipelineApi } from '../data/demo/middleware-pipeline-api';
import { MiddlewarePipelineRepositoryImpl } from '../data/demo/middleware-pipeline-repository-impl';
import type { MiddlewarePipelineRepository } from '../domain/demo/middleware-pipeline-repository';
import {
  GetMiddlewarePipelineDemoUseCase,
  MIDDLEWARE_PIPELINE_DEMO_USE_CASE,
} from '../domain/demo/get-middleware-pipeline-demo-usecase';
import { ReqresInspectorApi } from '../data/demo/reqres-inspector-api';
import { ReqresInspectorRepositoryImpl } from '../data/demo/reqres-inspector-repository-impl';
import type { ReqresInspectorRepository } from '../domain/demo/reqres-inspector-repository';
import {
  GetReqresInspectorDemoUseCase,
  REQRES_INSPECTOR_DEMO_USE_CASE,
} from '../domain/demo/get-reqres-inspector-demo-usecase';
import { ReqresMethodsApi } from '../data/demo/reqres-methods-api';
import { ReqresMethodsRepositoryImpl } from '../data/demo/reqres-methods-repository-impl';
import type { ReqresMethodsRepository } from '../domain/demo/reqres-methods-repository';
import {
  GetReqresMethodsDemoUseCase,
  REQRES_METHODS_DEMO_USE_CASE,
} from '../domain/demo/get-reqres-methods-demo-usecase';
import { ValidationArenaApi } from '../data/demo/validation-arena-api';
import { ValidationArenaRepositoryImpl } from '../data/demo/validation-arena-repository-impl';
import type { ValidationArenaRepository } from '../domain/demo/validation-arena-repository';
import {
  GetValidationArenaDemoUseCase,
  VALIDATION_ARENA_DEMO_USE_CASE,
} from '../domain/demo/get-validation-arena-demo-usecase';
import { PostsApi } from '../data/posts/posts-api';
import { PostsRepositoryImpl } from '../data/posts/posts-repository-impl';
import type { PostsRepository } from '../domain/posts/post-repository';
import { ListPostsUseCase } from '../domain/posts/list-posts-usecase';
import { CreatePostUseCase } from '../domain/posts/create-post-usecase';
import { ReplacePostUseCase } from '../domain/posts/replace-post-usecase';
import { DeletePostUseCase } from '../domain/posts/delete-post-usecase';
import { PostsWorkbenchApi } from '../data/demo/posts-workbench-api';
import { PostsWorkbenchRepositoryImpl } from '../data/demo/posts-workbench-repository-impl';
import type { PostsWorkbenchRepository } from '../domain/demo/posts-workbench-repository';
import {
  GetPostsWorkbenchDemoUseCase,
  POSTS_WORKBENCH_DEMO_USE_CASE,
} from '../domain/demo/get-posts-workbench-demo-usecase';
import { WebhookSimulatorApi } from '../data/demo/webhook-simulator-api';
import { WebhookSimulatorRepositoryImpl } from '../data/demo/webhook-simulator-repository-impl';
import type { WebhookSimulatorRepository } from '../domain/demo/webhook-simulator-repository';
import {
  GetWebhookSimulatorDemoUseCase,
  WEBHOOK_SIMULATOR_DEMO_USE_CASE,
} from '../domain/demo/get-webhook-simulator-demo-usecase';
import { DocsLinkApi } from '../data/demo/docs-link-api';
import { DocsLinkRepositoryImpl } from '../data/demo/docs-link-repository-impl';
import type { DocsLinkRepository } from '../domain/demo/docs-link-repository';
import {
  GetDocsLinkDemoUseCase,
  API_DOCS_LINK_DEMO_USE_CASE,
} from '../domain/demo/get-docs-link-demo-usecase';
import { GraphQLPlaygroundApi } from '../data/demo/graphql-playground-api';
import { GraphQLPlaygroundRepositoryImpl } from '../data/demo/graphql-playground-repository-impl';
import type {
  GraphQLPlaygroundRepository,
  GraphQLQueryRepository,
} from '../domain/demo/graphql-playground-repository';
import {
  GetGraphQLPlaygroundDemoUseCase,
  API_GRAPHQL_PLAYGROUND_DEMO_USE_CASE,
} from '../domain/demo/get-graphql-playground-demo-usecase';
import {
  ExecuteGraphQLDemoQueryUseCase,
  API_GRAPHQL_DEMO_QUERY_USE_CASE,
} from '../domain/demo/execute-graphql-demo-query-usecase';
import { GRPCConsoleApi } from '../data/demo/grpc-console-api';
import { GRPCConsoleRepositoryImpl } from '../data/demo/grpc-console-repository-impl';
import type { GRPCConsoleRepository } from '../domain/demo/grpc-console-repository';
import {
  GetGRPCConsoleDemoUseCase,
  API_GRPC_CONSOLE_DEMO_USE_CASE,
} from '../domain/demo/get-grpc-console-demo-usecase';
import { SchemaExplorerApi } from '../data/demo/schema-explorer-api';
import { SchemaExplorerRepositoryImpl } from '../data/demo/schema-explorer-repository-impl';
import type { SchemaExplorerRepository } from '../domain/demo/schema-explorer-repository';
import {
  GetSchemaExplorerDemoUseCase,
  SCHEMA_EXPLORER_DEMO_USE_CASE,
} from '../domain/demo/get-schema-explorer-demo-usecase';
import { UUIDLabApi } from '../data/demo/uuid-lab-api';
import { UUIDLabRepositoryImpl } from '../data/demo/uuid-lab-repository-impl';
import type { UUIDLabRepository } from '../domain/demo/uuid-lab-repository';
import {
  GetUUIDLabDemoUseCase,
  DB_UUID_DEMO_USE_CASE,
} from '../domain/demo/get-uuid-lab-demo-usecase';
import { SQLSafetyApi } from '../data/demo/sql-safety-api';
import { SQLSafetyRepositoryImpl } from '../data/demo/sql-safety-repository-impl';
import type { SQLSafetyRepository } from '../domain/demo/sql-safety-repository';
import {
  GetSQLSafetyDemoUseCase,
  DB_SQL_SAFETY_DEMO_USE_CASE,
} from '../domain/demo/get-sql-safety-demo-usecase';
import { MigrationsApi } from '../data/demo/db-migrations-api';
import { MigrationsRepositoryImpl } from '../data/demo/db-migrations-repository-impl';
import type { MigrationsRepository } from '../domain/demo/db-migrations-repository';
import {
  GetMigrationsDemoUseCase,
  DB_MIGRATIONS_DEMO_USE_CASE,
} from '../domain/demo/get-db-migrations-demo-usecase';
import { SQLLabApi } from '../data/demo/sql-lab-api';
import { SQLLabRepositoryImpl } from '../data/demo/sql-lab-repository-impl';
import type { SQLLabRepository } from '../domain/demo/sql-lab-repository';
import {
  GetSQLLabDemoUseCase,
  DB_SQL_LAB_DEMO_USE_CASE,
} from '../domain/demo/get-sql-lab-demo-usecase';
import { SearchApi } from '../data/demo/db-search-api';
import { SearchRepositoryImpl } from '../data/demo/db-search-repository-impl';
import type { SearchRepository } from '../domain/demo/db-search-repository';
import {
  GetSearchDemoUseCase,
  DB_SEARCH_DEMO_USE_CASE,
} from '../domain/demo/get-db-search-demo-usecase';
import { RaceApi } from '../data/demo/db-race-api';
import { RaceRepositoryImpl } from '../data/demo/db-race-repository-impl';
import type { RaceRepository } from '../domain/demo/db-race-repository';
import {
  GetRaceDemoUseCase,
  DB_RACE_DEMO_USE_CASE,
} from '../domain/demo/get-db-race-demo-usecase';
import { AuditApi } from '../data/demo/db-audit-api';
import { AuditRepositoryImpl } from '../data/demo/db-audit-repository-impl';
import type { AuditRepository } from '../domain/demo/db-audit-repository';
import {
  GetAuditDemoUseCase,
  DB_AUDIT_DEMO_USE_CASE,
} from '../domain/demo/get-db-audit-demo-usecase';
import { ORMProbeApi } from '../data/demo/orm-probe-api';
import { ORMProbeRepositoryImpl } from '../data/demo/orm-probe-repository-impl';
import type { ORMProbeRepository } from '../domain/demo/orm-probe-repository';
import {
  GetORMProbeDemoUseCase,
  DB_ORM_PROBE_DEMO_USE_CASE,
} from '../domain/demo/get-orm-probe-demo-usecase';
import { MongoConsoleApi } from '../data/demo/mongo-console-api';
import { MongoConsoleRepositoryImpl } from '../data/demo/mongo-console-repository-impl';
import type { MongoConsoleRepository } from '../domain/demo/mongo-console-repository';
import {
  GetMongoConsoleDemoUseCase,
  MONGO_CONSOLE_DEMO_USE_CASE,
} from '../domain/demo/get-mongo-console-demo-usecase';
import { RedisPlaygroundApi } from '../data/demo/redis-playground-api';
import { RedisPlaygroundRepositoryImpl } from '../data/demo/redis-playground-repository-impl';
import type { RedisPlaygroundRepository } from '../domain/demo/redis-playground-repository';
import {
  GetRedisPlaygroundDemoUseCase,
  REDIS_PLAYGROUND_DEMO_USE_CASE,
} from '../domain/demo/get-redis-playground-demo-usecase';
import { CachingLabApi } from '../data/demo/caching-lab-api';
import { CachingLabRepositoryImpl } from '../data/demo/caching-lab-repository-impl';
import type { CachingLabRepository } from '../domain/demo/caching-lab-repository';
import {
  GetCachingLabDemoUseCase,
  CACHING_LAB_DEMO_USE_CASE,
} from '../domain/demo/get-caching-lab-demo-usecase';
import { MessagingStreamApi } from '../data/demo/messaging-stream-api';
import { MessagingStreamRepositoryImpl } from '../data/demo/messaging-stream-repository-impl';
import type { MessagingStreamRepository } from '../domain/demo/messaging-stream-repository';
import {
  GetMessagingStreamDemoUseCase,
  MESSAGING_STREAM_DEMO_USE_CASE,
} from '../domain/demo/get-messaging-stream-demo-usecase';
import { MessagingOutboxApi } from '../data/demo/messaging-outbox-api';
import { MessagingOutboxRepositoryImpl } from '../data/demo/messaging-outbox-repository-impl';
import type { MessagingOutboxRepository } from '../domain/demo/messaging-outbox-repository';
import {
  GetMessagingOutboxDemoUseCase,
  MESSAGING_OUTBOX_DEMO_USE_CASE,
} from '../domain/demo/get-messaging-outbox-demo-usecase';
// The authentication topic's seven story demos share one payload shape,
// so the client pipeline is shared too: one model/port/use-case trio and
// one api/repository pair, instantiated per demo endpoint path and
// registered under seven tokens.
import { AuthStoryApi } from '../data/demo/auth-story-api';
import { AuthStoryRepositoryImpl } from '../data/demo/auth-story-repository-impl';
import { AUTH_STORY_PATHS } from '../data/demo/auth-story-dto';
import {
  GetAuthStoryDemoUseCase,
  AUTH_SESSIONS_DEMO_USE_CASE,
  AUTH_API_KEYS_DEMO_USE_CASE,
  AUTH_COMPARE_DEMO_USE_CASE,
  AUTH_JWT_DEMO_USE_CASE,
  AUTH_OAUTH_DEMO_USE_CASE,
  AUTH_HASHING_DEMO_USE_CASE,
  AUTH_REFRESH_DEMO_USE_CASE,
} from '../domain/demo/get-auth-story-demo-usecase';
import { AuthzMatrixApi } from '../data/demo/authz-matrix-api';
import { AuthzMatrixRepositoryImpl } from '../data/demo/authz-matrix-repository-impl';
import {
  GetAuthzMatrixDemoUseCase,
  AUTHZ_MATRIX_DEMO_USE_CASE,
} from '../domain/demo/get-authz-matrix-demo-usecase';
import { SecurityStoryApi } from '../data/demo/security-story-api';
import { SecurityStoryRepositoryImpl } from '../data/demo/security-story-repository-impl';
import { SECURITY_STORY_PATHS } from '../data/demo/security-story-dto';
import {
  GetSecurityStoryDemoUseCase,
  SECURITY_LAB_DEMO_USE_CASE,
  SECURITY_PII_DEMO_USE_CASE,
} from '../domain/demo/get-security-story-demo-usecase';
import { FilesStoryApi } from '../data/demo/files-story-api';
import { FilesStoryRepositoryImpl } from '../data/demo/files-story-repository-impl';
import { FILES_STORY_PATH } from '../data/demo/files-story-dto';
import {
  GetFilesWorkbenchDemoUseCase,
  FILES_WORKBENCH_DEMO_USE_CASE,
} from '../domain/demo/get-files-workbench-demo-usecase';
import { I18nStoryApi } from '../data/demo/i18n-story-api';
import { I18nStoryRepositoryImpl } from '../data/demo/i18n-story-repository-impl';
import { I18N_STORY_PATH } from '../data/demo/i18n-story-dto';
import {
  GetI18nNegotiatorDemoUseCase,
  I18N_NEGOTIATOR_DEMO_USE_CASE,
} from '../domain/demo/get-i18n-negotiator-demo-usecase';
import { RealtimeStoryApi } from '../data/demo/realtime-story-api';
import { RealtimeStoryRepositoryImpl } from '../data/demo/realtime-story-repository-impl';
import { REALTIME_STORY_PATHS } from '../data/demo/realtime-story-dto';
import {
  GetRealtimeStoryDemoUseCase,
  REALTIME_TICKER_DEMO_USE_CASE,
  REALTIME_CHAT_DEMO_USE_CASE,
} from '../domain/demo/get-realtime-story-demo-usecase';
import { JobsConsoleApi } from '../data/demo/jobs-console-api';
import { JobsConsoleRepositoryImpl } from '../data/demo/jobs-console-repository-impl';
import type { JobsConsoleRepository } from '../domain/demo/jobs-console-repository';
import {
  GetJobsConsoleDemoUseCase,
  JOBS_CONSOLE_DEMO_USE_CASE,
} from '../domain/demo/get-jobs-console-demo-usecase';
import { ErrorsExplorerApi } from '../data/demo/errors-explorer-api';
import { ErrorsExplorerRepositoryImpl } from '../data/demo/errors-explorer-repository-impl';
import type { ErrorsExplorerRepository } from '../domain/demo/errors-explorer-repository';
import {
  GetErrorsExplorerDemoUseCase,
  ERRORS_EXPLORER_DEMO_USE_CASE,
} from '../domain/demo/get-errors-explorer-demo-usecase';
import { DepsManifestApi } from '../data/demo/deps-manifest-api';
import { DepsManifestRepositoryImpl } from '../data/demo/deps-manifest-repository-impl';
import type { DepsManifestRepository } from '../domain/demo/deps-manifest-repository';
import {
  GetDepsManifestDemoUseCase,
  DEPS_MANIFEST_DEMO_USE_CASE,
} from '../domain/demo/get-deps-manifest-demo-usecase';
import { ArchTracerApi } from '../data/demo/arch-tracer-api';
import { ArchTracerRepositoryImpl } from '../data/demo/arch-tracer-repository-impl';
import type { ArchTracerRepository } from '../domain/demo/arch-tracer-repository';
import {
  GetArchTracerDemoUseCase,
  ARCH_TRACER_DEMO_USE_CASE,
} from '../domain/demo/get-arch-tracer-demo-usecase';
import { ObsViewerApi } from '../data/demo/obs-viewer-api';
import { ObsViewerRepositoryImpl } from '../data/demo/obs-viewer-repository-impl';
import type { ObsViewerRepository } from '../domain/demo/obs-viewer-repository';
import {
  GetObsViewerDemoUseCase,
  OBS_VIEWER_DEMO_USE_CASE,
} from '../domain/demo/get-obs-viewer-demo-usecase';
import { ObsTraceApi } from '../data/demo/obs-trace-api';
import { ObsTraceRepositoryImpl } from '../data/demo/obs-trace-repository-impl';
import type { ObsTraceRepository } from '../domain/demo/obs-trace-repository';
import {
  GetObsTraceDemoUseCase,
  OBS_TRACE_DEMO_USE_CASE,
} from '../domain/demo/get-obs-trace-demo-usecase';
import { TestingRunnerApi } from '../data/demo/testing-runner-api';
import { TestingRunnerRepositoryImpl } from '../data/demo/testing-runner-repository-impl';
import type { TestingRunnerRepository } from '../domain/demo/testing-runner-repository';
import {
  GetTestingRunnerDemoUseCase,
  TESTING_RUNNER_DEMO_USE_CASE,
} from '../domain/demo/get-testing-runner-demo-usecase';
import { DebugStallApi } from '../data/demo/debug-stall-api';
import { DebugStallRepositoryImpl } from '../data/demo/debug-stall-repository-impl';
import type { DebugStallRepository } from '../domain/demo/debug-stall-repository';
import {
  GetDebugStallDemoUseCase,
  DEBUG_STALL_DEMO_USE_CASE,
} from '../domain/demo/get-debug-stall-demo-usecase';
import { EnvInspectorApi } from '../data/demo/env-inspector-api';
import { EnvInspectorRepositoryImpl } from '../data/demo/env-inspector-repository-impl';
import type { EnvInspectorRepository } from '../domain/demo/env-inspector-repository';
import {
  GetEnvInspectorDemoUseCase,
  ENV_INSPECTOR_DEMO_USE_CASE,
} from '../domain/demo/get-env-inspector-demo-usecase';

/**
 * Service-locator container — this client's composition root, mirroring the
 * guide family's `container.ts` (same locator in every sibling repo). A small
 * hand-rolled locator:
 * `register` singletons keyed by symbol, `resolve` them. It keeps wiring in
 * one explicit, debuggable place and lets views resolve use cases without
 * prop-drilling or a module-bound default per service.
 *
 * Layer rules it enforces by construction: only THIS file imports data/
 * adapters; views resolve through the container and never import data/
 * directly; data/ never imports the container.
 *
 * Phase 0 wired the health pipeline; the content engine added the content
 * pipeline (content API → repository → index/detail use cases); the demo
 * scenario toggle (scenario API → repository → get/patch use cases, behind
 * the root header's ScenarioPanel) rides the same shape. Later phases add
 * the per-topic demo pipelines the same way: one import quartet + token
 * pair + register block + getter per demo.
 *
 * `initialize` is idempotent — re-running it just re-registers. Call it once
 * at startup (main.tsx) or in a test's beforeAll.
 */

/** Symbol tokens — the single registry of injectable service identities. */
export const ContainerTokens = {
  AppConfig: Symbol('AppConfig'),
  HealthApi: Symbol('HealthApi'),
  HealthRepository: Symbol('HealthRepository'),
  GetHealthUseCase: Symbol('GetHealthUseCase'),
  ContentApi: Symbol('ContentApi'),
  ContentRepository: Symbol('ContentRepository'),
  ListContentUseCase: Symbol('ListContentUseCase'),
  GetTopicDetailUseCase: Symbol('GetTopicDetailUseCase'),
  ScenarioApi: Symbol('ScenarioApi'),
  ScenarioRepository: Symbol('ScenarioRepository'),
  GetScenarioUseCase: Symbol('GetScenarioUseCase'),
  PatchScenarioUseCase: Symbol('PatchScenarioUseCase'),
  RuntimeTypesApi: Symbol('RuntimeTypesApi'),
  RuntimeTypesRepository: Symbol('RuntimeTypesRepository'),
  GetRuntimeTypesDemoUseCase: Symbol('GetRuntimeTypesDemoUseCase'),
  ConcurrencyApi: Symbol('ConcurrencyApi'),
  ConcurrencyRepository: Symbol('ConcurrencyRepository'),
  GetConcurrencyDemoUseCase: Symbol('GetConcurrencyDemoUseCase'),
  HttpHeadersApi: Symbol('HttpHeadersApi'),
  HttpHeadersRepository: Symbol('HttpHeadersRepository'),
  // Aliased to the domain module's own symbol, not a fresh one: the headers
  // demo view resolves its use case via
  // container.resolve(HTTP_HEADERS_DEMO_USE_CASE), so the registration key
  // must be that exact symbol — ContainerTokens stays the single registry of
  // identities by aliasing it here.
  GetHttpHeadersDemoUseCase: HTTP_HEADERS_DEMO_USE_CASE,
  AppInfoApi: Symbol('AppInfoApi'),
  AppInfoRepository: Symbol('AppInfoRepository'),
  // Same aliasing story as the headers demo (GetAppInfoDemoUseCaseToken).
  GetAppInfoDemoUseCase: GetAppInfoDemoUseCaseToken,
  InterfaceMatrixApi: Symbol('InterfaceMatrixApi'),
  InterfaceMatrixRepository: Symbol('InterfaceMatrixRepository'),
  // Same aliasing story as the headers demo (INTERFACE_MATRIX_DEMO_USE_CASE).
  GetInterfaceMatrixDemoUseCase: INTERFACE_MATRIX_DEMO_USE_CASE,
  ContextCancellationApi: Symbol('ContextCancellationApi'),
  ContextCancellationRepository: Symbol('ContextCancellationRepository'),
  GetContextCancellationDemoUseCase: Symbol('GetContextCancellationDemoUseCase'),
  RaceDetectorApi: Symbol('RaceDetectorApi'),
  RaceDetectorRepository: Symbol('RaceDetectorRepository'),
  GetRaceDetectorDemoUseCase: Symbol('GetRaceDetectorDemoUseCase'),
  RoutingExplorerApi: Symbol('RoutingExplorerApi'),
  RoutingExplorerRepository: Symbol('RoutingExplorerRepository'),
  // Same aliasing story as the headers demo (ROUTING_EXPLORER_DEMO_USE_CASE).
  GetRoutingExplorerDemoUseCase: ROUTING_EXPLORER_DEMO_USE_CASE,
  MiddlewarePipelineApi: Symbol('MiddlewarePipelineApi'),
  MiddlewarePipelineRepository: Symbol('MiddlewarePipelineRepository'),
  // Same aliasing story as the headers demo (MIDDLEWARE_PIPELINE_DEMO_USE_CASE).
  GetMiddlewarePipelineDemoUseCase: MIDDLEWARE_PIPELINE_DEMO_USE_CASE,
  ReqresInspectorApi: Symbol('ReqresInspectorApi'),
  ReqresInspectorRepository: Symbol('ReqresInspectorRepository'),
  // Same aliasing story as the headers demo (REQRES_INSPECTOR_DEMO_USE_CASE).
  GetReqresInspectorDemoUseCase: REQRES_INSPECTOR_DEMO_USE_CASE,
  ReqresMethodsApi: Symbol('ReqresMethodsApi'),
  ReqresMethodsRepository: Symbol('ReqresMethodsRepository'),
  // Same aliasing story as the headers demo (REQRES_METHODS_DEMO_USE_CASE).
  GetReqresMethodsDemoUseCase: REQRES_METHODS_DEMO_USE_CASE,
  ValidationArenaApi: Symbol('ValidationArenaApi'),
  ValidationArenaRepository: Symbol('ValidationArenaRepository'),
  // Same aliasing story as the headers demo (VALIDATION_ARENA_DEMO_USE_CASE).
  GetValidationArenaDemoUseCase: VALIDATION_ARENA_DEMO_USE_CASE,
  PostsApi: Symbol('PostsApi'),
  PostsRepository: Symbol('PostsRepository'),
  ListPostsUseCase: Symbol('ListPostsUseCase'),
  CreatePostsUseCase: Symbol('CreatePostsUseCase'),
  ReplacePostsUseCase: Symbol('ReplacePostsUseCase'),
  DeletePostsUseCase: Symbol('DeletePostsUseCase'),
  PostsWorkbenchApi: Symbol('PostsWorkbenchApi'),
  PostsWorkbenchRepository: Symbol('PostsWorkbenchRepository'),
  // Same aliasing story as the headers demo (POSTS_WORKBENCH_DEMO_USE_CASE).
  GetPostsWorkbenchDemoUseCase: POSTS_WORKBENCH_DEMO_USE_CASE,
  WebhookSimulatorApi: Symbol('WebhookSimulatorApi'),
  WebhookSimulatorRepository: Symbol('WebhookSimulatorRepository'),
  // Same aliasing story as the headers demo (WEBHOOK_SIMULATOR_DEMO_USE_CASE).
  GetWebhookSimulatorDemoUseCase: WEBHOOK_SIMULATOR_DEMO_USE_CASE,
  DocsLinkApi: Symbol('DocsLinkApi'),
  DocsLinkRepository: Symbol('DocsLinkRepository'),
  // Same aliasing story as the headers demo (API_DOCS_LINK_DEMO_USE_CASE).
  GetDocsLinkDemoUseCase: API_DOCS_LINK_DEMO_USE_CASE,
  GraphQLPlaygroundApi: Symbol('GraphQLPlaygroundApi'),
  GraphQLPlaygroundRepository: Symbol('GraphQLPlaygroundRepository'),
  // Two aliased use cases share the ONE playground repository below — the
  // payload load and the RUN button ride the same pipeline.
  GetGraphQLPlaygroundDemoUseCase: API_GRAPHQL_PLAYGROUND_DEMO_USE_CASE,
  ExecuteGraphQLDemoQueryUseCase: API_GRAPHQL_DEMO_QUERY_USE_CASE,
  GRPCConsoleApi: Symbol('GRPCConsoleApi'),
  GRPCConsoleRepository: Symbol('GRPCConsoleRepository'),
  // Same aliasing story as the headers demo (API_GRPC_CONSOLE_DEMO_USE_CASE).
  GetGRPCConsoleDemoUseCase: API_GRPC_CONSOLE_DEMO_USE_CASE,
  SchemaExplorerApi: Symbol('SchemaExplorerApi'),
  SchemaExplorerRepository: Symbol('SchemaExplorerRepository'),
  // Same aliasing story as the headers demo (SCHEMA_EXPLORER_DEMO_USE_CASE).
  GetSchemaExplorerDemoUseCase: SCHEMA_EXPLORER_DEMO_USE_CASE,
  UUIDLabApi: Symbol('UUIDLabApi'),
  UUIDLabRepository: Symbol('UUIDLabRepository'),
  // Same aliasing story as the headers demo (DB_UUID_DEMO_USE_CASE).
  GetUUIDLabDemoUseCase: DB_UUID_DEMO_USE_CASE,
  SQLSafetyApi: Symbol('SQLSafetyApi'),
  SQLSafetyRepository: Symbol('SQLSafetyRepository'),
  // Same aliasing story as the headers demo (DB_SQL_SAFETY_DEMO_USE_CASE).
  GetSQLSafetyDemoUseCase: DB_SQL_SAFETY_DEMO_USE_CASE,
  MigrationsApi: Symbol('MigrationsApi'),
  MigrationsRepository: Symbol('MigrationsRepository'),
  // Same aliasing story as the headers demo (DB_MIGRATIONS_DEMO_USE_CASE).
  GetMigrationsDemoUseCase: DB_MIGRATIONS_DEMO_USE_CASE,
  SQLLabApi: Symbol('SQLLabApi'),
  SQLLabRepository: Symbol('SQLLabRepository'),
  // Same aliasing story as the headers demo (DB_SQL_LAB_DEMO_USE_CASE).
  GetSQLLabDemoUseCase: DB_SQL_LAB_DEMO_USE_CASE,
  SearchApi: Symbol('SearchApi'),
  SearchRepository: Symbol('SearchRepository'),
  // Same aliasing story as the headers demo (DB_SEARCH_DEMO_USE_CASE).
  GetSearchDemoUseCase: DB_SEARCH_DEMO_USE_CASE,
  RaceApi: Symbol('RaceApi'),
  RaceRepository: Symbol('RaceRepository'),
  // Same aliasing story as the headers demo (DB_RACE_DEMO_USE_CASE).
  GetRaceDemoUseCase: DB_RACE_DEMO_USE_CASE,
  AuditApi: Symbol('AuditApi'),
  AuditRepository: Symbol('AuditRepository'),
  // Same aliasing story as the headers demo (DB_AUDIT_DEMO_USE_CASE).
  GetAuditDemoUseCase: DB_AUDIT_DEMO_USE_CASE,
  ORMProbeApi: Symbol('ORMProbeApi'),
  ORMProbeRepository: Symbol('ORMProbeRepository'),
  // Same aliasing story as the headers demo (DB_ORM_PROBE_DEMO_USE_CASE).
  GetORMProbeDemoUseCase: DB_ORM_PROBE_DEMO_USE_CASE,
  MongoConsoleApi: Symbol('MongoConsoleApi'),
  MongoConsoleRepository: Symbol('MongoConsoleRepository'),
  // Same aliasing story as the headers demo (MONGO_CONSOLE_DEMO_USE_CASE).
  GetMongoConsoleDemoUseCase: MONGO_CONSOLE_DEMO_USE_CASE,
  RedisPlaygroundApi: Symbol('RedisPlaygroundApi'),
  RedisPlaygroundRepository: Symbol('RedisPlaygroundRepository'),
  // Same aliasing story as the headers demo (REDIS_PLAYGROUND_DEMO_USE_CASE).
  GetRedisPlaygroundDemoUseCase: REDIS_PLAYGROUND_DEMO_USE_CASE,
  CachingLabApi: Symbol('CachingLabApi'),
  CachingLabRepository: Symbol('CachingLabRepository'),
  // Same aliasing story as the headers demo (CACHING_LAB_DEMO_USE_CASE).
  GetCachingLabDemoUseCase: CACHING_LAB_DEMO_USE_CASE,
  MessagingStreamApi: Symbol('MessagingStreamApi'),
  MessagingStreamRepository: Symbol('MessagingStreamRepository'),
  // Same aliasing story as the headers demo (MESSAGING_STREAM_DEMO_USE_CASE).
  GetMessagingStreamDemoUseCase: MESSAGING_STREAM_DEMO_USE_CASE,
  MessagingOutboxApi: Symbol('MessagingOutboxApi'),
  MessagingOutboxRepository: Symbol('MessagingOutboxRepository'),
  // Same aliasing story as the headers demo (MESSAGING_OUTBOX_DEMO_USE_CASE).
  GetMessagingOutboxDemoUseCase: MESSAGING_OUTBOX_DEMO_USE_CASE,
  // The auth story demos' tokens live in their own domain module; the
  // aliases here keep ContainerTokens the single registry of identities.
  GetAuthSessionsDemoUseCase: AUTH_SESSIONS_DEMO_USE_CASE,
  GetAuthAPIKeysDemoUseCase: AUTH_API_KEYS_DEMO_USE_CASE,
  GetAuthCompareDemoUseCase: AUTH_COMPARE_DEMO_USE_CASE,
  GetAuthJWTDemoUseCase: AUTH_JWT_DEMO_USE_CASE,
  GetAuthOAuthDemoUseCase: AUTH_OAUTH_DEMO_USE_CASE,
  GetAuthHashingDemoUseCase: AUTH_HASHING_DEMO_USE_CASE,
  GetAuthRefreshDemoUseCase: AUTH_REFRESH_DEMO_USE_CASE,
  // Same aliasing story as the auth stories (AUTHZ_MATRIX_DEMO_USE_CASE).
  GetAuthzMatrixDemoUseCase: AUTHZ_MATRIX_DEMO_USE_CASE,
  // Same aliasing story as the auth stories (the security demos' tokens).
  GetSecurityLabDemoUseCase: SECURITY_LAB_DEMO_USE_CASE,
  GetSecurityPIIDemoUseCase: SECURITY_PII_DEMO_USE_CASE,
  // Same aliasing story as the auth stories (files / i18n / realtime).
  GetFilesWorkbenchDemoUseCase: FILES_WORKBENCH_DEMO_USE_CASE,
  GetI18nNegotiatorDemoUseCase: I18N_NEGOTIATOR_DEMO_USE_CASE,
  GetRealtimeTickerDemoUseCase: REALTIME_TICKER_DEMO_USE_CASE,
  GetRealtimeChatDemoUseCase: REALTIME_CHAT_DEMO_USE_CASE,
  JobsConsoleApi: Symbol('JobsConsoleApi'),
  JobsConsoleRepository: Symbol('JobsConsoleRepository'),
  // Same aliasing story as the auth stories (JOBS_CONSOLE_DEMO_USE_CASE).
  GetJobsConsoleDemoUseCase: JOBS_CONSOLE_DEMO_USE_CASE,
  ErrorsExplorerApi: Symbol('ErrorsExplorerApi'),
  ErrorsExplorerRepository: Symbol('ErrorsExplorerRepository'),
  // Same aliasing story as the auth stories (ERRORS_EXPLORER_DEMO_USE_CASE).
  GetErrorsExplorerDemoUseCase: ERRORS_EXPLORER_DEMO_USE_CASE,
  DepsManifestApi: Symbol('DepsManifestApi'),
  DepsManifestRepository: Symbol('DepsManifestRepository'),
  // Same aliasing story as the auth stories (DEPS_MANIFEST_DEMO_USE_CASE).
  GetDepsManifestDemoUseCase: DEPS_MANIFEST_DEMO_USE_CASE,
  ArchTracerApi: Symbol('ArchTracerApi'),
  ArchTracerRepository: Symbol('ArchTracerRepository'),
  // Same aliasing story as the auth stories (ARCH_TRACER_DEMO_USE_CASE).
  GetArchTracerDemoUseCase: ARCH_TRACER_DEMO_USE_CASE,
  ObsViewerApi: Symbol('ObsViewerApi'),
  ObsViewerRepository: Symbol('ObsViewerRepository'),
  // Same aliasing story as the auth stories (OBS_VIEWER_DEMO_USE_CASE).
  GetObsViewerDemoUseCase: OBS_VIEWER_DEMO_USE_CASE,
  ObsTraceApi: Symbol('ObsTraceApi'),
  ObsTraceRepository: Symbol('ObsTraceRepository'),
  // Same aliasing story as the auth stories (OBS_TRACE_DEMO_USE_CASE).
  GetObsTraceDemoUseCase: OBS_TRACE_DEMO_USE_CASE,
  TestingRunnerApi: Symbol('TestingRunnerApi'),
  TestingRunnerRepository: Symbol('TestingRunnerRepository'),
  // Same aliasing story as the auth stories (TESTING_RUNNER_DEMO_USE_CASE).
  GetTestingRunnerDemoUseCase: TESTING_RUNNER_DEMO_USE_CASE,
  DebugStallApi: Symbol('DebugStallApi'),
  DebugStallRepository: Symbol('DebugStallRepository'),
  // Same aliasing story as the auth stories (DEBUG_STALL_DEMO_USE_CASE).
  GetDebugStallDemoUseCase: DEBUG_STALL_DEMO_USE_CASE,
  EnvInspectorApi: Symbol('EnvInspectorApi'),
  EnvInspectorRepository: Symbol('EnvInspectorRepository'),
  // Same aliasing story as the auth stories (ENV_INSPECTOR_DEMO_USE_CASE).
  GetEnvInspectorDemoUseCase: ENV_INSPECTOR_DEMO_USE_CASE,
} as const;

class Container {
  private readonly registry = new Map<symbol, unknown>();

  initialize(config: AppConfig): void {
    // Eager singletons — the full graph, config → api → repository → use case,
    // mirroring the server's composition-root wiring.
    this.register(ContainerTokens.AppConfig, config);

    this.register(ContainerTokens.HealthApi, new HealthApi(config));
    this.register(
      ContainerTokens.HealthRepository,
      new HealthRepositoryImpl(this.healthApi),
    );
    this.register(
      ContainerTokens.GetHealthUseCase,
      new GetHealthUseCase(this.healthRepository),
    );

    this.register(ContainerTokens.ContentApi, new ContentApi(config));
    this.register(
      ContainerTokens.ContentRepository,
      new ContentRepositoryImpl(this.contentApi),
    );
    this.register(
      ContainerTokens.ListContentUseCase,
      new ListContentUseCase(this.contentRepository),
    );
    this.register(
      ContainerTokens.GetTopicDetailUseCase,
      new GetTopicDetailUseCase(this.contentRepository),
    );

    this.register(ContainerTokens.ScenarioApi, new ScenarioApi(config));
    this.register(
      ContainerTokens.ScenarioRepository,
      new ScenarioRepositoryImpl(this.scenarioApi),
    );
    this.register(
      ContainerTokens.GetScenarioUseCase,
      new GetScenarioUseCase(this.scenarioRepository),
    );
    this.register(
      ContainerTokens.PatchScenarioUseCase,
      new PatchScenarioUseCase(this.scenarioRepository),
    );

    this.register(ContainerTokens.RuntimeTypesApi, new RuntimeTypesApi(config));
    this.register(
      ContainerTokens.RuntimeTypesRepository,
      new RuntimeTypesRepositoryImpl(this.runtimeTypesApi),
    );
    this.register(
      ContainerTokens.GetRuntimeTypesDemoUseCase,
      new GetRuntimeTypesDemoUseCase(this.runtimeTypesRepository),
    );

    this.register(ContainerTokens.ConcurrencyApi, new ConcurrencyApi(config));
    this.register(
      ContainerTokens.ConcurrencyRepository,
      new ConcurrencyRepositoryImpl(this.concurrencyApi),
    );
    this.register(
      ContainerTokens.GetConcurrencyDemoUseCase,
      new GetConcurrencyDemoUseCase(this.concurrencyRepository),
    );

    this.register(ContainerTokens.HttpHeadersApi, new HttpHeadersApi(config));
    this.register(
      ContainerTokens.HttpHeadersRepository,
      new HttpHeadersRepositoryImpl(this.httpHeadersApi),
    );
    this.register(
      ContainerTokens.GetHttpHeadersDemoUseCase,
      new GetHttpHeadersDemoUseCase(this.httpHeadersRepository),
    );

    this.register(ContainerTokens.AppInfoApi, new AppInfoApi(config));
    this.register(
      ContainerTokens.AppInfoRepository,
      new AppInfoRepositoryImpl(this.appInfoApi),
    );
    this.register(
      ContainerTokens.GetAppInfoDemoUseCase,
      new GetAppInfoDemoUseCase(this.appInfoRepository),
    );

    this.register(ContainerTokens.InterfaceMatrixApi, new InterfaceMatrixApi(config));
    this.register(
      ContainerTokens.InterfaceMatrixRepository,
      new InterfaceMatrixRepositoryImpl(this.interfaceMatrixApi),
    );
    this.register(
      ContainerTokens.GetInterfaceMatrixDemoUseCase,
      new GetInterfaceMatrixDemoUseCase(this.interfaceMatrixRepository),
    );

    this.register(ContainerTokens.ContextCancellationApi, new ContextCancellationApi(config));
    this.register(
      ContainerTokens.ContextCancellationRepository,
      new ContextCancellationRepositoryImpl(this.contextCancellationApi),
    );
    this.register(
      ContainerTokens.GetContextCancellationDemoUseCase,
      new GetContextCancellationDemoUseCase(this.contextCancellationRepository),
    );

    this.register(ContainerTokens.RaceDetectorApi, new RaceDetectorApi(config));
    this.register(
      ContainerTokens.RaceDetectorRepository,
      new RaceDetectorRepositoryImpl(this.raceDetectorApi),
    );
    this.register(
      ContainerTokens.GetRaceDetectorDemoUseCase,
      new GetRaceDetectorDemoUseCase(this.raceDetectorRepository),
    );

    this.register(ContainerTokens.RoutingExplorerApi, new RoutingExplorerApi(config));
    this.register(
      ContainerTokens.RoutingExplorerRepository,
      new RoutingExplorerRepositoryImpl(this.routingExplorerApi),
    );
    this.register(
      ContainerTokens.GetRoutingExplorerDemoUseCase,
      new GetRoutingExplorerDemoUseCase(this.routingExplorerRepository),
    );

    this.register(ContainerTokens.MiddlewarePipelineApi, new MiddlewarePipelineApi(config));
    this.register(
      ContainerTokens.MiddlewarePipelineRepository,
      new MiddlewarePipelineRepositoryImpl(this.middlewarePipelineApi),
    );
    this.register(
      ContainerTokens.GetMiddlewarePipelineDemoUseCase,
      new GetMiddlewarePipelineDemoUseCase(this.middlewarePipelineRepository),
    );

    this.register(ContainerTokens.ReqresInspectorApi, new ReqresInspectorApi(config));
    this.register(
      ContainerTokens.ReqresInspectorRepository,
      new ReqresInspectorRepositoryImpl(this.reqresInspectorApi),
    );
    this.register(
      ContainerTokens.GetReqresInspectorDemoUseCase,
      new GetReqresInspectorDemoUseCase(this.reqresInspectorRepository),
    );

    this.register(ContainerTokens.ReqresMethodsApi, new ReqresMethodsApi(config));
    this.register(
      ContainerTokens.ReqresMethodsRepository,
      new ReqresMethodsRepositoryImpl(this.reqresMethodsApi),
    );
    this.register(
      ContainerTokens.GetReqresMethodsDemoUseCase,
      new GetReqresMethodsDemoUseCase(this.reqresMethodsRepository),
    );

    this.register(ContainerTokens.ValidationArenaApi, new ValidationArenaApi(config));
    this.register(
      ContainerTokens.ValidationArenaRepository,
      new ValidationArenaRepositoryImpl(this.validationArenaApi),
    );
    this.register(
      ContainerTokens.GetValidationArenaDemoUseCase,
      new GetValidationArenaDemoUseCase(this.validationArenaRepository),
    );

    // The posts resource pipeline — the first REAL resource slice (the
    // api-design topic's /api/posts CRUD): one API + repository shared by
    // four verb use cases, mirroring the server's composition root.
    this.register(ContainerTokens.PostsApi, new PostsApi(config));
    this.register(
      ContainerTokens.PostsRepository,
      new PostsRepositoryImpl(this.postsApi),
    );
    this.register(
      ContainerTokens.ListPostsUseCase,
      new ListPostsUseCase(this.postsRepository),
    );
    this.register(
      ContainerTokens.CreatePostsUseCase,
      new CreatePostUseCase(this.postsRepository),
    );
    this.register(
      ContainerTokens.ReplacePostsUseCase,
      new ReplacePostUseCase(this.postsRepository),
    );
    this.register(
      ContainerTokens.DeletePostsUseCase,
      new DeletePostUseCase(this.postsRepository),
    );

    this.register(ContainerTokens.PostsWorkbenchApi, new PostsWorkbenchApi(config));
    this.register(
      ContainerTokens.PostsWorkbenchRepository,
      new PostsWorkbenchRepositoryImpl(this.postsWorkbenchApi),
    );
    this.register(
      ContainerTokens.GetPostsWorkbenchDemoUseCase,
      new GetPostsWorkbenchDemoUseCase(this.postsWorkbenchRepository),
    );

    this.register(ContainerTokens.WebhookSimulatorApi, new WebhookSimulatorApi(config));
    this.register(
      ContainerTokens.WebhookSimulatorRepository,
      new WebhookSimulatorRepositoryImpl(this.webhookSimulatorApi),
    );
    this.register(
      ContainerTokens.GetWebhookSimulatorDemoUseCase,
      new GetWebhookSimulatorDemoUseCase(this.webhookSimulatorRepository),
    );

    this.register(ContainerTokens.DocsLinkApi, new DocsLinkApi(config));
    this.register(
      ContainerTokens.DocsLinkRepository,
      new DocsLinkRepositoryImpl(this.docsLinkApi),
    );
    this.register(
      ContainerTokens.GetDocsLinkDemoUseCase,
      new GetDocsLinkDemoUseCase(this.docsLinkRepository),
    );

    // The graphql playground: ONE api + ONE repository, TWO use cases over
    // that shared repository (payload + RUN button).
    this.register(ContainerTokens.GraphQLPlaygroundApi, new GraphQLPlaygroundApi(config));
    this.register(
      ContainerTokens.GraphQLPlaygroundRepository,
      new GraphQLPlaygroundRepositoryImpl(this.graphqlPlaygroundApi),
    );
    this.register(
      ContainerTokens.GetGraphQLPlaygroundDemoUseCase,
      new GetGraphQLPlaygroundDemoUseCase(this.graphqlPlaygroundRepository),
    );
    this.register(
      ContainerTokens.ExecuteGraphQLDemoQueryUseCase,
      new ExecuteGraphQLDemoQueryUseCase(this.graphqlPlaygroundRepository),
    );

    this.register(ContainerTokens.GRPCConsoleApi, new GRPCConsoleApi(config));
    this.register(
      ContainerTokens.GRPCConsoleRepository,
      new GRPCConsoleRepositoryImpl(this.grpcConsoleApi),
    );
    this.register(
      ContainerTokens.GetGRPCConsoleDemoUseCase,
      new GetGRPCConsoleDemoUseCase(this.grpcConsoleRepository),
    );

    // The sql-schema topic's db demos: the live-schema introspection
    // pipeline and the UUIDv7 lab, over the same config → api →
    // repository → use case shape.
    this.register(ContainerTokens.SchemaExplorerApi, new SchemaExplorerApi(config));
    this.register(
      ContainerTokens.SchemaExplorerRepository,
      new SchemaExplorerRepositoryImpl(this.schemaExplorerApi),
    );
    this.register(
      ContainerTokens.GetSchemaExplorerDemoUseCase,
      new GetSchemaExplorerDemoUseCase(this.schemaExplorerRepository),
    );

    this.register(ContainerTokens.UUIDLabApi, new UUIDLabApi(config));
    this.register(
      ContainerTokens.UUIDLabRepository,
      new UUIDLabRepositoryImpl(this.uuidLabApi),
    );
    this.register(
      ContainerTokens.GetUUIDLabDemoUseCase,
      new GetUUIDLabDemoUseCase(this.uuidLabRepository),
    );

    // The databases-sql topic's demo sextet — one api + repository + use
    // case per demo, all over the same config → api → repository → use
    // case shape.
    this.register(ContainerTokens.SQLSafetyApi, new SQLSafetyApi(config));
    this.register(
      ContainerTokens.SQLSafetyRepository,
      new SQLSafetyRepositoryImpl(this.sqlSafetyApi),
    );
    this.register(
      ContainerTokens.GetSQLSafetyDemoUseCase,
      new GetSQLSafetyDemoUseCase(this.sqlSafetyRepository),
    );

    this.register(ContainerTokens.MigrationsApi, new MigrationsApi(config));
    this.register(
      ContainerTokens.MigrationsRepository,
      new MigrationsRepositoryImpl(this.migrationsApi),
    );
    this.register(
      ContainerTokens.GetMigrationsDemoUseCase,
      new GetMigrationsDemoUseCase(this.migrationsRepository),
    );

    this.register(ContainerTokens.SQLLabApi, new SQLLabApi(config));
    this.register(
      ContainerTokens.SQLLabRepository,
      new SQLLabRepositoryImpl(this.sqlLabApi),
    );
    this.register(
      ContainerTokens.GetSQLLabDemoUseCase,
      new GetSQLLabDemoUseCase(this.sqlLabRepository),
    );

    this.register(ContainerTokens.SearchApi, new SearchApi(config));
    this.register(
      ContainerTokens.SearchRepository,
      new SearchRepositoryImpl(this.searchApi),
    );
    this.register(
      ContainerTokens.GetSearchDemoUseCase,
      new GetSearchDemoUseCase(this.searchRepository),
    );

    this.register(ContainerTokens.RaceApi, new RaceApi(config));
    this.register(
      ContainerTokens.RaceRepository,
      new RaceRepositoryImpl(this.raceApi),
    );
    this.register(
      ContainerTokens.GetRaceDemoUseCase,
      new GetRaceDemoUseCase(this.raceRepository),
    );

    this.register(ContainerTokens.AuditApi, new AuditApi(config));
    this.register(
      ContainerTokens.AuditRepository,
      new AuditRepositoryImpl(this.auditApi),
    );
    this.register(
      ContainerTokens.GetAuditDemoUseCase,
      new GetAuditDemoUseCase(this.auditRepository),
    );

    this.register(ContainerTokens.ORMProbeApi, new ORMProbeApi(config));
    this.register(
      ContainerTokens.ORMProbeRepository,
      new ORMProbeRepositoryImpl(this.ormProbeApi),
    );
    this.register(
      ContainerTokens.GetORMProbeDemoUseCase,
      new GetORMProbeDemoUseCase(this.ormProbeRepository),
    );

    this.register(ContainerTokens.MongoConsoleApi, new MongoConsoleApi(config));
    this.register(
      ContainerTokens.MongoConsoleRepository,
      new MongoConsoleRepositoryImpl(this.mongoConsoleApi),
    );
    this.register(
      ContainerTokens.GetMongoConsoleDemoUseCase,
      new GetMongoConsoleDemoUseCase(this.mongoConsoleRepository),
    );

    this.register(ContainerTokens.RedisPlaygroundApi, new RedisPlaygroundApi(config));
    this.register(
      ContainerTokens.RedisPlaygroundRepository,
      new RedisPlaygroundRepositoryImpl(this.redisPlaygroundApi),
    );
    this.register(
      ContainerTokens.GetRedisPlaygroundDemoUseCase,
      new GetRedisPlaygroundDemoUseCase(this.redisPlaygroundRepository),
    );

    this.register(ContainerTokens.CachingLabApi, new CachingLabApi(config));
    this.register(
      ContainerTokens.CachingLabRepository,
      new CachingLabRepositoryImpl(this.cachingLabApi),
    );
    this.register(
      ContainerTokens.GetCachingLabDemoUseCase,
      new GetCachingLabDemoUseCase(this.cachingLabRepository),
    );

    this.register(ContainerTokens.MessagingStreamApi, new MessagingStreamApi(config));
    this.register(
      ContainerTokens.MessagingStreamRepository,
      new MessagingStreamRepositoryImpl(this.messagingStreamApi),
    );
    this.register(
      ContainerTokens.GetMessagingStreamDemoUseCase,
      new GetMessagingStreamDemoUseCase(this.messagingStreamRepository),
    );

    this.register(ContainerTokens.MessagingOutboxApi, new MessagingOutboxApi(config));
    this.register(
      ContainerTokens.MessagingOutboxRepository,
      new MessagingOutboxRepositoryImpl(this.messagingOutboxApi),
    );
    this.register(
      ContainerTokens.GetMessagingOutboxDemoUseCase,
      new GetMessagingOutboxDemoUseCase(this.messagingOutboxRepository),
    );

    // The seven auth story pipelines: same shape, one adapter class
    // parameterized by endpoint path, one register triple per demo.
    const authStoryPipelines = [
      [ContainerTokens.GetAuthSessionsDemoUseCase, AUTH_STORY_PATHS.sessions],
      [ContainerTokens.GetAuthAPIKeysDemoUseCase, AUTH_STORY_PATHS.apiKeys],
      [ContainerTokens.GetAuthCompareDemoUseCase, AUTH_STORY_PATHS.compare],
      [ContainerTokens.GetAuthJWTDemoUseCase, AUTH_STORY_PATHS.jwt],
      [ContainerTokens.GetAuthOAuthDemoUseCase, AUTH_STORY_PATHS.oauth],
      [ContainerTokens.GetAuthHashingDemoUseCase, AUTH_STORY_PATHS.hashing],
      [ContainerTokens.GetAuthRefreshDemoUseCase, AUTH_STORY_PATHS.refresh],
    ] as const;
    for (const [token, path] of authStoryPipelines) {
      const repository = new AuthStoryRepositoryImpl(new AuthStoryApi(config, path));
      this.register(token, new GetAuthStoryDemoUseCase(repository));
    }

    // Topic 24's authz matrix — the classic single-demo triple.
    const authzMatrixRepository = new AuthzMatrixRepositoryImpl(
      new AuthzMatrixApi(config),
    );
    this.register(
      ContainerTokens.GetAuthzMatrixDemoUseCase,
      new GetAuthzMatrixDemoUseCase(authzMatrixRepository),
    );

    // Topic 25's two security stories: the auth loop pattern again.
    const securityStoryPipelines = [
      [ContainerTokens.GetSecurityLabDemoUseCase, SECURITY_STORY_PATHS.lab],
      [ContainerTokens.GetSecurityPIIDemoUseCase, SECURITY_STORY_PATHS.pii],
    ] as const;
    for (const [token, path] of securityStoryPipelines) {
      const repository = new SecurityStoryRepositoryImpl(new SecurityStoryApi(config, path));
      this.register(token, new GetSecurityStoryDemoUseCase(repository));
    }

    // Topics 26 + 27: single-story pipelines, the same shape again.
    this.register(
      ContainerTokens.GetFilesWorkbenchDemoUseCase,
      new GetFilesWorkbenchDemoUseCase(
        new FilesStoryRepositoryImpl(new FilesStoryApi(config, FILES_STORY_PATH)),
      ),
    );
    this.register(
      ContainerTokens.GetI18nNegotiatorDemoUseCase,
      new GetI18nNegotiatorDemoUseCase(
        new I18nStoryRepositoryImpl(new I18nStoryApi(config, I18N_STORY_PATH)),
      ),
    );

    // Topic 28's two realtime stories: the auth loop pattern once more.
    const realtimeStoryPipelines = [
      [ContainerTokens.GetRealtimeTickerDemoUseCase, REALTIME_STORY_PATHS.ticker],
      [ContainerTokens.GetRealtimeChatDemoUseCase, REALTIME_STORY_PATHS.chat],
    ] as const;
    for (const [token, path] of realtimeStoryPipelines) {
      const repository = new RealtimeStoryRepositoryImpl(new RealtimeStoryApi(config, path));
      this.register(token, new GetRealtimeStoryDemoUseCase(repository));
    }

    // Topic 29's jobs console — the classic single-demo triple.
    this.register(ContainerTokens.JobsConsoleApi, new JobsConsoleApi(config));
    this.register(
      ContainerTokens.JobsConsoleRepository,
      new JobsConsoleRepositoryImpl(this.jobsConsoleApi),
    );
    this.register(
      ContainerTokens.GetJobsConsoleDemoUseCase,
      new GetJobsConsoleDemoUseCase(this.jobsConsoleRepository),
    );

    // Topic 30's errors explorer — the same shape once more.
    this.register(ContainerTokens.ErrorsExplorerApi, new ErrorsExplorerApi(config));
    this.register(
      ContainerTokens.ErrorsExplorerRepository,
      new ErrorsExplorerRepositoryImpl(this.errorsExplorerApi),
    );
    this.register(
      ContainerTokens.GetErrorsExplorerDemoUseCase,
      new GetErrorsExplorerDemoUseCase(this.errorsExplorerRepository),
    );

    // Topic 31's deps manifest — the classic single-demo triple.
    this.register(ContainerTokens.DepsManifestApi, new DepsManifestApi(config));
    this.register(
      ContainerTokens.DepsManifestRepository,
      new DepsManifestRepositoryImpl(this.depsManifestApi),
    );
    this.register(
      ContainerTokens.GetDepsManifestDemoUseCase,
      new GetDepsManifestDemoUseCase(this.depsManifestRepository),
    );

    // Topic 32's architecture tracer — the same shape once more.
    this.register(ContainerTokens.ArchTracerApi, new ArchTracerApi(config));
    this.register(
      ContainerTokens.ArchTracerRepository,
      new ArchTracerRepositoryImpl(this.archTracerApi),
    );
    this.register(
      ContainerTokens.GetArchTracerDemoUseCase,
      new GetArchTracerDemoUseCase(this.archTracerRepository),
    );

    // Topic 33's observability pair — the same shape once more.
    this.register(ContainerTokens.ObsViewerApi, new ObsViewerApi(config));
    this.register(
      ContainerTokens.ObsViewerRepository,
      new ObsViewerRepositoryImpl(this.obsViewerApi),
    );
    this.register(
      ContainerTokens.GetObsViewerDemoUseCase,
      new GetObsViewerDemoUseCase(this.obsViewerRepository),
    );
    this.register(ContainerTokens.ObsTraceApi, new ObsTraceApi(config));
    this.register(
      ContainerTokens.ObsTraceRepository,
      new ObsTraceRepositoryImpl(this.obsTraceApi),
    );
    this.register(
      ContainerTokens.GetObsTraceDemoUseCase,
      new GetObsTraceDemoUseCase(this.obsTraceRepository),
    );

    // Topic 34's testing runner — the same shape once more.
    this.register(ContainerTokens.TestingRunnerApi, new TestingRunnerApi(config));
    this.register(
      ContainerTokens.TestingRunnerRepository,
      new TestingRunnerRepositoryImpl(this.testingRunnerApi),
    );
    this.register(
      ContainerTokens.GetTestingRunnerDemoUseCase,
      new GetTestingRunnerDemoUseCase(this.testingRunnerRepository),
    );

    // Topic 35's debug stall — the same shape once more.
    this.register(ContainerTokens.DebugStallApi, new DebugStallApi(config));
    this.register(
      ContainerTokens.DebugStallRepository,
      new DebugStallRepositoryImpl(this.debugStallApi),
    );
    this.register(
      ContainerTokens.GetDebugStallDemoUseCase,
      new GetDebugStallDemoUseCase(this.debugStallRepository),
    );

    // Topic 36's env inspector — the same shape once more.
    this.register(ContainerTokens.EnvInspectorApi, new EnvInspectorApi(config));
    this.register(
      ContainerTokens.EnvInspectorRepository,
      new EnvInspectorRepositoryImpl(this.envInspectorApi),
    );
    this.register(
      ContainerTokens.GetEnvInspectorDemoUseCase,
      new GetEnvInspectorDemoUseCase(this.envInspectorRepository),
    );
  }

  // ---- typed getters: the only resolution surface views/hooks use ----

  get appConfig(): AppConfig {
    return this.resolve<AppConfig>(ContainerTokens.AppConfig);
  }

  get healthApi(): HealthApi {
    return this.resolve<HealthApi>(ContainerTokens.HealthApi);
  }

  get healthRepository(): HealthRepository {
    return this.resolve<HealthRepository>(ContainerTokens.HealthRepository);
  }

  get getHealthUseCase(): GetHealthUseCase {
    return this.resolve<GetHealthUseCase>(ContainerTokens.GetHealthUseCase);
  }

  get contentApi(): ContentApi {
    return this.resolve<ContentApi>(ContainerTokens.ContentApi);
  }

  get contentRepository(): ContentRepository {
    return this.resolve<ContentRepository>(ContainerTokens.ContentRepository);
  }

  get listContentUseCase(): ListContentUseCase {
    return this.resolve<ListContentUseCase>(ContainerTokens.ListContentUseCase);
  }

  get getTopicDetailUseCase(): GetTopicDetailUseCase {
    return this.resolve<GetTopicDetailUseCase>(ContainerTokens.GetTopicDetailUseCase);
  }

  get scenarioApi(): ScenarioApi {
    return this.resolve<ScenarioApi>(ContainerTokens.ScenarioApi);
  }

  get scenarioRepository(): ScenarioRepository {
    return this.resolve<ScenarioRepository>(ContainerTokens.ScenarioRepository);
  }

  get getScenarioUseCase(): GetScenarioUseCase {
    return this.resolve<GetScenarioUseCase>(ContainerTokens.GetScenarioUseCase);
  }

  get patchScenarioUseCase(): PatchScenarioUseCase {
    return this.resolve<PatchScenarioUseCase>(ContainerTokens.PatchScenarioUseCase);
  }

  get runtimeTypesApi(): RuntimeTypesApi {
    return this.resolve<RuntimeTypesApi>(ContainerTokens.RuntimeTypesApi);
  }

  get runtimeTypesRepository(): RuntimeTypesRepository {
    return this.resolve<RuntimeTypesRepository>(ContainerTokens.RuntimeTypesRepository);
  }

  get getRuntimeTypesDemoUseCase(): GetRuntimeTypesDemoUseCase {
    return this.resolve<GetRuntimeTypesDemoUseCase>(ContainerTokens.GetRuntimeTypesDemoUseCase);
  }

  get concurrencyApi(): ConcurrencyApi {
    return this.resolve<ConcurrencyApi>(ContainerTokens.ConcurrencyApi);
  }

  get concurrencyRepository(): ConcurrencyRepository {
    return this.resolve<ConcurrencyRepository>(ContainerTokens.ConcurrencyRepository);
  }

  get getConcurrencyDemoUseCase(): GetConcurrencyDemoUseCase {
    return this.resolve<GetConcurrencyDemoUseCase>(ContainerTokens.GetConcurrencyDemoUseCase);
  }

  get httpHeadersApi(): HttpHeadersApi {
    return this.resolve<HttpHeadersApi>(ContainerTokens.HttpHeadersApi);
  }

  get httpHeadersRepository(): HttpHeadersRepository {
    return this.resolve<HttpHeadersRepository>(ContainerTokens.HttpHeadersRepository);
  }

  get getHttpHeadersDemoUseCase(): GetHttpHeadersDemoUseCase {
    return this.resolve<GetHttpHeadersDemoUseCase>(ContainerTokens.GetHttpHeadersDemoUseCase);
  }

  get appInfoApi(): AppInfoApi {
    return this.resolve<AppInfoApi>(ContainerTokens.AppInfoApi);
  }

  get appInfoRepository(): AppInfoRepository {
    return this.resolve<AppInfoRepository>(ContainerTokens.AppInfoRepository);
  }

  get getAppInfoDemoUseCase(): GetAppInfoDemoUseCase {
    return this.resolve<GetAppInfoDemoUseCase>(ContainerTokens.GetAppInfoDemoUseCase);
  }

  get interfaceMatrixApi(): InterfaceMatrixApi {
    return this.resolve<InterfaceMatrixApi>(ContainerTokens.InterfaceMatrixApi);
  }

  get interfaceMatrixRepository(): InterfaceMatrixRepository {
    return this.resolve<InterfaceMatrixRepository>(ContainerTokens.InterfaceMatrixRepository);
  }

  get getInterfaceMatrixDemoUseCase(): GetInterfaceMatrixDemoUseCase {
    return this.resolve<GetInterfaceMatrixDemoUseCase>(ContainerTokens.GetInterfaceMatrixDemoUseCase);
  }

  get contextCancellationApi(): ContextCancellationApi {
    return this.resolve<ContextCancellationApi>(ContainerTokens.ContextCancellationApi);
  }

  get contextCancellationRepository(): ContextCancellationRepository {
    return this.resolve<ContextCancellationRepository>(ContainerTokens.ContextCancellationRepository);
  }

  get getContextCancellationDemoUseCase(): GetContextCancellationDemoUseCase {
    return this.resolve<GetContextCancellationDemoUseCase>(
      ContainerTokens.GetContextCancellationDemoUseCase,
    );
  }

  get raceDetectorApi(): RaceDetectorApi {
    return this.resolve<RaceDetectorApi>(ContainerTokens.RaceDetectorApi);
  }

  get raceDetectorRepository(): RaceDetectorRepository {
    return this.resolve<RaceDetectorRepository>(ContainerTokens.RaceDetectorRepository);
  }

  get getRaceDetectorDemoUseCase(): GetRaceDetectorDemoUseCase {
    return this.resolve<GetRaceDetectorDemoUseCase>(ContainerTokens.GetRaceDetectorDemoUseCase);
  }

  get routingExplorerApi(): RoutingExplorerApi {
    return this.resolve<RoutingExplorerApi>(ContainerTokens.RoutingExplorerApi);
  }

  get routingExplorerRepository(): RoutingExplorerRepository {
    return this.resolve<RoutingExplorerRepository>(ContainerTokens.RoutingExplorerRepository);
  }

  get getRoutingExplorerDemoUseCase(): GetRoutingExplorerDemoUseCase {
    return this.resolve<GetRoutingExplorerDemoUseCase>(ContainerTokens.GetRoutingExplorerDemoUseCase);
  }

  get middlewarePipelineApi(): MiddlewarePipelineApi {
    return this.resolve<MiddlewarePipelineApi>(ContainerTokens.MiddlewarePipelineApi);
  }

  get middlewarePipelineRepository(): MiddlewarePipelineRepository {
    return this.resolve<MiddlewarePipelineRepository>(ContainerTokens.MiddlewarePipelineRepository);
  }

  get getMiddlewarePipelineDemoUseCase(): GetMiddlewarePipelineDemoUseCase {
    return this.resolve<GetMiddlewarePipelineDemoUseCase>(
      ContainerTokens.GetMiddlewarePipelineDemoUseCase,
    );
  }

  get reqresInspectorApi(): ReqresInspectorApi {
    return this.resolve<ReqresInspectorApi>(ContainerTokens.ReqresInspectorApi);
  }

  get reqresInspectorRepository(): ReqresInspectorRepository {
    return this.resolve<ReqresInspectorRepository>(ContainerTokens.ReqresInspectorRepository);
  }

  get getReqresInspectorDemoUseCase(): GetReqresInspectorDemoUseCase {
    return this.resolve<GetReqresInspectorDemoUseCase>(ContainerTokens.GetReqresInspectorDemoUseCase);
  }

  get reqresMethodsApi(): ReqresMethodsApi {
    return this.resolve<ReqresMethodsApi>(ContainerTokens.ReqresMethodsApi);
  }

  get reqresMethodsRepository(): ReqresMethodsRepository {
    return this.resolve<ReqresMethodsRepository>(ContainerTokens.ReqresMethodsRepository);
  }

  get getReqresMethodsDemoUseCase(): GetReqresMethodsDemoUseCase {
    return this.resolve<GetReqresMethodsDemoUseCase>(ContainerTokens.GetReqresMethodsDemoUseCase);
  }

  get validationArenaApi(): ValidationArenaApi {
    return this.resolve<ValidationArenaApi>(ContainerTokens.ValidationArenaApi);
  }

  get validationArenaRepository(): ValidationArenaRepository {
    return this.resolve<ValidationArenaRepository>(ContainerTokens.ValidationArenaRepository);
  }

  get getValidationArenaDemoUseCase(): GetValidationArenaDemoUseCase {
    return this.resolve<GetValidationArenaDemoUseCase>(ContainerTokens.GetValidationArenaDemoUseCase);
  }

  get postsApi(): PostsApi {
    return this.resolve<PostsApi>(ContainerTokens.PostsApi);
  }

  get postsRepository(): PostsRepository {
    return this.resolve<PostsRepository>(ContainerTokens.PostsRepository);
  }

  get listPostsUseCase(): ListPostsUseCase {
    return this.resolve<ListPostsUseCase>(ContainerTokens.ListPostsUseCase);
  }

  get createPostUseCase(): CreatePostUseCase {
    return this.resolve<CreatePostUseCase>(ContainerTokens.CreatePostsUseCase);
  }

  get replacePostUseCase(): ReplacePostUseCase {
    return this.resolve<ReplacePostUseCase>(ContainerTokens.ReplacePostsUseCase);
  }

  get deletePostUseCase(): DeletePostUseCase {
    return this.resolve<DeletePostUseCase>(ContainerTokens.DeletePostsUseCase);
  }

  get postsWorkbenchApi(): PostsWorkbenchApi {
    return this.resolve<PostsWorkbenchApi>(ContainerTokens.PostsWorkbenchApi);
  }

  get postsWorkbenchRepository(): PostsWorkbenchRepository {
    return this.resolve<PostsWorkbenchRepository>(ContainerTokens.PostsWorkbenchRepository);
  }

  get getPostsWorkbenchDemoUseCase(): GetPostsWorkbenchDemoUseCase {
    return this.resolve<GetPostsWorkbenchDemoUseCase>(ContainerTokens.GetPostsWorkbenchDemoUseCase);
  }

  get webhookSimulatorApi(): WebhookSimulatorApi {
    return this.resolve<WebhookSimulatorApi>(ContainerTokens.WebhookSimulatorApi);
  }

  get webhookSimulatorRepository(): WebhookSimulatorRepository {
    return this.resolve<WebhookSimulatorRepository>(ContainerTokens.WebhookSimulatorRepository);
  }

  get getWebhookSimulatorDemoUseCase(): GetWebhookSimulatorDemoUseCase {
    return this.resolve<GetWebhookSimulatorDemoUseCase>(
      ContainerTokens.GetWebhookSimulatorDemoUseCase,
    );
  }

  get docsLinkApi(): DocsLinkApi {
    return this.resolve<DocsLinkApi>(ContainerTokens.DocsLinkApi);
  }

  get docsLinkRepository(): DocsLinkRepository {
    return this.resolve<DocsLinkRepository>(ContainerTokens.DocsLinkRepository);
  }

  get getDocsLinkDemoUseCase(): GetDocsLinkDemoUseCase {
    return this.resolve<GetDocsLinkDemoUseCase>(ContainerTokens.GetDocsLinkDemoUseCase);
  }

  get graphqlPlaygroundApi(): GraphQLPlaygroundApi {
    return this.resolve<GraphQLPlaygroundApi>(ContainerTokens.GraphQLPlaygroundApi);
  }

  // The intersection of the demo's TWO ports: the one registered repository
  // (the impl adapts to both) serves the payload use case and the RUN use
  // case alike — one transport, one server.
  get graphqlPlaygroundRepository(): GraphQLPlaygroundRepository & GraphQLQueryRepository {
    return this.resolve<GraphQLPlaygroundRepository & GraphQLQueryRepository>(
      ContainerTokens.GraphQLPlaygroundRepository,
    );
  }

  get getGraphQLPlaygroundDemoUseCase(): GetGraphQLPlaygroundDemoUseCase {
    return this.resolve<GetGraphQLPlaygroundDemoUseCase>(
      ContainerTokens.GetGraphQLPlaygroundDemoUseCase,
    );
  }

  get executeGraphQLDemoQueryUseCase(): ExecuteGraphQLDemoQueryUseCase {
    return this.resolve<ExecuteGraphQLDemoQueryUseCase>(
      ContainerTokens.ExecuteGraphQLDemoQueryUseCase,
    );
  }

  get grpcConsoleApi(): GRPCConsoleApi {
    return this.resolve<GRPCConsoleApi>(ContainerTokens.GRPCConsoleApi);
  }

  get grpcConsoleRepository(): GRPCConsoleRepository {
    return this.resolve<GRPCConsoleRepository>(ContainerTokens.GRPCConsoleRepository);
  }

  get getGRPCConsoleDemoUseCase(): GetGRPCConsoleDemoUseCase {
    return this.resolve<GetGRPCConsoleDemoUseCase>(ContainerTokens.GetGRPCConsoleDemoUseCase);
  }

  get schemaExplorerApi(): SchemaExplorerApi {
    return this.resolve<SchemaExplorerApi>(ContainerTokens.SchemaExplorerApi);
  }

  get schemaExplorerRepository(): SchemaExplorerRepository {
    return this.resolve<SchemaExplorerRepository>(ContainerTokens.SchemaExplorerRepository);
  }

  get getSchemaExplorerDemoUseCase(): GetSchemaExplorerDemoUseCase {
    return this.resolve<GetSchemaExplorerDemoUseCase>(ContainerTokens.GetSchemaExplorerDemoUseCase);
  }

  get uuidLabApi(): UUIDLabApi {
    return this.resolve<UUIDLabApi>(ContainerTokens.UUIDLabApi);
  }

  get uuidLabRepository(): UUIDLabRepository {
    return this.resolve<UUIDLabRepository>(ContainerTokens.UUIDLabRepository);
  }

  get getUUIDLabDemoUseCase(): GetUUIDLabDemoUseCase {
    return this.resolve<GetUUIDLabDemoUseCase>(ContainerTokens.GetUUIDLabDemoUseCase);
  }

  get sqlSafetyApi(): SQLSafetyApi {
    return this.resolve<SQLSafetyApi>(ContainerTokens.SQLSafetyApi);
  }

  get sqlSafetyRepository(): SQLSafetyRepository {
    return this.resolve<SQLSafetyRepository>(ContainerTokens.SQLSafetyRepository);
  }

  get getSQLSafetyDemoUseCase(): GetSQLSafetyDemoUseCase {
    return this.resolve<GetSQLSafetyDemoUseCase>(ContainerTokens.GetSQLSafetyDemoUseCase);
  }

  get migrationsApi(): MigrationsApi {
    return this.resolve<MigrationsApi>(ContainerTokens.MigrationsApi);
  }

  get migrationsRepository(): MigrationsRepository {
    return this.resolve<MigrationsRepository>(ContainerTokens.MigrationsRepository);
  }

  get getMigrationsDemoUseCase(): GetMigrationsDemoUseCase {
    return this.resolve<GetMigrationsDemoUseCase>(ContainerTokens.GetMigrationsDemoUseCase);
  }

  get sqlLabApi(): SQLLabApi {
    return this.resolve<SQLLabApi>(ContainerTokens.SQLLabApi);
  }

  get sqlLabRepository(): SQLLabRepository {
    return this.resolve<SQLLabRepository>(ContainerTokens.SQLLabRepository);
  }

  get getSQLLabDemoUseCase(): GetSQLLabDemoUseCase {
    return this.resolve<GetSQLLabDemoUseCase>(ContainerTokens.GetSQLLabDemoUseCase);
  }

  get searchApi(): SearchApi {
    return this.resolve<SearchApi>(ContainerTokens.SearchApi);
  }

  get searchRepository(): SearchRepository {
    return this.resolve<SearchRepository>(ContainerTokens.SearchRepository);
  }

  get getSearchDemoUseCase(): GetSearchDemoUseCase {
    return this.resolve<GetSearchDemoUseCase>(ContainerTokens.GetSearchDemoUseCase);
  }

  get raceApi(): RaceApi {
    return this.resolve<RaceApi>(ContainerTokens.RaceApi);
  }

  get raceRepository(): RaceRepository {
    return this.resolve<RaceRepository>(ContainerTokens.RaceRepository);
  }

  get getRaceDemoUseCase(): GetRaceDemoUseCase {
    return this.resolve<GetRaceDemoUseCase>(ContainerTokens.GetRaceDemoUseCase);
  }

  get auditApi(): AuditApi {
    return this.resolve<AuditApi>(ContainerTokens.AuditApi);
  }

  get auditRepository(): AuditRepository {
    return this.resolve<AuditRepository>(ContainerTokens.AuditRepository);
  }

  get getAuditDemoUseCase(): GetAuditDemoUseCase {
    return this.resolve<GetAuditDemoUseCase>(ContainerTokens.GetAuditDemoUseCase);
  }

  get ormProbeApi(): ORMProbeApi {
    return this.resolve<ORMProbeApi>(ContainerTokens.ORMProbeApi);
  }

  get ormProbeRepository(): ORMProbeRepository {
    return this.resolve<ORMProbeRepository>(ContainerTokens.ORMProbeRepository);
  }

  get getORMProbeDemoUseCase(): GetORMProbeDemoUseCase {
    return this.resolve<GetORMProbeDemoUseCase>(ContainerTokens.GetORMProbeDemoUseCase);
  }

  get mongoConsoleApi(): MongoConsoleApi {
    return this.resolve<MongoConsoleApi>(ContainerTokens.MongoConsoleApi);
  }

  get mongoConsoleRepository(): MongoConsoleRepository {
    return this.resolve<MongoConsoleRepository>(ContainerTokens.MongoConsoleRepository);
  }

  get getMongoConsoleDemoUseCase(): GetMongoConsoleDemoUseCase {
    return this.resolve<GetMongoConsoleDemoUseCase>(ContainerTokens.GetMongoConsoleDemoUseCase);
  }

  get redisPlaygroundApi(): RedisPlaygroundApi {
    return this.resolve<RedisPlaygroundApi>(ContainerTokens.RedisPlaygroundApi);
  }

  get redisPlaygroundRepository(): RedisPlaygroundRepository {
    return this.resolve<RedisPlaygroundRepository>(ContainerTokens.RedisPlaygroundRepository);
  }

  get getRedisPlaygroundDemoUseCase(): GetRedisPlaygroundDemoUseCase {
    return this.resolve<GetRedisPlaygroundDemoUseCase>(
      ContainerTokens.GetRedisPlaygroundDemoUseCase,
    );
  }

  get cachingLabApi(): CachingLabApi {
    return this.resolve<CachingLabApi>(ContainerTokens.CachingLabApi);
  }

  get cachingLabRepository(): CachingLabRepository {
    return this.resolve<CachingLabRepository>(ContainerTokens.CachingLabRepository);
  }

  get getCachingLabDemoUseCase(): GetCachingLabDemoUseCase {
    return this.resolve<GetCachingLabDemoUseCase>(
      ContainerTokens.GetCachingLabDemoUseCase,
    );
  }

  get messagingStreamApi(): MessagingStreamApi {
    return this.resolve<MessagingStreamApi>(ContainerTokens.MessagingStreamApi);
  }

  get messagingStreamRepository(): MessagingStreamRepository {
    return this.resolve<MessagingStreamRepository>(ContainerTokens.MessagingStreamRepository);
  }

  get getMessagingStreamDemoUseCase(): GetMessagingStreamDemoUseCase {
    return this.resolve<GetMessagingStreamDemoUseCase>(
      ContainerTokens.GetMessagingStreamDemoUseCase,
    );
  }

  get messagingOutboxApi(): MessagingOutboxApi {
    return this.resolve<MessagingOutboxApi>(ContainerTokens.MessagingOutboxApi);
  }

  get messagingOutboxRepository(): MessagingOutboxRepository {
    return this.resolve<MessagingOutboxRepository>(ContainerTokens.MessagingOutboxRepository);
  }

  get getMessagingOutboxDemoUseCase(): GetMessagingOutboxDemoUseCase {
    return this.resolve<GetMessagingOutboxDemoUseCase>(
      ContainerTokens.GetMessagingOutboxDemoUseCase,
    );
  }

  get getAuthSessionsDemoUseCase(): GetAuthStoryDemoUseCase {
    return this.resolve<GetAuthStoryDemoUseCase>(ContainerTokens.GetAuthSessionsDemoUseCase);
  }

  get getAuthAPIKeysDemoUseCase(): GetAuthStoryDemoUseCase {
    return this.resolve<GetAuthStoryDemoUseCase>(ContainerTokens.GetAuthAPIKeysDemoUseCase);
  }

  get getAuthCompareDemoUseCase(): GetAuthStoryDemoUseCase {
    return this.resolve<GetAuthStoryDemoUseCase>(ContainerTokens.GetAuthCompareDemoUseCase);
  }

  get getAuthJWTDemoUseCase(): GetAuthStoryDemoUseCase {
    return this.resolve<GetAuthStoryDemoUseCase>(ContainerTokens.GetAuthJWTDemoUseCase);
  }

  get getAuthOAuthDemoUseCase(): GetAuthStoryDemoUseCase {
    return this.resolve<GetAuthStoryDemoUseCase>(ContainerTokens.GetAuthOAuthDemoUseCase);
  }

  get getAuthHashingDemoUseCase(): GetAuthStoryDemoUseCase {
    return this.resolve<GetAuthStoryDemoUseCase>(ContainerTokens.GetAuthHashingDemoUseCase);
  }

  get getAuthRefreshDemoUseCase(): GetAuthStoryDemoUseCase {
    return this.resolve<GetAuthStoryDemoUseCase>(ContainerTokens.GetAuthRefreshDemoUseCase);
  }

  get jobsConsoleApi(): JobsConsoleApi {
    return this.resolve<JobsConsoleApi>(ContainerTokens.JobsConsoleApi);
  }

  get jobsConsoleRepository(): JobsConsoleRepository {
    return this.resolve<JobsConsoleRepository>(ContainerTokens.JobsConsoleRepository);
  }

  get getJobsConsoleDemoUseCase(): GetJobsConsoleDemoUseCase {
    return this.resolve<GetJobsConsoleDemoUseCase>(
      ContainerTokens.GetJobsConsoleDemoUseCase,
    );
  }

  get errorsExplorerApi(): ErrorsExplorerApi {
    return this.resolve<ErrorsExplorerApi>(ContainerTokens.ErrorsExplorerApi);
  }

  get errorsExplorerRepository(): ErrorsExplorerRepository {
    return this.resolve<ErrorsExplorerRepository>(ContainerTokens.ErrorsExplorerRepository);
  }

  get getErrorsExplorerDemoUseCase(): GetErrorsExplorerDemoUseCase {
    return this.resolve<GetErrorsExplorerDemoUseCase>(
      ContainerTokens.GetErrorsExplorerDemoUseCase,
    );
  }

  get depsManifestApi(): DepsManifestApi {
    return this.resolve<DepsManifestApi>(ContainerTokens.DepsManifestApi);
  }

  get depsManifestRepository(): DepsManifestRepository {
    return this.resolve<DepsManifestRepository>(ContainerTokens.DepsManifestRepository);
  }

  get getDepsManifestDemoUseCase(): GetDepsManifestDemoUseCase {
    return this.resolve<GetDepsManifestDemoUseCase>(
      ContainerTokens.GetDepsManifestDemoUseCase,
    );
  }

  get archTracerApi(): ArchTracerApi {
    return this.resolve<ArchTracerApi>(ContainerTokens.ArchTracerApi);
  }

  get archTracerRepository(): ArchTracerRepository {
    return this.resolve<ArchTracerRepository>(ContainerTokens.ArchTracerRepository);
  }

  get getArchTracerDemoUseCase(): GetArchTracerDemoUseCase {
    return this.resolve<GetArchTracerDemoUseCase>(
      ContainerTokens.GetArchTracerDemoUseCase,
    );
  }

  get obsViewerApi(): ObsViewerApi {
    return this.resolve<ObsViewerApi>(ContainerTokens.ObsViewerApi);
  }

  get obsViewerRepository(): ObsViewerRepository {
    return this.resolve<ObsViewerRepository>(ContainerTokens.ObsViewerRepository);
  }

  get getObsViewerDemoUseCase(): GetObsViewerDemoUseCase {
    return this.resolve<GetObsViewerDemoUseCase>(
      ContainerTokens.GetObsViewerDemoUseCase,
    );
  }

  get obsTraceApi(): ObsTraceApi {
    return this.resolve<ObsTraceApi>(ContainerTokens.ObsTraceApi);
  }

  get obsTraceRepository(): ObsTraceRepository {
    return this.resolve<ObsTraceRepository>(ContainerTokens.ObsTraceRepository);
  }

  get getObsTraceDemoUseCase(): GetObsTraceDemoUseCase {
    return this.resolve<GetObsTraceDemoUseCase>(
      ContainerTokens.GetObsTraceDemoUseCase,
    );
  }

  get testingRunnerApi(): TestingRunnerApi {
    return this.resolve<TestingRunnerApi>(ContainerTokens.TestingRunnerApi);
  }

  get testingRunnerRepository(): TestingRunnerRepository {
    return this.resolve<TestingRunnerRepository>(ContainerTokens.TestingRunnerRepository);
  }

  get getTestingRunnerDemoUseCase(): GetTestingRunnerDemoUseCase {
    return this.resolve<GetTestingRunnerDemoUseCase>(
      ContainerTokens.GetTestingRunnerDemoUseCase,
    );
  }

  get debugStallApi(): DebugStallApi {
    return this.resolve<DebugStallApi>(ContainerTokens.DebugStallApi);
  }

  get debugStallRepository(): DebugStallRepository {
    return this.resolve<DebugStallRepository>(ContainerTokens.DebugStallRepository);
  }

  get getDebugStallDemoUseCase(): GetDebugStallDemoUseCase {
    return this.resolve<GetDebugStallDemoUseCase>(
      ContainerTokens.GetDebugStallDemoUseCase,
    );
  }

  get envInspectorApi(): EnvInspectorApi {
    return this.resolve<EnvInspectorApi>(ContainerTokens.EnvInspectorApi);
  }

  get envInspectorRepository(): EnvInspectorRepository {
    return this.resolve<EnvInspectorRepository>(ContainerTokens.EnvInspectorRepository);
  }

  get getEnvInspectorDemoUseCase(): GetEnvInspectorDemoUseCase {
    return this.resolve<GetEnvInspectorDemoUseCase>(
      ContainerTokens.GetEnvInspectorDemoUseCase,
    );
  }

  // ---- wiring helpers ----

  /** Register (or replace) a singleton under its token. */
  register(token: symbol, instance: unknown): void {
    this.registry.set(token, instance);
  }

  /** Resolve a singleton; throws loudly when the token was never registered. */
  resolve<T>(token: symbol): T {
    const instance = this.registry.get(token);
    if (instance === undefined) {
      throw new Error(`Dependency "${String(token.description)}" is not registered`);
    }
    return instance as T;
  }

  /** Test-only: the raw symbol → instance wiring, for wiring assertions. */
  getWiring(): ReadonlyMap<symbol, unknown> {
    return this.registry;
  }

  /** Test-only: tear down registered singletons. */
  dispose(): void {
    this.registry.clear();
  }
}

export const container = new Container();
