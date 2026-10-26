import { describe, expect, it } from 'vitest';
import { failed, success, type Result } from '../../core/result';
import { NetworkFailure } from '../../core/error-handling/failures';
import type { RaceDetectorDemoData } from './race-detector';
import type { RaceDetectorRepository } from './race-detector-repository';
import { GetRaceDetectorDemoUseCase } from './get-race-detector-demo-usecase';

/**
 * GetRaceDetectorDemoUseCase spec — the use case is a thin pass-through
 * seam, so the contract to pin is exactly that: whatever the repository
 * resolves (Success or Failed) arrives unchanged, and nothing throws.
 */

/** In-memory repository stand-in — the port's contract is all the use case sees. */
function fakeRepository(get: Result<RaceDetectorDemoData>): RaceDetectorRepository {
  return {
    get: () => Promise.resolve(get),
  };
}

const data: RaceDetectorDemoData = {
  race: {
    code: 'func RacyTotal(goroutines, increments int) int {\n\tcounter := 0\n}',
    command: 'go test -race -count=1 .',
    verdict: '==================\nWARNING: DATA RACE\n',
    explanation: 'the race detector instruments loads and stores',
    fix: 'synchronize: mutex, or ownership over a channel',
  },
  mutex: {
    strategy: 'sync.Mutex — every increment a critical section',
    goroutines: 200,
    incrementsEach: 25,
    expected: 5000,
    observed: 5000,
    exact: true,
  },
  channel: {
    strategy: 'channel fold — local tallies, one owner folds after the join',
    goroutines: 200,
    incrementsEach: 25,
    expected: 5000,
    observed: 5000,
    exact: true,
  },
  hygiene: [
    { case: 'unbuffered send with a waiting reader', readerPresent: true, outcome: 'delivered' },
    { case: 'guarded send after the reader is gone', readerPresent: false, outcome: 'abandoned' },
  ],
};

describe('GetRaceDetectorDemoUseCase', () => {
  it('returns the payload the repository resolved, unchanged', async () => {
    const useCase = new GetRaceDetectorDemoUseCase(fakeRepository(success(data)));

    const result = await useCase.execute();

    expect(result).toEqual(success(data));
  });

  it('returns a Failed Result when the payload cannot be loaded (never throws)', async () => {
    const useCase = new GetRaceDetectorDemoUseCase(
      fakeRepository(failed(new NetworkFailure('Could not reach the server.'))),
    );

    const result = await useCase.execute();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });
});
