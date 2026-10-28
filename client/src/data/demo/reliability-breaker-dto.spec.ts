import { describe, expect, it } from 'vitest';
import { reliabilityBreakerDtoFromJson } from './reliability-breaker-dto';

/**
 * ReliabilityBreakerDTO spec — the fromJson whitelist: a full payload
 * builds the domain shape (the scenario toggle, the persistent breaker
 * with its config/counters/transitions, the walk timeline, the retry
 * series); every violation (missing panels, wrong types, a row
 * without its measured timings) throws so parseDto can upgrade it to
 * an InvalidResponseException.
 */

const fullPayload = {
  mode: 'process-lifetime breaker: one shared circuit breaker protects the dependency',
  scenario: {
    mode: 'error',
    getEndpoint: '/api/demo/scenario',
    patchEndpoint: '/api/demo/scenario',
    modes: ['none', 'error', 'latency'],
    note: 'the runtime toggle the dependency honors on every call',
  },
  breaker: {
    state: 'open',
    stateAgeMs: 1234.5,
    config: {
      failureThreshold: 3,
      cooldownMs: 2000,
      callTimeoutMs: 120,
      walkAttempts: 4,
      halfOpenProbes: 1,
      retryAttempts: 3,
      retryBaseDelayMs: 40,
      retryJitterMs: 20,
    },
    counters: { allowed: 9, fastFailed: 4, succeeded: 5, failed: 4 },
    probeInFlight: false,
    transitions: [
      {
        at: '2026-10-28T09:30:00Z',
        from: 'closed',
        to: 'open',
        cause: '3 consecutive failures — threshold reached',
      },
      {
        at: '2026-10-28T09:30:02Z',
        from: 'open',
        to: 'half-open',
        cause: 'cooldown elapsed — probing the dependency',
      },
    ],
  },
  walk: {
    mode: 'error',
    rows: [
      {
        index: 1,
        stateBefore: 'closed',
        action: 'call',
        outcome: 'injected error',
        detail: 'reliability demo: the scenario armed the dependency to fail',
        elapsedMs: 0.021,
        stateAfter: 'closed',
      },
      {
        index: 2,
        stateBefore: 'open',
        action: 'fast-fail',
        outcome: 'refused',
        detail: 'open — fast-fail; 1.4s of the 2s cooldown remains',
        elapsedMs: 0.004,
        stateAfter: 'open',
      },
    ],
    note: 'the threshold tripped on attempt 1',
  },
  retry: {
    mode: 'error',
    succeeded: false,
    rows: [
      { attempt: 1, outcome: 'injected error', elapsedMs: 0.02, backoffMs: 41.25 },
      { attempt: 2, outcome: 'injected error', elapsedMs: 0.02, backoffMs: 80.5 },
      { attempt: 3, outcome: 'injected error', elapsedMs: 0.02, backoffMs: 0 },
    ],
    totalElapsedMs: 121.9,
    note: 'the series exhausted its 3 attempts',
  },
  reasoning: 'the breaker state persists across requests',
};

describe('reliabilityBreakerDtoFromJson', () => {
  it('builds the full domain shape from a complete payload', () => {
    const dto = reliabilityBreakerDtoFromJson(structuredClone(fullPayload));

    expect(dto.scenario.mode).toBe('error');
    expect(dto.scenario.modes).toEqual(['none', 'error', 'latency']);
    expect(dto.breaker.state).toBe('open');
    expect(dto.breaker.stateAgeMs).toBe(1234.5);
    expect(dto.breaker.config.failureThreshold).toBe(3);
    expect(dto.breaker.config.halfOpenProbes).toBe(1);
    expect(dto.breaker.counters).toEqual({ allowed: 9, fastFailed: 4, succeeded: 5, failed: 4 });
    expect(dto.breaker.transitions).toHaveLength(2);
    expect(dto.breaker.transitions[1]?.from).toBe('open');
    expect(dto.walk.rows[1]?.action).toBe('fast-fail');
    expect(dto.walk.rows[1]?.detail).toContain('cooldown remains');
    expect(dto.retry.rows[0]?.backoffMs).toBe(41.25);
    expect(dto.retry.succeeded).toBe(false);
    expect(dto.reasoning).toContain('persists across requests');
  });

  it('keeps the detail fields optional but every panel note required', () => {
    const payload = structuredClone(fullPayload);
    delete (payload.walk.rows[0] as Record<string, unknown>).detail;
    const dto = reliabilityBreakerDtoFromJson(payload);
    expect(dto.walk.rows[0]?.detail).toBeUndefined();

    const noNote = structuredClone(fullPayload);
    delete (noNote.walk as Record<string, unknown>).note;
    expect(() => reliabilityBreakerDtoFromJson(noNote)).toThrow('note');
  });

  it('throws on every shape violation', () => {
    const cases: Array<[string, (payload: Record<string, unknown>) => void]> = [
      ['missing breaker', (p) => delete p.breaker],
      ['missing scenario', (p) => delete p.scenario],
      ['missing retry', (p) => delete p.retry],
      ['state not a string', (p) => ((p.breaker as Record<string, unknown>).state = 7)],
      ['counters missing a field', (p) => delete (p.breaker as Record<string, unknown>).counters],
      ['transitions not an array', (p) => ((p.breaker as Record<string, unknown>).transitions = 'none')],
      ['walk rows missing elapsedMs', (p) => {
        const walk = p.walk as Record<string, unknown>;
        const rows = walk.rows as Array<Record<string, unknown>>;
        delete rows[0]?.elapsedMs;
      }],
      ['modes not strings', (p) => {
        const scenario = p.scenario as Record<string, unknown>;
        scenario.modes = [1, 2, 3];
      }],
      ['empty mode string', (p) => ((p.scenario as Record<string, unknown>).mode = '')],
    ];
    for (const [name, mutate] of cases) {
      const payload = structuredClone(fullPayload) as Record<string, unknown>;
      mutate(payload);
      expect(() => reliabilityBreakerDtoFromJson(payload), name).toThrow();
    }
    expect(() => reliabilityBreakerDtoFromJson(0)).toThrow('must be an object');
  });
});
