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
