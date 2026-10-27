import { describe, expect, it } from 'vitest';
import { filesStoryDtoFromJson, toFilesStoryDemo } from './files-story-dto';
import type { FilesStoryDemoData } from '../../domain/demo/files-story';

/**
 * FilesStoryDTO spec — whitelist parsing with shape violations throwing
 * (parseDto upgrades them to InvalidResponseException at the boundary).
 * Verdicts validate against the server's closed ok/rejected/info
 * vocabulary; story prose validates as non-empty strings, never pinned
 * to literals.
 */

const wellFormed: FilesStoryDemoData = {
  mechanism: 'files-workbench',
  beats: [
    {
      beat: 'the streamed upload',
      detail:
        'a real multipart body — 4 MiB of deterministic bytes — flows through a PIPE into the store part by part.',
      verdict: 'ok',
      facts: [
        { label: 'object', value: 'workbench.bin · 4 MiB · stored: true' },
        { label: 'the bound', value: 'io.LimitReader at 8 MiB' },
      ],
    },
    {
      beat: 'the size bound refuses mid-stream',
      detail: 'the LimitReader lets exactly one proving byte past the line, the copy stops there.',
      verdict: 'rejected',
      facts: [{ label: 'partial files left', value: '0 — the staged temp is deleted on every failure path' }],
    },
  ],
  proofs: [
    {
      label: 'streaming honesty',
      value: '4 MiB flowed for 131.1 KiB of allocations — io.Copy\u2019s fixed buffer, not the object',
    },
  ],
  reasoning:
    'the bytes exist on disk and on the wire, never in memory, because every hop reads a bounded buffer.',
};

describe('filesStoryDtoFromJson', () => {
  it('accepts a well-formed payload unchanged', () => {
    expect(filesStoryDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('rejects an unknown verdict', () => {
    const poisoned = {
      ...wellFormed,
      beats: [{ ...wellFormed.beats[0], verdict: 'greenish' }],
    };

    expect(() => filesStoryDtoFromJson(poisoned)).toThrow(/unknown files story verdict/);
  });

  it('rejects a beat whose facts carry a non-object', () => {
    const poisoned = {
      ...wellFormed,
      beats: [{ ...wellFormed.beats[0], facts: [{ label: 'x', value: 'y' }, 'not a fact'] }],
    };

    expect(() => filesStoryDtoFromJson(poisoned)).toThrow(/fact must be an object/);
  });

  it('rejects a payload missing the reasoning paragraph', () => {
    const poisoned = { ...wellFormed } as Record<string, unknown>;
    delete poisoned.reasoning;

    expect(() => filesStoryDtoFromJson(poisoned)).toThrow(/reasoning/);
  });

  it('rejects an empty proofs array entry label', () => {
    const poisoned = { ...wellFormed, proofs: [{ label: '', value: 'v' }] };

    expect(() => filesStoryDtoFromJson(poisoned)).toThrow(/label/);
  });

  it('rejects a non-object body outright', () => {
    expect(() => filesStoryDtoFromJson('nope')).toThrow(/must be an object/);
    expect(() => filesStoryDtoFromJson(null)).toThrow(/must be an object/);
  });

  it('toFilesStoryDemo is the identity seam while DTO and domain are 1:1', () => {
    expect(toFilesStoryDemo(wellFormed)).toBe(wellFormed);
  });
});
