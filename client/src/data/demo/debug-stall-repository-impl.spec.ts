import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { DebugStallApi } from './debug-stall-api';
import { DebugStallRepositoryImpl } from './debug-stall-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * DebugStallRepositoryImpl spec — fixture-driven (happy + failure):
 * the stall payload (workload, capture, stalled/contrast stacks,
 * endpoints) round-trips; malformed payloads and transport failures
 * resolve as Failed Results, never throws; and the server's typed 500
 * (a lost wedge — no stack invented) guards into a ServerFailure
 * carrying the envelope's code.
 */

const demoPayload = {
  mode: 'runtime-derived: the stalled stack below was captured from THIS process',
  workload: { workers: 5, healthyDone: 4, wedge: 'one worker blocks on a channel receive' },
  capture: {
    profile: 'goroutine',
    goroutinesSeen: 9,
    captureMs: 0.31,
    captureTries: 1,
    curl: "curl -s 'localhost:8100/internal/debug/pprof/goroutine?debug=2'",
  },
  stalled: {
    id: 42,
    state: 'chan receive',
    wait: '1 minutes',
    frames: [
      {
        function: 'guide/internal/domain/demos.debugStallWedge(0x0)',
        location: 'server/internal/domain/demos/debug_stall.go:118',
      },
    ],
    createdBy: 'guide/internal/domain/demos.RunDebugStall',
    createdAt: 'server/internal/domain/demos/debug_stall.go:157',
  },
  contrast: {
    id: 1,
    state: 'running',
    frames: [
      {
        function: 'guide/internal/domain/demos.RunDebugStall(0x0)',
        location: 'server/internal/domain/demos/debug_stall.go:201',
      },
    ],
  },
  endpoints: [
    { path: '/internal/debug/pprof/goroutine?debug=2', profile: 'goroutine', note: 'the census' },
  ],
  reasoning: 'the runtime names the wedge',
};

function makeRepository(): DebugStallRepositoryImpl {
  return new DebugStallRepositoryImpl(new DebugStallApi(createAppConfig()));
}

describe('DebugStallRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/debug/stall', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.stalled.state).toBe('chan receive');
      expect(result.value.stalled.frames[0]?.location).toContain('debug_stall.go:');
      expect(result.value.contrast.id).not.toBe(result.value.stalled.id);
      expect(result.value.endpoints[0]?.path).toContain('/internal/debug/pprof/');
    }
  });

  it('guards a malformed payload into a Failed Result', async () => {
    stubApiFetch([{ url: '/api/demo/debug/stall', body: { mode: 'half a payload' } }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
    }
  });

  it('guards a network failure into a Failed Result (network_error, status 0)', async () => {
    stubNetworkFailure();

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
    }
  });

  it('guards the server typed 500 (lost wedge) into a Failed Result', async () => {
    stubApiFetch([
      {
        url: '/api/demo/debug/stall',
        status: 500,
        body: {
          error: {
            code: 'internal',
            message:
              "The stall demo could not find its wedged goroutine on the process's own profile; no stack was invented to cover for it.",
            requestId: 'rid-stall-lost',
          },
        },
      },
    ]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('internal');
      expect(result.failure.message).toContain('no stack was invented');
    }
  });
});
