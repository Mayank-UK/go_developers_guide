import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubNetworkFailure } from '../../test/fetch_stub';
import { ArchEventstoreApi } from './arch-eventstore-api';
import { ArchEventstoreRepositoryImpl } from './arch-eventstore-repository-impl';
import { archEventstoreDtoFromJson, type ArchEventstoreDemoDto } from './arch-eventstore-dto';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * ArchEventstoreRepositoryImpl spec — the adapter contract: a resolved
 * DTO maps to a success Result carrying the domain model, and every
 * rejection (network, non-2xx, malformed payload) is guarded into a
 * failed Result instead of escaping as an unhandled rejection.
 */

const dto: ArchEventstoreDemoDto = archEventstoreDtoFromJson({
  mode: 'event store lifecycle',
  honestyNote: 'no-invention pin',
  streamId: 'inventory-tee-medium',
  store: [],
  panels: [],
  replay: {
    foldedEvents: 0,
    appliedInOrder: [],
    state: { onHand: 0, reserved: 0, available: 0, reservations: {} },
    naiveExpected: { onHand: 0, reserved: 0, available: 0, reservations: {} },
    rebuildsRun: 0,
    byteIdentical: true,
    absentStreamId: 'inventory-never-stocked',
    absentState: { onHand: 0, reserved: 0, available: 0, reservations: {} },
    absentVersion: 0,
    note: '',
  },
});

describe('ArchEventstoreRepositoryImpl', () => {
  it('maps a resolved DTO into a success Result', async () => {
    const api = { get: vi.fn().mockResolvedValue(dto) } as unknown as ArchEventstoreApi;
    const repository = new ArchEventstoreRepositoryImpl(api);

    const result = await repository.get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.streamId).toBe('inventory-tee-medium');
      expect(result.value.mode).toBe('event store lifecycle');
    }
  });

  it('guards a transport failure into a failed Result', async () => {
    stubNetworkFailure();

    const repository = new ArchEventstoreRepositoryImpl(new ArchEventstoreApi(createAppConfig()));

    const result = await repository.get();

    expect(result.ok).toBe(false);
  });
});
