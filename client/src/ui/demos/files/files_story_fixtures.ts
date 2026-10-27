import type { FilesStoryDemoData } from '../../../domain/demo/files-story';

/**
 * The shared fixture for the workbench demo spec — shaped exactly like
 * the server's payload, with the topic's own distinctive beats so the
 * spec pins the demo's teaching content (the streaming honesty number,
 * the mid-stream refusal, the range census, the sweep, the bomb) rather
 * than generic story chrome.
 */

/** A workbench-shaped story: the five beats, honest to the wire shape. */
export function filesWorkbenchFixture(
  overrides: Partial<FilesStoryDemoData> = {},
): FilesStoryDemoData {
  return {
    mechanism: 'files-workbench',
    beats: [
      {
        beat: 'the streamed upload',
        detail:
          'a real multipart body — 4 MiB of deterministic bytes — flows through a PIPE into the store part by part; the exchange allocated 131.1 KiB.',
        verdict: 'ok',
        facts: [
          { label: 'object', value: 'workbench.bin · 4 MiB · stored: true' },
          { label: 'the bound', value: 'io.LimitReader at 8 MiB — the copy stops the moment the stream exceeds it' },
        ],
      },
      {
        beat: 'the size bound refuses mid-stream',
        detail: 'the LimitReader lets exactly one proving byte past the line; nothing landed (0 leftover staging files).',
        verdict: 'rejected',
        facts: [{ label: 'partial files left', value: '0 — the staged temp is deleted on every failure path' }],
      },
      {
        beat: 'the ranged download',
        detail:
          'the mid-file slice matched byte-for-byte; the past-the-end range answered 416 with the honest census bytes */4194304.',
        verdict: 'ok',
        facts: [{ label: 'Content-Range', value: 'bytes 1048576-1048639/4194304' }],
      },
      {
        beat: 'the traversal sweep',
        detail: '6 of 6 hostile names refused; the symlink escape caught by EvalSymlinks re-proving the prefix.',
        verdict: 'rejected',
        facts: [{ label: 'the sweep', value: '../escape.txt\u2192REFUSED  /etc/passwd\u2192REFUSED' }],
      },
      {
        beat: 'the thumbnail',
        detail: 'box sampling downscales 320\u00d7200 to 64\u00d740; a 53-byte header claiming 30000\u00d730000 refused before allocation.',
        verdict: 'ok',
        facts: [{ label: 'thumbnail', value: '64\u00d740 \u2014 png, jpeg (q85)' }],
      },
    ],
    proofs: [
      {
        label: 'streaming honesty',
        value: '4 MiB flowed for 131.1 KiB of allocations — io.Copy\u2019s fixed buffer, not the object',
      },
    ],
    reasoning:
      'uploads stream, downloads seek, limits refuse mid-stream, and names never become paths.',
    ...overrides,
  };
}
