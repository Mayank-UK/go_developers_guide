import { beforeAll, describe, expect, it } from 'vitest';
import { container, ContainerTokens } from './container';
import { createAppConfig } from './config/app-config';
import { ContentRepositoryImpl } from '../data/content/content-repository-impl';
import { HealthRepositoryImpl } from '../data/health/health-repository-impl';
import { ScenarioRepositoryImpl } from '../data/scenario/scenario-repository-impl';
import { GetTopicDetailUseCase } from '../domain/content/get-topic-detail-usecase';
import { ListContentUseCase } from '../domain/content/list-content-usecase';
import { GetHealthUseCase } from '../domain/health/get-health-usecase';
import { GetScenarioUseCase } from '../domain/scenario/get-scenario-usecase';
import { PatchScenarioUseCase } from '../domain/scenario/patch-scenario-usecase';
import { RuntimeTypesRepositoryImpl } from '../data/demo/runtime-types-repository-impl';
import { GetRuntimeTypesDemoUseCase } from '../domain/demo/get-runtime-types-demo-usecase';
import { ConcurrencyRepositoryImpl } from '../data/demo/concurrency-repository-impl';
import { GetConcurrencyDemoUseCase } from '../domain/demo/get-concurrency-demo-usecase';
import { HttpHeadersRepositoryImpl } from '../data/demo/http-headers-repository-impl';
import {
  GetHttpHeadersDemoUseCase,
  HTTP_HEADERS_DEMO_USE_CASE,
} from '../domain/demo/get-http-headers-demo-usecase';
import { AppInfoRepositoryImpl } from '../data/demo/app-info-repository-impl';
import {
  GetAppInfoDemoUseCase,
  GetAppInfoDemoUseCaseToken,
} from '../domain/demo/get-app-info-demo-usecase';
import { InterfaceMatrixRepositoryImpl } from '../data/demo/interface-matrix-repository-impl';
import {
  GetInterfaceMatrixDemoUseCase,
  INTERFACE_MATRIX_DEMO_USE_CASE,
} from '../domain/demo/get-interface-matrix-demo-usecase';
import { ContextCancellationRepositoryImpl } from '../data/demo/context-cancellation-repository-impl';
import { GetContextCancellationDemoUseCase } from '../domain/demo/get-context-cancellation-demo-usecase';
import { RaceDetectorRepositoryImpl } from '../data/demo/race-detector-repository-impl';
import { GetRaceDetectorDemoUseCase } from '../domain/demo/get-race-detector-demo-usecase';
import { RoutingExplorerRepositoryImpl } from '../data/demo/routing-explorer-repository-impl';
import {
  GetRoutingExplorerDemoUseCase,
  ROUTING_EXPLORER_DEMO_USE_CASE,
} from '../domain/demo/get-routing-explorer-demo-usecase';
import { MiddlewarePipelineRepositoryImpl } from '../data/demo/middleware-pipeline-repository-impl';
import {
  GetMiddlewarePipelineDemoUseCase,
  MIDDLEWARE_PIPELINE_DEMO_USE_CASE,
} from '../domain/demo/get-middleware-pipeline-demo-usecase';
import { ReqresInspectorRepositoryImpl } from '../data/demo/reqres-inspector-repository-impl';
import {
  GetReqresInspectorDemoUseCase,
  REQRES_INSPECTOR_DEMO_USE_CASE,
} from '../domain/demo/get-reqres-inspector-demo-usecase';
import { ReqresMethodsRepositoryImpl } from '../data/demo/reqres-methods-repository-impl';
import {
  GetReqresMethodsDemoUseCase,
  REQRES_METHODS_DEMO_USE_CASE,
} from '../domain/demo/get-reqres-methods-demo-usecase';
import { ValidationArenaRepositoryImpl } from '../data/demo/validation-arena-repository-impl';
import {
  GetValidationArenaDemoUseCase,
  VALIDATION_ARENA_DEMO_USE_CASE,
} from '../domain/demo/get-validation-arena-demo-usecase';
import { PostsRepositoryImpl } from '../data/posts/posts-repository-impl';
import { ListPostsUseCase } from '../domain/posts/list-posts-usecase';
import { CreatePostUseCase } from '../domain/posts/create-post-usecase';
import { ReplacePostUseCase } from '../domain/posts/replace-post-usecase';
import { DeletePostUseCase } from '../domain/posts/delete-post-usecase';
import { PostsWorkbenchRepositoryImpl } from '../data/demo/posts-workbench-repository-impl';
import {
  GetPostsWorkbenchDemoUseCase,
  POSTS_WORKBENCH_DEMO_USE_CASE,
} from '../domain/demo/get-posts-workbench-demo-usecase';
import { WebhookSimulatorRepositoryImpl } from '../data/demo/webhook-simulator-repository-impl';
import {
  GetWebhookSimulatorDemoUseCase,
  WEBHOOK_SIMULATOR_DEMO_USE_CASE,
} from '../domain/demo/get-webhook-simulator-demo-usecase';
import { DocsLinkRepositoryImpl } from '../data/demo/docs-link-repository-impl';
import {
  GetDocsLinkDemoUseCase,
  API_DOCS_LINK_DEMO_USE_CASE,
} from '../domain/demo/get-docs-link-demo-usecase';
import { GraphQLPlaygroundRepositoryImpl } from '../data/demo/graphql-playground-repository-impl';
import {
  GetGraphQLPlaygroundDemoUseCase,
  API_GRAPHQL_PLAYGROUND_DEMO_USE_CASE,
} from '../domain/demo/get-graphql-playground-demo-usecase';
import {
  ExecuteGraphQLDemoQueryUseCase,
  API_GRAPHQL_DEMO_QUERY_USE_CASE,
} from '../domain/demo/execute-graphql-demo-query-usecase';
import { GRPCConsoleRepositoryImpl } from '../data/demo/grpc-console-repository-impl';
import {
  GetGRPCConsoleDemoUseCase,
  API_GRPC_CONSOLE_DEMO_USE_CASE,
} from '../domain/demo/get-grpc-console-demo-usecase';
import { SchemaExplorerRepositoryImpl } from '../data/demo/schema-explorer-repository-impl';
import {
  GetSchemaExplorerDemoUseCase,
  SCHEMA_EXPLORER_DEMO_USE_CASE,
} from '../domain/demo/get-schema-explorer-demo-usecase';
import { UUIDLabRepositoryImpl } from '../data/demo/uuid-lab-repository-impl';
import { SQLSafetyRepositoryImpl } from '../data/demo/sql-safety-repository-impl';
import { MigrationsRepositoryImpl } from '../data/demo/db-migrations-repository-impl';
import { SQLLabRepositoryImpl } from '../data/demo/sql-lab-repository-impl';
import { SearchRepositoryImpl } from '../data/demo/db-search-repository-impl';
import { RaceRepositoryImpl } from '../data/demo/db-race-repository-impl';
import { AuditRepositoryImpl } from '../data/demo/db-audit-repository-impl';
import { ORMProbeRepositoryImpl } from '../data/demo/orm-probe-repository-impl';
import {
  GetORMProbeDemoUseCase,
  DB_ORM_PROBE_DEMO_USE_CASE,
} from '../domain/demo/get-orm-probe-demo-usecase';
import { MongoConsoleRepositoryImpl } from '../data/demo/mongo-console-repository-impl';
import {
  GetMongoConsoleDemoUseCase,
  MONGO_CONSOLE_DEMO_USE_CASE,
} from '../domain/demo/get-mongo-console-demo-usecase';
import { RedisPlaygroundRepositoryImpl } from '../data/demo/redis-playground-repository-impl';
import {
  GetRedisPlaygroundDemoUseCase,
  REDIS_PLAYGROUND_DEMO_USE_CASE,
} from '../domain/demo/get-redis-playground-demo-usecase';
import {
  GetUUIDLabDemoUseCase,
  DB_UUID_DEMO_USE_CASE,
} from '../domain/demo/get-uuid-lab-demo-usecase';
import {
  GetSQLSafetyDemoUseCase,
  DB_SQL_SAFETY_DEMO_USE_CASE,
} from '../domain/demo/get-sql-safety-demo-usecase';
import {
  GetMigrationsDemoUseCase,
  DB_MIGRATIONS_DEMO_USE_CASE,
} from '../domain/demo/get-db-migrations-demo-usecase';
import {
  GetSQLLabDemoUseCase,
  DB_SQL_LAB_DEMO_USE_CASE,
} from '../domain/demo/get-sql-lab-demo-usecase';
import {
  GetSearchDemoUseCase,
  DB_SEARCH_DEMO_USE_CASE,
} from '../domain/demo/get-db-search-demo-usecase';
import {
  GetRaceDemoUseCase,
  DB_RACE_DEMO_USE_CASE,
} from '../domain/demo/get-db-race-demo-usecase';
import {
  GetAuditDemoUseCase,
  DB_AUDIT_DEMO_USE_CASE,
} from '../domain/demo/get-db-audit-demo-usecase';

/**
 * The default config wires only fetch-based sources, so initializing the
 * container here is side-effect-free — no request leaves the machine until a
 * use case is executed against a stubbed fetch.
 */
beforeAll(() => {
  container.initialize(createAppConfig());
});

describe('container', () => {
  it('wires the health pipeline through the layers: config → api → repository → use case', () => {
    expect(container.getHealthUseCase).toBeInstanceOf(GetHealthUseCase);
    expect(container.healthRepository).toBeInstanceOf(HealthRepositoryImpl);
    expect(container.appConfig.apiBaseUrl).toBe('/api');
  });

  it('wires the content pipeline through the layers: config → api → repository → use cases', () => {
    expect(container.listContentUseCase).toBeInstanceOf(ListContentUseCase);
    expect(container.getTopicDetailUseCase).toBeInstanceOf(GetTopicDetailUseCase);
    expect(container.contentRepository).toBeInstanceOf(ContentRepositoryImpl);
  });

  it('wires the scenario pipeline through the layers: config → api → repository → use cases', () => {
    expect(container.getScenarioUseCase).toBeInstanceOf(GetScenarioUseCase);
    expect(container.patchScenarioUseCase).toBeInstanceOf(PatchScenarioUseCase);
    expect(container.scenarioRepository).toBeInstanceOf(ScenarioRepositoryImpl);
  });

  it('wires the runtime-types demo pipeline through the layers: config → api → repository → use case', () => {
    expect(container.getRuntimeTypesDemoUseCase).toBeInstanceOf(GetRuntimeTypesDemoUseCase);
    expect(container.runtimeTypesRepository).toBeInstanceOf(RuntimeTypesRepositoryImpl);
    expect(container.appConfig.apiBaseUrl).toBe('/api');
  });

  it('wires the concurrency demo pipeline through the layers: config → api → repository → use case', () => {
    expect(container.getConcurrencyDemoUseCase).toBeInstanceOf(GetConcurrencyDemoUseCase);
    expect(container.concurrencyRepository).toBeInstanceOf(ConcurrencyRepositoryImpl);
    expect(container.appConfig.apiBaseUrl).toBe('/api');
  });

  it('wires the http-headers demo pipeline through the layers: config → api → repository → use case', () => {
    expect(container.getHttpHeadersDemoUseCase).toBeInstanceOf(GetHttpHeadersDemoUseCase);
    expect(container.httpHeadersRepository).toBeInstanceOf(HttpHeadersRepositoryImpl);
    expect(container.appConfig.apiBaseUrl).toBe('/api');
    // The demo view resolves the domain module's own token — the container
    // token is an alias of it, so both resolve the same registration.
    expect(container.resolve(HTTP_HEADERS_DEMO_USE_CASE)).toBe(container.getHttpHeadersDemoUseCase);
  });

  it('wires the app-info demo pipeline through the layers: config → api → repository → use case', () => {
    expect(container.getAppInfoDemoUseCase).toBeInstanceOf(GetAppInfoDemoUseCase);
    expect(container.appInfoRepository).toBeInstanceOf(AppInfoRepositoryImpl);
    expect(container.appConfig.apiBaseUrl).toBe('/api');
    expect(container.resolve(GetAppInfoDemoUseCaseToken)).toBe(container.getAppInfoDemoUseCase);
  });

  it('wires the interface-matrix demo pipeline through the layers: config → api → repository → use case', () => {
    expect(container.getInterfaceMatrixDemoUseCase).toBeInstanceOf(GetInterfaceMatrixDemoUseCase);
    expect(container.interfaceMatrixRepository).toBeInstanceOf(InterfaceMatrixRepositoryImpl);
    expect(container.appConfig.apiBaseUrl).toBe('/api');
    expect(container.resolve(INTERFACE_MATRIX_DEMO_USE_CASE)).toBe(
      container.getInterfaceMatrixDemoUseCase,
    );
  });

  it('wires the context-cancellation demo pipeline through the layers: config → api → repository → use case', () => {
    expect(container.getContextCancellationDemoUseCase).toBeInstanceOf(
      GetContextCancellationDemoUseCase,
    );
    expect(container.contextCancellationRepository).toBeInstanceOf(
      ContextCancellationRepositoryImpl,
    );
    expect(container.appConfig.apiBaseUrl).toBe('/api');
  });

  it('wires the race-detector demo pipeline through the layers: config → api → repository → use case', () => {
    expect(container.getRaceDetectorDemoUseCase).toBeInstanceOf(GetRaceDetectorDemoUseCase);
    expect(container.raceDetectorRepository).toBeInstanceOf(RaceDetectorRepositoryImpl);
    expect(container.appConfig.apiBaseUrl).toBe('/api');
  });

  it('wires the routing-explorer demo pipeline through the layers: config → api → repository → use case', () => {
    expect(container.getRoutingExplorerDemoUseCase).toBeInstanceOf(GetRoutingExplorerDemoUseCase);
    expect(container.routingExplorerRepository).toBeInstanceOf(RoutingExplorerRepositoryImpl);
    expect(container.appConfig.apiBaseUrl).toBe('/api');
    expect(container.resolve(ROUTING_EXPLORER_DEMO_USE_CASE)).toBe(
      container.getRoutingExplorerDemoUseCase,
    );
  });

  it('wires the middleware-pipeline demo pipeline through the layers: config → api → repository → use case', () => {
    expect(container.getMiddlewarePipelineDemoUseCase).toBeInstanceOf(
      GetMiddlewarePipelineDemoUseCase,
    );
    expect(container.middlewarePipelineRepository).toBeInstanceOf(MiddlewarePipelineRepositoryImpl);
    expect(container.appConfig.apiBaseUrl).toBe('/api');
    expect(container.resolve(MIDDLEWARE_PIPELINE_DEMO_USE_CASE)).toBe(
      container.getMiddlewarePipelineDemoUseCase,
    );
  });

  it('wires the reqres-inspector demo pipeline through the layers: config → api → repository → use case', () => {
    expect(container.getReqresInspectorDemoUseCase).toBeInstanceOf(GetReqresInspectorDemoUseCase);
    expect(container.reqresInspectorRepository).toBeInstanceOf(ReqresInspectorRepositoryImpl);
    expect(container.appConfig.apiBaseUrl).toBe('/api');
    expect(container.resolve(REQRES_INSPECTOR_DEMO_USE_CASE)).toBe(
      container.getReqresInspectorDemoUseCase,
    );
  });

  it('wires the reqres-methods demo pipeline through the layers: config → api → repository → use case', () => {
    expect(container.getReqresMethodsDemoUseCase).toBeInstanceOf(GetReqresMethodsDemoUseCase);
    expect(container.reqresMethodsRepository).toBeInstanceOf(ReqresMethodsRepositoryImpl);
    expect(container.appConfig.apiBaseUrl).toBe('/api');
    expect(container.resolve(REQRES_METHODS_DEMO_USE_CASE)).toBe(
      container.getReqresMethodsDemoUseCase,
    );
  });

  it('wires the validation-arena demo pipeline through the layers: config → api → repository → use case', () => {
    expect(container.getValidationArenaDemoUseCase).toBeInstanceOf(GetValidationArenaDemoUseCase);
    expect(container.validationArenaRepository).toBeInstanceOf(ValidationArenaRepositoryImpl);
    expect(container.appConfig.apiBaseUrl).toBe('/api');
    expect(container.resolve(VALIDATION_ARENA_DEMO_USE_CASE)).toBe(
      container.getValidationArenaDemoUseCase,
    );
  });

  it('wires the posts resource pipeline: one api + repository behind four verb use cases', () => {
    expect(container.listPostsUseCase).toBeInstanceOf(ListPostsUseCase);
    expect(container.createPostUseCase).toBeInstanceOf(CreatePostUseCase);
    expect(container.replacePostUseCase).toBeInstanceOf(ReplacePostUseCase);
    expect(container.deletePostUseCase).toBeInstanceOf(DeletePostUseCase);
    expect(container.postsRepository).toBeInstanceOf(PostsRepositoryImpl);
    expect(container.appConfig.apiBaseUrl).toBe('/api');
    // All four verbs share the ONE repository instance — the mirror of the
    // server's composition root (one postsRepo behind five use cases).
    const wiring = container.getWiring();
    expect(wiring.get(ContainerTokens.CreatePostsUseCase)).toBe(container.createPostUseCase);
    expect(wiring.get(ContainerTokens.ListPostsUseCase)).toBe(container.listPostsUseCase);
  });

  it('wires the posts-workbench demo pipeline through the layers: config → api → repository → use case', () => {
    expect(container.getPostsWorkbenchDemoUseCase).toBeInstanceOf(GetPostsWorkbenchDemoUseCase);
    expect(container.postsWorkbenchRepository).toBeInstanceOf(PostsWorkbenchRepositoryImpl);
    expect(container.appConfig.apiBaseUrl).toBe('/api');
    expect(container.resolve(POSTS_WORKBENCH_DEMO_USE_CASE)).toBe(
      container.getPostsWorkbenchDemoUseCase,
    );
  });

  it('wires the webhook-simulator demo pipeline through the layers: config → api → repository → use case', () => {
    expect(container.getWebhookSimulatorDemoUseCase).toBeInstanceOf(GetWebhookSimulatorDemoUseCase);
    expect(container.webhookSimulatorRepository).toBeInstanceOf(WebhookSimulatorRepositoryImpl);
    expect(container.appConfig.apiBaseUrl).toBe('/api');
    expect(container.resolve(WEBHOOK_SIMULATOR_DEMO_USE_CASE)).toBe(
      container.getWebhookSimulatorDemoUseCase,
    );
  });

  it('wires the docs-link demo pipeline through the layers: config → api → repository → use case', () => {
    expect(container.getDocsLinkDemoUseCase).toBeInstanceOf(GetDocsLinkDemoUseCase);
    expect(container.docsLinkRepository).toBeInstanceOf(DocsLinkRepositoryImpl);
    expect(container.appConfig.apiBaseUrl).toBe('/api');
    expect(container.resolve(API_DOCS_LINK_DEMO_USE_CASE)).toBe(container.getDocsLinkDemoUseCase);
  });

  it('wires the graphql playground: one api + one repository shared by its TWO use cases', () => {
    expect(container.getGraphQLPlaygroundDemoUseCase).toBeInstanceOf(GetGraphQLPlaygroundDemoUseCase);
    expect(container.executeGraphQLDemoQueryUseCase).toBeInstanceOf(ExecuteGraphQLDemoQueryUseCase);
    expect(container.graphqlPlaygroundRepository).toBeInstanceOf(GraphQLPlaygroundRepositoryImpl);
    expect(container.appConfig.apiBaseUrl).toBe('/api');
    expect(container.resolve(API_GRAPHQL_PLAYGROUND_DEMO_USE_CASE)).toBe(
      container.getGraphQLPlaygroundDemoUseCase,
    );
    expect(container.resolve(API_GRAPHQL_DEMO_QUERY_USE_CASE)).toBe(
      container.executeGraphQLDemoQueryUseCase,
    );
  });

  it('wires the grpc-console demo pipeline through the layers: config → api → repository → use case', () => {
    expect(container.getGRPCConsoleDemoUseCase).toBeInstanceOf(GetGRPCConsoleDemoUseCase);
    expect(container.grpcConsoleRepository).toBeInstanceOf(GRPCConsoleRepositoryImpl);
    expect(container.appConfig.apiBaseUrl).toBe('/api');
    expect(container.resolve(API_GRPC_CONSOLE_DEMO_USE_CASE)).toBe(
      container.getGRPCConsoleDemoUseCase,
    );
  });

  it('wires the schema-explorer demo pipeline through the layers: config → api → repository → use case', () => {
    expect(container.getSchemaExplorerDemoUseCase).toBeInstanceOf(GetSchemaExplorerDemoUseCase);
    expect(container.schemaExplorerRepository).toBeInstanceOf(SchemaExplorerRepositoryImpl);
    expect(container.appConfig.apiBaseUrl).toBe('/api');
    expect(container.resolve(SCHEMA_EXPLORER_DEMO_USE_CASE)).toBe(
      container.getSchemaExplorerDemoUseCase,
    );
  });

  it('wires the uuid-lab demo pipeline through the layers: config → api → repository → use case', () => {
    expect(container.getUUIDLabDemoUseCase).toBeInstanceOf(GetUUIDLabDemoUseCase);
    expect(container.uuidLabRepository).toBeInstanceOf(UUIDLabRepositoryImpl);
    expect(container.appConfig.apiBaseUrl).toBe('/api');
    expect(container.resolve(DB_UUID_DEMO_USE_CASE)).toBe(container.getUUIDLabDemoUseCase);
  });

  it('wires the databases-sql demo sextet through the layers', () => {
    const cases = [
      {
        useCase: container.getSQLSafetyDemoUseCase,
        klass: GetSQLSafetyDemoUseCase,
        repository: container.sqlSafetyRepository,
        repositoryClass: SQLSafetyRepositoryImpl,
        token: DB_SQL_SAFETY_DEMO_USE_CASE,
      },
      {
        useCase: container.getMigrationsDemoUseCase,
        klass: GetMigrationsDemoUseCase,
        repository: container.migrationsRepository,
        repositoryClass: MigrationsRepositoryImpl,
        token: DB_MIGRATIONS_DEMO_USE_CASE,
      },
      {
        useCase: container.getSQLLabDemoUseCase,
        klass: GetSQLLabDemoUseCase,
        repository: container.sqlLabRepository,
        repositoryClass: SQLLabRepositoryImpl,
        token: DB_SQL_LAB_DEMO_USE_CASE,
      },
      {
        useCase: container.getSearchDemoUseCase,
        klass: GetSearchDemoUseCase,
        repository: container.searchRepository,
        repositoryClass: SearchRepositoryImpl,
        token: DB_SEARCH_DEMO_USE_CASE,
      },
      {
        useCase: container.getRaceDemoUseCase,
        klass: GetRaceDemoUseCase,
        repository: container.raceRepository,
        repositoryClass: RaceRepositoryImpl,
        token: DB_RACE_DEMO_USE_CASE,
      },
      {
        useCase: container.getAuditDemoUseCase,
        klass: GetAuditDemoUseCase,
        repository: container.auditRepository,
        repositoryClass: AuditRepositoryImpl,
        token: DB_AUDIT_DEMO_USE_CASE,
      },
      {
        useCase: container.getORMProbeDemoUseCase,
        klass: GetORMProbeDemoUseCase,
        repository: container.ormProbeRepository,
        repositoryClass: ORMProbeRepositoryImpl,
        token: DB_ORM_PROBE_DEMO_USE_CASE,
      },
      {
        useCase: container.getMongoConsoleDemoUseCase,
        klass: GetMongoConsoleDemoUseCase,
        repository: container.mongoConsoleRepository,
        repositoryClass: MongoConsoleRepositoryImpl,
        token: MONGO_CONSOLE_DEMO_USE_CASE,
      },
      {
        useCase: container.getRedisPlaygroundDemoUseCase,
        klass: GetRedisPlaygroundDemoUseCase,
        repository: container.redisPlaygroundRepository,
        repositoryClass: RedisPlaygroundRepositoryImpl,
        token: REDIS_PLAYGROUND_DEMO_USE_CASE,
      },
    ];
    for (const pipeline of cases) {
      expect(pipeline.useCase).toBeInstanceOf(pipeline.klass);
      expect(pipeline.repository).toBeInstanceOf(pipeline.repositoryClass);
      expect(container.resolve(pipeline.token)).toBe(pipeline.useCase);
    }
    expect(container.appConfig.apiBaseUrl).toBe('/api');
  });

  it('getWiring exposes every registered singleton keyed by symbol token', () => {
    const wiring = container.getWiring();

    expect(wiring.get(ContainerTokens.GetHealthUseCase)).toBeInstanceOf(
      GetHealthUseCase,
    );
    expect(wiring.get(ContainerTokens.HealthRepository)).toBeInstanceOf(
      HealthRepositoryImpl,
    );
    expect(wiring.size).toBe(Object.keys(ContainerTokens).length);
  });

  it('resolve throws loudly for an unregistered token', () => {
    expect(() => container.resolve(Symbol('never registered'))).toThrow(
      /is not registered/,
    );
  });

  it('re-initialization is idempotent; dispose clears the registry', () => {
    const before = container.getHealthUseCase;
    container.initialize(createAppConfig());
    expect(container.getHealthUseCase).toBeInstanceOf(GetHealthUseCase);
    expect(container.getHealthUseCase).not.toBe(before);

    container.dispose();
    expect(() => container.healthRepository).toThrow(/is not registered/);
    // re-initialize so downstream suites / teardown stay valid
    container.initialize(createAppConfig());
  });
});
