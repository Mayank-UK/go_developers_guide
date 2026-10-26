import { describe, expect, it } from 'vitest';
import { raceDetectorDtoFromJson, toRaceDetectorDemo } from './race-detector-dto';
import type { RaceDetectorDemoData } from '../../domain/demo/race-detector';

/**
 * RaceDetectorDTO spec — hostile-input discipline: only the exact wire
 * shape parses, anything else throws (and therefore surfaces upstream as
 * an invalid_response Failure, never as a guessed default). The
 * well-formed fixture mirrors the REAL builder output of
 * GET /api/demo/conc/race, with the multi-line exhibit strings shortened
 * to their load-bearing lines.
 */

const wellFormed: RaceDetectorDemoData = {
  race: {
    code: 'package raceexhibit\n\nimport "sync"\n\nfunc RacyTotal(goroutines, increments int) int {\n\tcounter := 0\n',
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
    {
      case: 'unbuffered send with a waiting reader',
      readerPresent: true,
      outcome: 'delivered "done" — goroutine returned',
    },
    {
      case: 'unbuffered send after the reader is gone, guarded by select+default',
      readerPresent: false,
      outcome:
        'send abandoned by the default arm — goroutine returned (a plain send would block forever: the leak)',
    },
  ],
};

describe('raceDetectorDtoFromJson', () => {
  it('parses a well-formed payload round-trip', () => {
    expect(raceDetectorDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('keeps both live totals exact and the hygiene contrast intact', () => {
    const parsed = raceDetectorDtoFromJson(wellFormed);

    expect(parsed.mutex.observed).toBe(5000);
    expect(parsed.channel.exact).toBe(true);
    expect(parsed.hygiene.map((row) => row.readerPresent)).toEqual([true, false]);
  });

  it.each([
    ['null', null],
    ['an array', [wellFormed]],
    ['a string', 'nope'],
    ['missing race', { ...wellFormed, race: undefined }],
    ['missing mutex', { ...wellFormed, mutex: undefined }],
    ['missing channel', { ...wellFormed, channel: undefined }],
    ['missing hygiene', { ...wellFormed, hygiene: undefined }],
    ['hygiene not an array', { ...wellFormed, hygiene: 'nope' }],
    ['a non-object hygiene row', { ...wellFormed, hygiene: ['nope'] }],
    ['missing verdict', { ...wellFormed, race: { ...wellFormed.race, verdict: undefined } }],
    ['empty verdict', { ...wellFormed, race: { ...wellFormed.race, verdict: '' } }],
    ['missing exact flag', { ...wellFormed, mutex: { ...wellFormed.mutex, exact: undefined } }],
    ['non-boolean exact', { ...wellFormed, channel: { ...wellFormed.channel, exact: 1 } }],
    ['NaN observed', { ...wellFormed, mutex: { ...wellFormed.mutex, observed: Number.NaN } }],
    ['missing readerPresent', { ...wellFormed, hygiene: [{ ...wellFormed.hygiene[0]!, readerPresent: undefined }] }],
    ['non-object race exhibit', { ...wellFormed, race: 'pinned' }],
  ])('throws for %s', (_label, body) => {
    expect(() => raceDetectorDtoFromJson(body)).toThrow();
  });

  it('drops unknown wire keys (whitelist, not pass-through)', () => {
    const parsed = raceDetectorDtoFromJson({
      ...wellFormed,
      extra: 'ignored',
      race: { ...wellFormed.race, sneaky: true },
    });

    expect(parsed).toEqual(wellFormed);
  });
});

describe('toRaceDetectorDemo', () => {
  it('maps the DTO onto the domain model (identity while 1:1)', () => {
    expect(toRaceDetectorDemo(wellFormed)).toBe(wellFormed);
  });
});
