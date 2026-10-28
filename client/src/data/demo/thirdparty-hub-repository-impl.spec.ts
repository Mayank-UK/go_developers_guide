import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubNetworkFailure } from '../../test/fetch_stub';
import { ThirdPartyHubApi } from './thirdparty-hub-api';
import { ThirdPartyHubRepositoryImpl } from './thirdparty-hub-repository-impl';
import { thirdPartyHubDtoFromJson, type ThirdPartyHubDemoDto } from './thirdparty-hub-dto';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * ThirdPartyHubRepositoryImpl spec — the adapter contract: a resolved
 * DTO maps to a success Result carrying the domain model, and every
 * rejection (network, non-2xx, malformed payload) is guarded into a
 * failed Result instead of escaping as an unhandled rejection.
 */

const dto: ThirdPartyHubDemoDto = thirdPartyHubDtoFromJson({
  mode: 'adapter-surface hub',
  honestyNote: 'no-invention pin',
  wiring: {
    source: 'GET/PATCH /api/demo/scenario — the dataMode field',
    current: 'fake',
    getEndpoint: '/api/demo/scenario',
    patchEndpoint: '/api/demo/scenario',
    rows: [],
    note: '',
  },
  vendors: [],
  taxonomyMap: [],
});

describe('ThirdPartyHubRepositoryImpl', () => {
  it('maps a resolved DTO into a success Result', async () => {
    const api = { get: vi.fn().mockResolvedValue(dto) } as unknown as ThirdPartyHubApi;
    const repository = new ThirdPartyHubRepositoryImpl(api);

    const result = await repository.get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.wiring.current).toBe('fake');
      expect(result.value.mode).toBe('adapter-surface hub');
    }
  });

  it('guards a transport failure into a failed Result', async () => {
    stubNetworkFailure();

    const repository = new ThirdPartyHubRepositoryImpl(new ThirdPartyHubApi(createAppConfig()));

    const result = await repository.get();

    expect(result.ok).toBe(false);
  });
});
