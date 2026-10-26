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
import { ContextCancellationRepositoryImpl } from '../data/demo/context-cancellation-repository-impl';
import { GetContextCancellationDemoUseCase } from '../domain/demo/get-context-cancellation-demo-usecase';
import { RaceDetectorRepositoryImpl } from '../data/demo/race-detector-repository-impl';
import { GetRaceDetectorDemoUseCase } from '../domain/demo/get-race-detector-demo-usecase';

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
