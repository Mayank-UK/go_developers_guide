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
