import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubNetworkFailure } from '../../test/fetch_stub';
import { ArchSagaApi } from './arch-saga-api';
import { ArchSagaRepositoryImpl } from './arch-saga-repository-impl';
import { archSagaDtoFromJson, type ArchSagaDemoDto } from './arch-saga-dto';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * ArchSagaRepositoryImpl spec — the adapter contract: a resolved DTO
 * maps to a success Result carrying the domain model, and every
 * rejection (network, non-2xx, malformed payload) is guarded into a
 * failed Result instead of escaping as an unhandled rejection.
 */

const dto: ArchSagaDemoDto = archSagaDtoFromJson({
  mode: 'compensation ledger',
  source: 'server/internal/domain/patterns/saga.go',
  honestyNote: 'no-invention pin',
  steps: [],
  runs: [],
  semantics: [],
  surface: [],
});

describe('ArchSagaRepositoryImpl', () => {
  it('maps a resolved DTO into a success Result', async () => {
    const api = { get: vi.fn().mockResolvedValue(dto) } as unknown as ArchSagaApi;
    const repository = new ArchSagaRepositoryImpl(api);

    const result = await repository.get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.source).toBe('server/internal/domain/patterns/saga.go');
      expect(result.value.mode).toBe('compensation ledger');
    }
  });

  it('guards a transport failure into a failed Result', async () => {
    stubNetworkFailure();

    const repository = new ArchSagaRepositoryImpl(new ArchSagaApi(createAppConfig()));

    const result = await repository.get();

    expect(result.ok).toBe(false);
  });
});
