import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { ArchTracerApi } from './arch-tracer-api';
import { ArchTracerRepositoryImpl } from './arch-tracer-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * ArchTracerRepositoryImpl spec — fixture-driven (happy + failure): the
 * tracer payload (timeline, leaf, violation panel) round-trips;
 * malformed payloads and transport failures resolve as Failed Results,
 * never throws.
 */

const demoPayload = {
  trace: {
    request: 'GET /api/posts?limit=3&sort=newest',
    dataMode: 'fake',
    leaf: 'InMemoryRepository.List — server/internal/data/posts/inmemory.go',
    flipNote: 'PATCH /api/demo/scenario, then reload',
    hops: [
      {
        order: 1,
        layer: 'middleware',
        file: 'server/internal/presentation/middleware.go',
        symbol: 'Chain(RequestID → RequestLogger → Recover → EnvelopeMuxErrors)',
        note: 'the documented order',
        durationMs: 0.35,
      },
    ],
    status: 200,
    items: 3,
    elapsedMs: 0.36,
    requestId: 'rid-arch-9',
    note: 'measured where it ran',
  },
  violation: {
    label: 'counterexample — not code this repo compiles',
    snippet: 'func (h *BadPostsHandler) ServeHTTP…',
    why: 'four jobs in one handler',
    caughtBy: 'the import graph',
    honest: 'zero production hits today',
  },
  reasoning: 'the dependency rule executed',
};

function makeRepository(): ArchTracerRepositoryImpl {
  return new ArchTracerRepositoryImpl(new ArchTracerApi(createAppConfig()));
}

describe('ArchTracerRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/arch/tracer', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.trace.dataMode).toBe('fake');
      expect(result.value.trace.hops[0]?.symbol).toContain('Chain(RequestID');
      expect(result.value.violation.label).toContain('counterexample');
    }
  });

  it('rejects a payload whose trace panel is missing', async () => {
    stubApiFetch([{ url: '/api/demo/arch/tracer', body: { ...demoPayload, trace: undefined } }]);

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
      expect(result.failure.status).toBe(0);
    }
  });
});
