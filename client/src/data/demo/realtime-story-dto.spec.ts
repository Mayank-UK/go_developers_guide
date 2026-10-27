import { describe, expect, it } from 'vitest';
import { realtimeStoryDtoFromJson, toRealtimeStoryDemo } from './realtime-story-dto';
import type { RealtimeStoryDemoData } from '../../domain/demo/realtime-story';

/**
 * RealtimeStoryDTO spec — whitelist parsing with shape violations
 * throwing (parseDto upgrades them to InvalidResponseException at the
 * boundary). Verdicts validate against the server's closed
 * ok/rejected/info vocabulary; story prose — including every derived
 * frame byte the ticker story shows — validates as non-empty strings,
 * never pinned to literals.
 */

const wellFormed: RealtimeStoryDemoData = {
  mechanism: 'sse-ticker',
  beats: [
    {
      beat: 'the frame anatomy',
      detail: 'an SSE event is field lines ended by ONE blank line — the blank line is the delivery signal.',
      verdict: 'ok',
      facts: [
        {
          label: 'a real tick frame',
          value: 'id: 41\nevent: tick\ndata: {"seq":41,"kind":"tick","from":"feed","body":"tick","at":"2026-10-27T09:30:00Z"}\n\n',
        },
      ],
    },
    {
      beat: 'the slow consumer',
      detail: 'the stalled subscriber stopped reading; its buffer stayed full past the drop budget.',
      verdict: 'rejected',
      facts: [{ label: 'the eviction close', value: 'code 1013 — evicted by the slow-consumer policy' }],
    },
  ],
  proofs: [
    {
      label: 'the resume math',
      value: 'ReplayAfter(38) → 39, 40, 41, computed live by the payload builder from a real feed',
    },
  ],
  reasoning: 'the server flushes, numbers, retains, and keeps the line warm — the browser owns the retry.',
};

describe('realtimeStoryDtoFromJson', () => {
  it('accepts a well-formed payload unchanged', () => {
    expect(realtimeStoryDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('rejects an unknown verdict', () => {
    const poisoned = {
      ...wellFormed,
      beats: [{ ...wellFormed.beats[0], verdict: 'greenish' }],
    };

    expect(() => realtimeStoryDtoFromJson(poisoned)).toThrow(/unknown realtime story verdict/);
  });

  it('rejects a beat whose facts carry a non-object', () => {
    const poisoned = {
      ...wellFormed,
      beats: [{ ...wellFormed.beats[0], facts: [{ label: 'x', value: 'y' }, 'not a fact'] }],
    };

    expect(() => realtimeStoryDtoFromJson(poisoned)).toThrow(/fact must be an object/);
  });

  it('rejects a payload missing the reasoning paragraph', () => {
    const poisoned = { ...wellFormed } as Record<string, unknown>;
    delete poisoned.reasoning;

    expect(() => realtimeStoryDtoFromJson(poisoned)).toThrow(/reasoning/);
  });

  it('rejects an empty proofs entry label', () => {
    const poisoned = { ...wellFormed, proofs: [{ label: '', value: 'v' }] };

    expect(() => realtimeStoryDtoFromJson(poisoned)).toThrow(/label/);
  });

  it('rejects a non-object body outright', () => {
    expect(() => realtimeStoryDtoFromJson('nope')).toThrow(/must be an object/);
    expect(() => realtimeStoryDtoFromJson(null)).toThrow(/must be an object/);
  });

  it('toRealtimeStoryDemo is the identity seam while DTO and domain are 1:1', () => {
    expect(toRealtimeStoryDemo(wellFormed)).toBe(wellFormed);
  });
});
