import { describe, expect, it } from 'vitest';
import {
  scenarioDtoFromJson,
  serializeScenarioPatch,
  toScenarioState,
} from './scenario-dto';

/**
 * ScenarioDTO spec — hostile-input discipline: only the exact wire enums
 * parse, anything else throws (and therefore surfaces upstream as an
 * invalid_response Failure, never as a guessed default); the patch
 * serializer emits ONLY the keys the caller set.
 */
describe('scenarioDtoFromJson', () => {
  it('parses a well-formed snapshot', () => {
    expect(
      scenarioDtoFromJson({ dataMode: 'live', injectFailure: 'latency' }),
    ).toEqual({ dataMode: 'live', injectFailure: 'latency' });
  });

  it('accepts every legal enum value', () => {
    for (const dataMode of ['fake', 'live']) {
      for (const injectFailure of ['none', 'error', 'latency']) {
        expect(
          scenarioDtoFromJson({ dataMode, injectFailure }),
        ).toEqual({ dataMode, injectFailure });
      }
    }
  });

  it.each([
    ['null', null],
    ['an array', [{ dataMode: 'fake', injectFailure: 'none' }]],
    ['a string', 'fake'],
    ['missing dataMode', { injectFailure: 'none' }],
    ['missing injectFailure', { dataMode: 'fake' }],
    ['unknown dataMode', { dataMode: 'stale', injectFailure: 'none' }],
    ['unknown injectFailure', { dataMode: 'fake', injectFailure: 'bogus' }],
    ['non-string dataMode', { dataMode: 1, injectFailure: 'none' }],
    ['null injectFailure', { dataMode: 'fake', injectFailure: null }],
  ])('throws for %s', (_label, body) => {
    expect(() => scenarioDtoFromJson(body)).toThrow();
  });

  it('drops unknown wire keys (whitelist, not pass-through)', () => {
    expect(
      scenarioDtoFromJson({
        dataMode: 'fake',
        injectFailure: 'none',
        extra: 'ignored',
      }),
    ).toEqual({ dataMode: 'fake', injectFailure: 'none' });
  });
});

describe('serializeScenarioPatch', () => {
  it('emits only the set key — a partial patch never implies the other', () => {
    expect(JSON.parse(serializeScenarioPatch({ injectFailure: 'latency' }))).toEqual({
      injectFailure: 'latency',
    });
    expect(JSON.parse(serializeScenarioPatch({ dataMode: 'live' }))).toEqual({
      dataMode: 'live',
    });
  });

  it('emits both keys when both are set, and an empty object for an empty patch', () => {
    expect(
      JSON.parse(serializeScenarioPatch({ dataMode: 'live', injectFailure: 'error' })),
    ).toEqual({ dataMode: 'live', injectFailure: 'error' });
    expect(JSON.parse(serializeScenarioPatch({}))).toEqual({});
  });
});

describe('toScenarioState', () => {
  it('maps the DTO onto the domain model', () => {
    expect(
      toScenarioState({ dataMode: 'live', injectFailure: 'error' }),
    ).toEqual({ dataMode: 'live', injectFailure: 'error' });
  });
});
