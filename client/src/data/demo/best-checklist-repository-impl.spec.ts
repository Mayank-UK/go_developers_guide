import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { BestChecklistApi } from './best-checklist-api';
import { BestChecklistRepositoryImpl } from './best-checklist-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * BestChecklistRepositoryImpl spec — fixture-driven (happy + failure):
 * the self-audit panel (areas, not-dos, summary) round-trips; malformed
 * payloads and transport failures resolve as Failed Results, never
 * throws.
 */

const demoPayload = {
  mode: 'boot-cached: the whole audit executes ONCE at process boot',
  computedAt: '2026-10-28T09:15:04Z',
  areas: [
    {
      id: 'git-and-review',
      title: 'Git & review',
      note: 'Machines check what machines can; humans read intent.',
      items: [
        {
          id: 'ratchet-green',
          practice: 'Quoted teaching is real — the snippet-ratchet suite passes',
          checkKind: 'exec',
          verdict: 'pass',
          evidence: ['command: go test -count=1 -run ratchet ./internal/content'],
        },
        {
          id: 'review-your-diffs',
          practice: 'Every diff gets a human read before it lands',
          checkKind: 'manual',
          verdict: 'manual',
          evidence: [],
          whyManual: 'no automation reads a diff’s design intent',
        },
      ],
    },
  ],
  notDones: [
    {
      id: 'no-hosted-ci',
      notDone: 'No hosted CI or Dependabot',
      reason: 'Local verification is the only gate, by family decision.',
      artifacts: ['the verify one-liner — this topic’s terminal companion'],
    },
  ],
  summary: { items: 15, passed: 13, failed: 0, manual: 2 },
  honestyNote: 'Every machine verdict was computed from machinery that actually executed.',
};

function makeRepository(): BestChecklistRepositoryImpl {
  return new BestChecklistRepositoryImpl(new BestChecklistApi(createAppConfig()));
}

describe('BestChecklistRepositoryImpl', () => {
  it('resolves the audit panel on the happy path', async () => {
    stubApiFetch([{ url: '/api/demo/best/checklist', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (!result.ok) return;
    expect(result.value.summary.passed).toBe(13);
    expect(result.value.areas[0]?.items[1]?.verdict).toBe('manual');
    expect(result.value.notDones[0]?.id).toBe('no-hosted-ci');
  });

  it('resolves a Failed Result when the payload violates the shape', async () => {
    stubApiFetch([{ url: '/api/demo/best/checklist', body: { mode: 'broken', areas: 'nope' } }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
  });

  it('resolves a Failed Result when the transport itself fails', async () => {
    stubNetworkFailure();

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
  });

  it('resolves a Failed Result on a non-2xx error envelope', async () => {
    stubApiFetch([
      {
        url: '/api/demo/best/checklist',
        status: 503,
        body: { error: { code: 'service_unavailable', message: 'unavailable', requestId: 'r-1' } },
      },
    ]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
  });
});
