import { describe, expect, it } from 'vitest';
import { authStoryDtoFromJson, toAuthStoryDemo } from './auth-story-dto';
import type { AuthStoryBeat, AuthStoryDemoData, AuthStoryFact } from '../../domain/demo/auth-story';

/**
 * AuthStoryDTO spec — whitelist parsing with shape violations throwing
 * (parseDto upgrades them to InvalidResponseException at the boundary).
 * The verdict vocabulary is CLOSED on the wire — the four values the
 * server's auth-story constants serve (ok/rejected/alarm/info) pass
 * through untouched; anything else is a shape violation. Story prose
 * validates as non-empty strings, never pinned to literals.
 */

const cookieFact: AuthStoryFact = {
  label: 'Set-Cookie',
  value: '__Host-session=…; Secure; HttpOnly; SameSite=Lax',
};

const signatureFact: AuthStoryFact = {
  label: 'verify error',
  value: 'signature is invalid after the payload edit',
};

const loginBeat: AuthStoryBeat = {
  beat: 'the login',
  detail: 'credentials verified against the stored argon2id hash, a session pair minted.',
  verdict: 'ok',
  facts: [cookieFact],
};

const tamperedBeat: AuthStoryBeat = {
  beat: 'the tampered token',
  detail: 'one byte of payload edited — the signature no longer verifies, the request dies.',
  verdict: 'rejected',
  facts: [signatureFact],
};

const algBeat: AuthStoryBeat = {
  beat: 'the algorithm strip',
  detail: 'alg removed from the header: the server refuses to guess and treats it as RS256.',
  verdict: 'alarm',
  facts: [{ label: 'alg', value: 'absent → refused, never defaulted' }],
};

const wellFormed: AuthStoryDemoData = {
  mechanism: 'auth-jwt',
  beats: [loginBeat, tamperedBeat, algBeat],
  proofs: [{ label: 'token lifetime', value: '15m access · 30d refresh, rotated on use' }],
  reasoning: 'every check the server performs is a check the client can watch fail.',
};

describe('authStoryDtoFromJson', () => {
  it('accepts a well-formed payload unchanged', () => {
    expect(authStoryDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('carries the closed verdict vocabulary ok/rejected/alarm/info through untouched', () => {
    // The vocabulary is closed — the four values the server's constants
    // serve pass through; the DTO re-types nothing.
    expect(authStoryDtoFromJson(wellFormed).beats.map((beat) => beat.verdict)).toEqual([
      'ok',
      'rejected',
      'alarm',
    ]);
    const withInfo = {
      ...wellFormed,
      beats: [
        ...wellFormed.beats,
        { beat: 'the roster', detail: 'a neutral fact beat.', verdict: 'info', facts: [] },
      ],
    };
    expect(authStoryDtoFromJson(withInfo).beats.at(-1)?.verdict).toBe('info');
  });

  it('rejects a verdict outside the closed vocabulary', () => {
    const poisoned = {
      ...wellFormed,
      beats: [{ ...loginBeat, verdict: 'magenta' }],
    };

    expect(() => authStoryDtoFromJson(poisoned)).toThrow(/ok\|rejected\|alarm\|info/);
  });

  it('drops unknown keys (whitelist, not pass-through)', () => {
    const padded = { ...wellFormed, surprise: 'nope' };
    expect(authStoryDtoFromJson(padded)).toEqual(wellFormed);
  });

  it('rejects a beat whose facts carry a non-object', () => {
    const poisoned = {
      ...wellFormed,
      beats: [{ ...loginBeat, facts: [cookieFact, 'not a fact'] }],
    };

    expect(() => authStoryDtoFromJson(poisoned)).toThrow(/fact must be an object/);
  });

  it('rejects a beat missing its verdict key', () => {
    const stripped = { ...loginBeat } as Record<string, unknown>;
    delete stripped.verdict;
    const poisoned = { ...wellFormed, beats: [stripped] };

    expect(() => authStoryDtoFromJson(poisoned)).toThrow(/verdict/);
  });

  it('rejects a beat missing its teaching sentence', () => {
    const stripped = { ...tamperedBeat } as Record<string, unknown>;
    delete stripped.detail;
    const poisoned = { ...wellFormed, beats: [stripped] };

    expect(() => authStoryDtoFromJson(poisoned)).toThrow(/detail/);
  });

  it('rejects beats delivered as a non-array', () => {
    const poisoned = { ...wellFormed, beats: 'three beats, honest' };

    expect(() => authStoryDtoFromJson(poisoned)).toThrow(/expected array at "beats"/);
  });

  it('rejects a non-string mechanism', () => {
    const poisoned = { ...wellFormed, mechanism: 7 };

    expect(() => authStoryDtoFromJson(poisoned)).toThrow(/expected string at "mechanism"/);
  });

  it('rejects a payload missing the reasoning paragraph', () => {
    const poisoned = { ...wellFormed } as Record<string, unknown>;
    delete poisoned.reasoning;

    expect(() => authStoryDtoFromJson(poisoned)).toThrow(/reasoning/);
  });

  it('rejects an empty proofs array entry label', () => {
    const poisoned = { ...wellFormed, proofs: [{ label: '', value: 'v' }] };

    expect(() => authStoryDtoFromJson(poisoned)).toThrow(/label/);
  });

  it('rejects a non-object body outright', () => {
    expect(() => authStoryDtoFromJson('nope')).toThrow(/must be an object/);
    expect(() => authStoryDtoFromJson(null)).toThrow(/must be an object/);
  });

  it('toAuthStoryDemo is the identity seam while DTO and domain are 1:1', () => {
    expect(toAuthStoryDemo(wellFormed)).toBe(wellFormed);
  });
});
