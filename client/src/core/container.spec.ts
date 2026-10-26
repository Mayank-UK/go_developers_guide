import { beforeAll, describe, expect, it } from 'vitest';
import { container, ContainerTokens } from './container';
import { createAppConfig } from './config/app-config';
import { ContentRepositoryImpl } from '../data/content/content-repository-impl';
import { HealthRepositoryImpl } from '../data/health/health-repository-impl';
import { GetTopicDetailUseCase } from '../domain/content/get-topic-detail-usecase';
import { ListContentUseCase } from '../domain/content/list-content-usecase';
import { GetHealthUseCase } from '../domain/health/get-health-usecase';

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
