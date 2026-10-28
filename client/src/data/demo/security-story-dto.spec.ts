import { describe, expect, it } from 'vitest';
import { securityStoryDtoFromJson, toSecurityStoryDemo } from './security-story-dto';
import type {
  SecurityStoryBeat,
  SecurityStoryDemoData,
  SecurityStoryFact,
} from '../../domain/demo/security-story';

/**
 * SecurityStoryDTO spec — whitelist parsing with shape violations
 * throwing (parseDto upgrades them to InvalidResponseException at the
 * boundary). The verdict vocabulary is OPEN on the wire (the client
 * colors, never interprets): the observed ok/rejected/info values must
 * pass through untouched. Story prose validates as non-empty strings,
 * never pinned to literals.
 */

const headerFact: SecurityStoryFact = {
  label: 'response headers',
  value: 'Content-Security-Policy + Strict-Transport-Security present',
};

const sweepBeat: SecurityStoryBeat = {
  beat: 'the input sweep',
  detail: 'the payload is refused by the validator before any query is built — never string-spooged.',
  verdict: 'rejected',
  facts: [{ label: "bobby'; DROP TABLE", value: 'refused at the boundary, 422' }],
};

const headerBeat: SecurityStoryBeat = {
  beat: 'the header audit',
  detail: 'the probe request comes back with both hardening headers set — observed, not assumed.',
  verdict: 'ok',
  facts: [headerFact],
};

const piiBeat: SecurityStoryBeat = {
  beat: 'the PII scan',
  detail: 'masked columns leave the server already redacted — the client never sees raw PII.',
  verdict: 'info',
  facts: [{ label: 'masked fields', value: 'pan → **** 0042 · email → j***@example.com' }],
};

const wellFormed: SecurityStoryDemoData = {
  mechanism: 'security-lab',
  beats: [sweepBeat, headerBeat, piiBeat],
  proofs: [{ label: 'rate limit', value: 'the lab degrades to a typed 429 under hammering' }],
  reasoning: 'the lab refuses first and explains second, so every refusal is observable.',
};

describe('securityStoryDtoFromJson', () => {
  it('accepts a well-formed payload unchanged', () => {
    expect(securityStoryDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('carries the observed verdict values ok/rejected/info through untouched', () => {
    // The vocabulary is open on the wire — the DTO passes the served
    // values through; it never re-types or filters them.
    expect(securityStoryDtoFromJson(wellFormed).beats.map((beat) => beat.verdict)).toEqual([
      'rejected',
      'ok',
      'info',
    ]);
  });

  it('drops unknown keys (whitelist, not pass-through)', () => {
    const padded = { ...wellFormed, surprise: 'nope' };
    expect(securityStoryDtoFromJson(padded)).toEqual(wellFormed);
  });

  it('rejects a beat whose facts carry a non-object', () => {
    const poisoned = {
      ...wellFormed,
      beats: [{ ...headerBeat, facts: [headerFact, 'not a fact'] }],
    };

    expect(() => securityStoryDtoFromJson(poisoned)).toThrow(/fact must be an object/);
  });

  it('rejects a beat missing its verdict key', () => {
    const stripped = { ...headerBeat } as Record<string, unknown>;
    delete stripped.verdict;
    const poisoned = { ...wellFormed, beats: [stripped] };

    expect(() => securityStoryDtoFromJson(poisoned)).toThrow(/verdict/);
  });

  it('rejects a beat missing its teaching sentence', () => {
    const stripped = { ...sweepBeat } as Record<string, unknown>;
    delete stripped.detail;
    const poisoned = { ...wellFormed, beats: [stripped] };

    expect(() => securityStoryDtoFromJson(poisoned)).toThrow(/detail/);
  });

  it('rejects proofs delivered as a non-array', () => {
    const poisoned = { ...wellFormed, proofs: 'one proof, honest' };

    expect(() => securityStoryDtoFromJson(poisoned)).toThrow(/expected array at "proofs"/);
  });

  it('rejects a non-string mechanism', () => {
    const poisoned = { ...wellFormed, mechanism: false };

    expect(() => securityStoryDtoFromJson(poisoned)).toThrow(/expected string at "mechanism"/);
  });

  it('rejects a payload missing the reasoning paragraph', () => {
    const poisoned = { ...wellFormed } as Record<string, unknown>;
    delete poisoned.reasoning;

    expect(() => securityStoryDtoFromJson(poisoned)).toThrow(/reasoning/);
  });

  it('rejects an empty proofs array entry label', () => {
    const poisoned = { ...wellFormed, proofs: [{ label: '', value: 'v' }] };

    expect(() => securityStoryDtoFromJson(poisoned)).toThrow(/label/);
  });

  it('rejects a non-object body outright', () => {
    expect(() => securityStoryDtoFromJson('nope')).toThrow(/must be an object/);
    expect(() => securityStoryDtoFromJson(null)).toThrow(/must be an object/);
  });

  it('toSecurityStoryDemo is the identity seam while DTO and domain are 1:1', () => {
    expect(toSecurityStoryDemo(wellFormed)).toBe(wellFormed);
  });
});
