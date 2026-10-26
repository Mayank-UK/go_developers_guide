import { describe, expect, it } from 'vitest';
import { failed, success } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import { GetRaceDemoUseCase } from './get-db-race-demo-usecase';
import type { RaceRepository } from './db-race-repository';
import type { RaceDemoData } from './db-race';

const fixture: RaceDemoData = {
  isolation: 'read committed',
  scratchBefore: 'race demo scratch',
  scratchAfter: 'race demo scratch (updated)',
  steps: [],
  lockWaited: true,
  serializableNote: 'Under SERIALIZABLE …',
};

describe('GetRaceDemoUseCase', () => {
  it('returns the repository payload on success', async () => {
    const useCase = new GetRaceDemoUseCase({
      get: () => Promise.resolve(success(fixture)),
    } as RaceRepository);
    const result = await useCase.execute();
    expect(result.ok && result.value.isolation).toBe('read committed');
  });

  it('passes failures through untouched', async () => {
    const useCase = new GetRaceDemoUseCase({
      get: () => Promise.resolve(failed(new NetworkFailure('down'))),
    } as RaceRepository);
    expect((await useCase.execute()).ok).toBe(false);
  });
});
