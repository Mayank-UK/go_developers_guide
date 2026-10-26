import { afterEach, describe, expect, it, vi } from 'vitest';
import { ServerFailure } from '../../core/error-handling/failures';
import { failed } from '../../core/result';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { RaceDetectorApi } from './race-detector-api';
import { RaceDetectorRepositoryImpl } from './race-detector-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * RaceDetectorRepositoryImpl spec — fixture-driven (happy + failure): the
 * happy payload (mirroring the REAL builder output, exhibit strings
 * trimmed) round-trips into the domain shape; network / envelope /
 * malformed-payload failures all resolve as Failed Results, never throws.
 */

const demoPayload = {
  race: {
    code: 'func RacyTotal(goroutines, increments int) int {\n\tcounter := 0\n',
    command: 'go test -race -count=1 .',
    verdict: '==================\nWARNING: DATA RACE\n--- FAIL: TestRacyTotal (0.00s)\nFAIL',
    explanation: 'the race detector instruments loads and stores',
    fix: 'synchronize: guard the shared variable with a mutex',
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
    { case: 'unbuffered send with a waiting reader', readerPresent: true, outcome: 'delivered "done" — goroutine returned' },
    {
      case: 'unbuffered send after the reader is gone, guarded by select+default',
      readerPresent: false,
      outcome: 'send abandoned by the default arm (a plain send would leak)',
    },
  ],
};

function makeRepository(): RaceDetectorRepositoryImpl {
  return new RaceDetectorRepositoryImpl(new RaceDetectorApi(createAppConfig()));
}

describe('RaceDetectorRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/conc/race', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.race.command).toBe('go test -race -count=1 .');
      expect(result.value.race.verdict).toContain('WARNING: DATA RACE');
      expect(result.value.mutex.observed).toBe(5000);
      expect(result.value.channel.exact).toBe(true);
      expect(result.value.hygiene[1]?.readerPresent).toBe(false);
    }
  });

  it('guards a network failure into a Failed Result (network_error, status 0)', async () => {
    stubNetworkFailure();

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });

  it('guards an error envelope into a Failed Result with its diagnostics', async () => {
    stubApiFetch([
      {
        url: '/api/demo/conc/race',
        status: 503,
        body: { error: { code: 'service_unavailable', message: 'calm down', requestId: 'req-9' } },
      },
    ]);

    const result = await makeRepository().get();

    expect(result).toEqual(
      failed(
        new ServerFailure('calm down', {
          code: 'service_unavailable',
          status: 503,
          requestId: 'req-9',
        }),
      ),
    );
  });

  it('guards a malformed payload into a Failed Result (invalid_response)', async () => {
    stubApiFetch([
      // Missing every beat — the DTO validator must reject it.
      { url: '/api/demo/conc/race', body: { hygiene: 'nope' } },
    ]);

    const result = await makeRepository().get();

    expect(result).toEqual(
      failed(
        new ServerFailure('The server response did not match the expected shape.', {
          code: 'invalid_response',
          status: 200,
        }),
      ),
    );
  });

  it('rejects a missing exact flag instead of letting it through', async () => {
    stubApiFetch([
      {
        url: '/api/demo/conc/race',
        body: { ...demoPayload, mutex: { ...demoPayload.mutex, exact: undefined } },
      },
    ]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
    }
  });
});
