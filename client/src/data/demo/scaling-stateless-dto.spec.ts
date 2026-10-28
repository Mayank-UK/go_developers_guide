import { describe, expect, it } from 'vitest';
import { scalingStatelessDtoFromJson } from './scaling-stateless-dto';

/**
 * ScalingStatelessDTO spec — the fromJson whitelist: a full payload
 * builds the domain shape (the config panel, the measured
 * distribution, the thirteen-step session round, the counter beat,
 * the verdicts); every violation (missing panels, wrong types, a step
 * without its measured observation) throws so parseDto can upgrade it
 * to an InvalidResponseException.
 */

const fullPayload = {
  mode: 'one fleet, two placements: three in-process replicas behind a real round-robin balancer',
  config: {
    replicas: 3,
    distributionRequests: 12,
    clientId: 'ada',
    counterIncrements: 6,
    localToken: 's-local-ada',
    sharedToken: 's-shared-ada',
    sharedStoreNote: 'an in-process analogue of the redis container this stack carries',
  },
  distribution: {
    totalServed: 12,
    rows: [
      { replicaId: 'replica-1', served: 4 },
      { replicaId: 'replica-2', served: 4 },
      { replicaId: 'replica-3', served: 4 },
    ],
    note: '12 request(s) across 3 replica(s), per-replica 4/4/4 — spread exactly even',
  },
  steps: [
    {
      step: 1,
      phase: 'local-memory',
      label: 'login: the session lands in replica-local memory',
      route: 'round-robin',
      replicaId: 'replica-1',
      outcome: 'stored',
      detail: "the write goes to the replica the balancer picked — its own map, nobody else's",
      observed: 'replica-1 holds 1 session(s) after storing s-local-ada — the only copy in the fleet',
    },
    {
      step: 5,
      phase: 'swap',
      label: 'a rolling update replaces the pinned replica',
      route: 'rolling update',
      replicaId: 'replica-1',
      outcome: 'replaced',
      detail: "one replica terminated, a fresh one under the same slot — the k8s manifest's maxUnavailable 0 dance",
      observed: 'replica-1 terminated: 6 request(s) served, 1 session(s) held — both gone',
    },
  ],
  counter: {
    rows: [
      { replicaId: 'replica-1', increments: 2 },
      { replicaId: 'replica-2', increments: 2 },
      { replicaId: 'replica-3', increments: 2 },
    ],
    total: 6,
    note: '6 increments through three different replicas read back as 6 from one store',
  },
  verdicts: [
    {
      id: 'distribution',
      label: 'the balancer spreads the work',
      value: '12 requests landed 4/4/4 across 3 replicas — measured per replica after the round',
    },
    {
      id: 'the-trade',
      label: 'the trade this topic keeps making',
      value: 'in-process state is fast, free, and invisible to every other replica',
    },
  ],
  honesty: 'Every count on this panel is real — the round is deterministic',
};

describe('scalingStatelessDtoFromJson', () => {
  it('builds the domain shape from a full payload', () => {
    const dto = scalingStatelessDtoFromJson(fullPayload);
    expect(dto.config.replicas).toBe(3);
    expect(dto.config.localToken).toBe('s-local-ada');
    expect(dto.distribution.totalServed).toBe(12);
    expect(dto.distribution.rows).toHaveLength(3);
    expect(dto.distribution.rows[0]).toEqual({ replicaId: 'replica-1', served: 4 });
    expect(dto.steps).toHaveLength(2);
    expect(dto.steps[1]?.phase).toBe('swap');
    expect(dto.steps[1]?.outcome).toBe('replaced');
    expect(dto.counter.total).toBe(6);
    expect(dto.counter.rows[2]?.increments).toBe(2);
    expect(dto.verdicts[0]?.value).toContain('4/4/4');
    expect(dto.honesty).toContain('deterministic');
  });

  it('requires the panel notes and observations as real text', () => {
    const noNote = structuredClone(fullPayload);
    delete (noNote.distribution as Record<string, unknown>).note;
    expect(() => scalingStatelessDtoFromJson(noNote)).toThrow('note');

    const noObserved = structuredClone(fullPayload);
    delete (noObserved.steps as Array<Record<string, unknown>>)[0]?.observed;
    expect(() => scalingStatelessDtoFromJson(noObserved)).toThrow('observed');
  });

  it('throws on every shape violation', () => {
    const cases: Array<[string, (payload: Record<string, unknown>) => void]> = [
      ['missing config', (p) => delete p.config],
      ['missing distribution', (p) => delete p.distribution],
      ['missing counter', (p) => delete p.counter],
      ['missing verdicts', (p) => delete p.verdicts],
      ['steps not an array', (p) => (p.steps = 'none')],
      ['served not a number', (p) => {
        const distribution = p.distribution as Record<string, unknown>;
        const rows = distribution.rows as Array<Record<string, unknown>>;
        rows[0]!.served = 'four';
      }],
      ['phase not a string', (p) => ((p.steps as Array<Record<string, unknown>>)[0]!.phase = 7)],
      ['totalServed missing', (p) => delete (p.distribution as Record<string, unknown>).totalServed],
      ['clientId empty', (p) => ((p.config as Record<string, unknown>).clientId = '')],
    ];
    for (const [name, mutate] of cases) {
      const payload = structuredClone(fullPayload) as Record<string, unknown>;
      mutate(payload);
      expect(() => scalingStatelessDtoFromJson(payload), name).toThrow();
    }
    expect(() => scalingStatelessDtoFromJson(0)).toThrow('must be an object');
  });
});
