import { describe, expect, it } from 'vitest';
import { errorsExplorerDtoFromJson } from './errors-explorer-dto';
import type { ErrorsExplorerDemoData } from '../../domain/demo/errors-explorer';

/**
 * ErrorsExplorer DTO spec — fixture-driven: the full payload
 * round-trips into the domain shape, and each closed vocabulary
 * (trigger mode, walk injection mode) plus every required field is
 * enforced: a violation throws, which parseDto upgrades to an
 * InvalidResponseException upstream.
 */

const fixture: ErrorsExplorerDemoData = {
  kinds: [
    {
      kind: 'validation_failed',
      httpStatus: 422,
      defaultMessage: 'Validation failed.',
      raisedBy: 'the boundary decoders and the store\u2019s own guard',
      loggedAs: 'error response \u00b7 warn \u2014 kind, message, status, request id',
      trigger: {
        mode: 'live',
        method: 'PATCH',
        path: '/api/demo/scenario',
        body: '{"injectFailure": "bogus"}',
        note: 'an enum value the merged patch refuses',
      },
    },
    {
      kind: 'internal',
      httpStatus: 500,
      defaultMessage: 'Internal server error.',
      raisedBy: 'nothing raises internal on purpose',
      loggedAs: 'panic recovered \u00b7 error \u2014 the stack and the request id',
      trigger: {
        mode: 'recipe',
        method: 'GET',
        path: '/api/demo/errors/probe',
        prep: 'PATCH /api/demo/scenario {"injectFailure": "error"}',
        note: 'the probe panics; the Recover layer answers the 500 envelope',
      },
    },
  ],
  envelopeNote: 'every failure answers the SAME envelope',
  scenario: {
    getEndpoint: '/api/demo/scenario',
    patchEndpoint: '/api/demo/scenario',
    modes: ['none', 'error', 'latency'],
    note: 'the runtime toggle every demo reads',
  },
  probe: {
    probeEndpoint: '/api/demo/errors/probe',
    latencyMillis: 1500,
    healthyNote: 'injectFailure=none answers 200 healthy',
    injectedNote: 'injectFailure=error answers the recovered 500 envelope',
    latencyNote: 'injectFailure=latency sleeps out its 1500 ms injection',
  },
  walk: {
    injectFailure: 'error',
    status: 500,
    elapsedMillis: 4,
    requestID: 'rid-explorer-1',
    body: '{"error":{"code":"internal","message":"Internal server error.","requestId":"rid-explorer-1"}}',
    note: 'error injection honored: the probe panicked',
  },
  reasoning: 'eight kinds, one envelope, zero exceptions',
};

describe('errorsExplorerDtoFromJson', () => {
  it('round-trips the full payload into the domain shape', () => {
    const parsed = errorsExplorerDtoFromJson(fixture);

    expect(parsed.kinds).toHaveLength(2);
    expect(parsed.kinds[0]?.kind).toBe('validation_failed');
    expect(parsed.kinds[0]?.httpStatus).toBe(422);
    expect(parsed.kinds[0]?.trigger.body).toBe('{"injectFailure": "bogus"}');
    expect(parsed.kinds[1]?.trigger.prep).toContain('injectFailure');
    expect(parsed.scenario.modes).toEqual(['none', 'error', 'latency']);
    expect(parsed.probe.latencyMillis).toBe(1500);
    expect(parsed.walk.status).toBe(500);
    expect(parsed.walk.note).toContain('probe panicked');
    expect(parsed.reasoning).toBe('eight kinds, one envelope, zero exceptions');
  });

  it('rejects an unknown trigger mode', () => {
    const poisoned = {
      ...fixture,
      kinds: [{ ...fixture.kinds[0]!, trigger: { ...fixture.kinds[0]!.trigger, mode: 'maybe' } }],
    };

    expect(() => errorsExplorerDtoFromJson(poisoned)).toThrow(/live\|recipe/);
  });

  it('rejects an unknown walk injection mode', () => {
    const poisoned = { ...fixture, walk: { ...fixture.walk, injectFailure: 'sometimes' } };

    expect(() => errorsExplorerDtoFromJson(poisoned)).toThrow(/none\|error\|latency/);
  });

  it('rejects a payload whose walk body is missing', () => {
    const poisoned = { ...fixture, walk: { ...fixture.walk, body: '' } };

    expect(() => errorsExplorerDtoFromJson(poisoned)).toThrow('body');
  });

  it('rejects a non-object payload', () => {
    expect(() => errorsExplorerDtoFromJson(null)).toThrow();
    expect(() => errorsExplorerDtoFromJson('nope')).toThrow();
  });
});
