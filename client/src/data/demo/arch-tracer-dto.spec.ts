import { describe, expect, it } from 'vitest';
import { archTracerDtoFromJson } from './arch-tracer-dto';

/**
 * ArchTracerDTO spec — the fromJson whitelist: a full payload builds
 * the domain shape; every violation (wrong layer vocabulary, wrong
 * dataMode, missing panels) throws so parseDto can upgrade it to an
 * InvalidResponseException.
 */

const fullPayload = {
  trace: {
    request: 'GET /api/posts?limit=3&sort=newest',
    dataMode: 'fake',
    leaf: 'InMemoryRepository.List — server/internal/data/posts/inmemory.go',
    flipNote: 'PATCH /api/demo/scenario {"dataMode":"live"} — reload for the other leaf',
    hops: [
      {
        order: 1,
        layer: 'middleware',
        file: 'server/internal/presentation/middleware.go',
        symbol: 'Chain(RequestID → RequestLogger → Recover → EnvelopeMuxErrors)',
        note: 'the documented order',
        durationMs: 0.35,
      },
      {
        order: 4,
        layer: 'data',
        file: 'server/internal/data/posts/inmemory.go',
        symbol: 'InMemoryRepository.List',
        note: 'the leaf that answered',
        durationMs: 0.11,
      },
    ],
    status: 200,
    items: 3,
    elapsedMs: 0.36,
    requestId: 'rid-arch-1',
    note: 'every hop is an execution that just happened',
  },
  violation: {
    label: 'counterexample — not code this repo compiles',
    snippet: 'func (h *BadPostsHandler) ServeHTTP…',
    why: 'the handler owns four jobs the architecture gives to four places',
    caughtBy: 'the import graph: one grep away',
    honest: 'no production presentation file imports the data layer',
  },
  reasoning: 'one request, four layers, every hop measured',
};

describe('archTracerDtoFromJson', () => {
  it('builds the domain payload from a valid wire body', () => {
    const dto = archTracerDtoFromJson(fullPayload);

    expect(dto.trace.request).toBe('GET /api/posts?limit=3&sort=newest');
    expect(dto.trace.dataMode).toBe('fake');
    expect(dto.trace.hops).toHaveLength(2);
    expect(dto.trace.hops[0]?.layer).toBe('middleware');
    expect(dto.trace.hops[1]?.durationMs).toBe(0.11);
    expect(dto.violation.label).toContain('counterexample');
    expect(dto.reasoning).toContain('measured');
  });

  it('rejects a hop whose layer is outside the closed vocabulary', () => {
    const broken = {
      ...fullPayload,
      trace: {
        ...fullPayload.trace,
        hops: [{ ...fullPayload.trace.hops[0], layer: 'controller' }],
      },
    };

    expect(() => archTracerDtoFromJson(broken)).toThrow(/unknown arch trace layer/);
  });

  it('rejects a dataMode outside fake|live', () => {
    const broken = {
      ...fullPayload,
      trace: { ...fullPayload.trace, dataMode: 'cached' },
    };

    expect(() => archTracerDtoFromJson(broken)).toThrow(/unknown arch trace dataMode/);
  });

  it('rejects a payload missing the violation panel', () => {
    const { violation: _violation, ...withoutViolation } = fullPayload;

    expect(() => archTracerDtoFromJson(withoutViolation)).toThrow();
  });
});
