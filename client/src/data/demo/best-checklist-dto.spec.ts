import { describe, expect, it } from 'vitest';
import { bestChecklistDtoFromJson } from './best-checklist-dto';

/**
 * BestChecklistDTO spec — the fromJson whitelist: a full audit payload
 * builds the domain shape (areas with verdict rows, the not-dos, the
 * summary), the honest computing placeholder (empty strings and empty
 * arrays) is DATA and passes, and every violation (missing panels,
 * wrong types, unknown verdicts) throws so parseDto can upgrade it to
 * an InvalidResponseException.
 */

const fullPayload = {
  mode: 'boot-cached: the whole audit executes ONCE at process boot',
  computedAt: '2026-10-28T09:15:04Z',
  areas: [
    {
      id: 'setup',
      title: 'Setup',
      note: 'Toolchain discipline — format and pins that hold the floor.',
      items: [
        {
          id: 'gofmt-clean',
          practice: 'gofmt is non-negotiable — gofmt -l over the server tree lists nothing',
          checkKind: 'exec',
          verdict: 'pass',
          evidence: ['command: gofmt -l . (in server/)'],
        },
        {
          id: 'module-pins',
          practice: 'Dependencies are pinned, exactly',
          checkKind: 'file',
          verdict: 'pass',
          evidence: ['server/go.mod: go 1.27.3 directive, 15 direct requires'],
        },
      ],
    },
    {
      id: 'human-practices',
      title: 'Human practices',
      note: 'The judgments no checklist can make for you.',
      items: [
        {
          id: 'prod-secrets-real',
          practice: 'Production overrides every dev-default secret with real values',
          checkKind: 'manual',
          verdict: 'manual',
          evidence: [],
          whyManual: 'an operator fact no local check can see',
        },
      ],
    },
  ],
  notDones: [
    {
      id: 'stdlib-first',
      notDone: 'No web framework, no test framework — stdlib first',
      reason: 'net/http is the backbone; chi, gin, and echo are taught by comparison only.',
      artifacts: ['server/cmd/guide/main.go — the whole surface on net/http'],
    },
  ],
  summary: { items: 15, passed: 13, failed: 0, manual: 2 },
  honestyNote: 'Every machine verdict on this panel was computed from machinery that actually executed.',
};

const placeholderPayload = {
  mode: 'computing: the boot audit is still executing',
  computedAt: '',
  areas: [],
  notDones: [],
  summary: { items: 0, passed: 0, failed: 0, manual: 0 },
  honestyNote: '',
};

/** Builds a one-area payload whose single row is the hostile override. */
function withHostileRow(overrides: Record<string, unknown>): Record<string, unknown> {
  const row: Record<string, unknown> = {
    id: 'hostile-row',
    practice: 'a practice line',
    checkKind: 'file',
    verdict: 'pass',
    evidence: ['the evidence'],
  };
  return {
    mode: 'boot-cached',
    computedAt: '2026-10-28T09:15:04Z',
    areas: [{ id: 'setup', title: 'Setup', note: '', items: [{ ...row, ...overrides }] }],
    notDoses: [],
    summary: { items: 1, passed: 1, failed: 0, manual: 0 },
    honestyNote: '',
  };
}

describe('bestChecklistDtoFromJson', () => {
  it('builds the full audit from a complete payload, dropping unknown keys', () => {
    const dto = bestChecklistDtoFromJson({ ...fullPayload, surprise: true });

    expect(dto.mode).toContain('boot-cached');
    expect(dto.computedAt).toBe('2026-10-28T09:15:04Z');
    expect(dto.areas).toHaveLength(2);
    expect(dto.areas.find((area) => area.id === 'setup')?.items[0]).toEqual({
      id: 'gofmt-clean',
      practice: expect.any(String),
      checkKind: 'exec',
      verdict: 'pass',
      evidence: ['command: gofmt -l . (in server/)'],
      whyManual: undefined,
    });
    expect(dto.areas.find((area) => area.id === 'human-practices')?.items[0]?.whyManual).toBe(
      'an operator fact no local check can see',
    );
    expect(dto.notDones[0]?.notDone).toContain('stdlib first');
    expect(dto.summary).toEqual({ items: 15, passed: 13, failed: 0, manual: 2 });
    expect(dto.honestyNote).not.toBe('');
  });

  it('accepts the honest computing placeholder — empty strings and arrays are data', () => {
    const dto = bestChecklistDtoFromJson(placeholderPayload);

    expect(dto.computedAt).toBe('');
    expect(dto.areas).toEqual([]);
    expect(dto.notDones).toEqual([]);
    expect(dto.summary.items).toBe(0);
  });

  it('throws on a non-object payload', () => {
    expect(() => bestChecklistDtoFromJson(null)).toThrow();
    expect(() => bestChecklistDtoFromJson('panel')).toThrow();
  });

  it('throws when a required panel is missing or the wrong type', () => {
    expect(() => bestChecklistDtoFromJson({ ...fullPayload, areas: null })).toThrow();
    expect(() => bestChecklistDtoFromJson({ ...fullPayload, summary: 'fifteen' })).toThrow();
    expect(() => bestChecklistDtoFromJson({ ...fullPayload, notDones: undefined })).toThrow();
  });

  it('throws on an unknown check kind or verdict — hostile rows never reach the domain', () => {
    expect(() => bestChecklistDtoFromJson(withHostileRow({ checkKind: 'vibes' }))).toThrow(/checkKind/);
    expect(() => bestChecklistDtoFromJson(withHostileRow({ verdict: 'green' }))).toThrow(/verdict/);
  });

  it('throws when a row loses its evidence array or its identity', () => {
    expect(() => bestChecklistDtoFromJson(withHostileRow({ evidence: undefined }))).toThrow(/evidence/);
    expect(() => bestChecklistDtoFromJson(withHostileRow({ id: undefined }))).toThrow(/id/);
  });
});
