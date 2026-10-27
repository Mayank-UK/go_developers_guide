import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { FilesStoryApi } from './files-story-api';
import { FilesStoryRepositoryImpl } from './files-story-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * FilesStoryRepositoryImpl spec — fixture-driven (happy + failure): the
 * workbench story payload (five beats with verdicts and facts, proofs,
 * reasoning) round-trips; a poisoned payload and a network failure
 * resolve as Failed Results, never throws.
 */

const storyPayload = {
  mechanism: 'files-workbench',
  beats: [
    {
      beat: 'the streamed upload',
      detail: 'the whole exchange allocated 131.1 KiB against 4 MiB of payload.',
      verdict: 'ok',
      facts: [{ label: 'object', value: 'workbench.bin · 4 MiB · stored: true' }],
    },
    {
      beat: 'the size bound refuses mid-stream',
      detail: 'the copy stops, the staged temp file is removed, the store answers ErrTooLarge.',
      verdict: 'rejected',
      facts: [{ label: 'partial files left', value: '0' }],
    },
    {
      beat: 'the ranged download',
      detail: 'bytes 1048576-1048639/4194304, matched byte-for-byte; past-the-end answered 416.',
      verdict: 'ok',
      facts: [{ label: 'Content-Range', value: 'bytes 1048576-1048639/4194304' }],
    },
    {
      beat: 'the traversal sweep',
      detail: '6 of 6 attempts refused; the symlink escape caught by EvalSymlinks re-proving the prefix.',
      verdict: 'rejected',
      facts: [{ label: 'the sweep', value: '../escape.txt\u2192REFUSED  /etc/passwd\u2192REFUSED' }],
    },
    {
      beat: 'the thumbnail',
      detail: 'box sampling downscales the longest edge to 64; the bomb refused before allocation.',
      verdict: 'ok',
      facts: [{ label: 'thumbnail', value: '64\u00d740 \u2014 png, jpeg (q85)' }],
    },
  ],
  proofs: [{ label: 'the pixel budget', value: 'a 30000\u00d730000 claim refused in 53 bytes' }],
  reasoning: 'uploads stream, downloads seek, limits refuse mid-stream, names never become paths.',
};

function makeRepository(): FilesStoryRepositoryImpl {
  return new FilesStoryRepositoryImpl(new FilesStoryApi(createAppConfig(), '/demo/files/workbench'));
}

describe('FilesStoryRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/files/workbench', body: storyPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.mechanism).toBe('files-workbench');
      expect(result.value.beats).toHaveLength(5);
      expect(result.value.beats[1]?.verdict).toBe('rejected');
      expect(result.value.beats[3]?.facts[0]?.label).toBe('the sweep');
      expect(result.value.proofs[0]?.label).toBe('the pixel budget');
      expect(result.value.reasoning).toContain('never become paths');
    }
  });

  it('rejects a payload whose beat verdict is unknown', async () => {
    const poisoned = {
      ...storyPayload,
      beats: [{ ...storyPayload.beats[0], verdict: 'maybe' }],
    };
    stubApiFetch([{ url: '/api/demo/files/workbench', body: poisoned }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
    }
  });

  it('guards a network failure into a Failed Result (network_error, status 0)', async () => {
    stubNetworkFailure();

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });
});
