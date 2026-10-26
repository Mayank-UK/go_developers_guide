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
