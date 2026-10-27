import { describe, expect, it } from 'vitest';
import { obsViewerDtoFromJson } from './obs-viewer-dto';

/**
 * ObsViewerDTO spec — the fromJson whitelist: a full payload builds
 * the domain shape (anatomy, level matrix, correlated replay); every
 * violation (missing panels, wrong field types) throws so parseDto
 * can upgrade it to an InvalidResponseException. Optional correlation
 * fields on replay lines round-trip as undefined when absent.
 */

const fullPayload = {
  anatomy: {
    line: '{"time":"2026-10-27T10:00:00Z","level":"INFO","msg":"http_request"}',
    fields: [
      { key: 'time', value: '"2026-10-27T10:00:00Z"', note: 'stamped by the handler' },
      { key: 'level', value: '"INFO"', note: 'the dial the line passed' },
    ],
    note: 'the line above is verbatim what slog wrote',
  },
  levels: {
    rows: [
      {
        level: 'DEBUG',
        survivesAt: ['debug'],
        emittedBy: 'nothing today — the investigation dial',
        note: 'measured by really running the gate',
      },
      {
        level: 'INFO',
        survivesAt: ['debug', 'info'],
        emittedBy: 'the process heartbeat',
        note: 'measured by really running the gate',
      },
    ],
    gate: 'GDG_LOG_LEVEL (debug|info|warn|error)',
    note: 'the dial is asymmetric by design',
  },
  replay: {
    requests: [
      { label: 'the 200', request: 'GET /api/posts?limit=2&sort=newest', requestId: 'rid-obs-1', status: 200 },
      { label: 'the 404', request: 'GET /api/posts/obs-viewer-missing', requestId: 'rid-obs-1', status: 404 },
    ],
    lines: [
      {
        source: 'the 404 (its failure line)',
        level: 'WARN',
        msg: 'error response',
        requestId: 'rid-obs-1',
        raw: '{"level":"WARN","msg":"error response","request_id":"rid-obs-1"}',
      },
      {
        source: 'the replay',
        raw: 'not json — an honest unparseable row',
      },
    ],
    note: 'two replays, one buffer — one id threads them',
  },
  reasoning: 'the log is data',
};

describe('obsViewerDtoFromJson', () => {
  it('builds the domain payload from a valid wire body', () => {
    const dto = obsViewerDtoFromJson(fullPayload);

    expect(dto.anatomy.line).toContain('http_request');
    expect(dto.anatomy.fields[0]?.key).toBe('time');
    expect(dto.levels.rows).toHaveLength(2);
    expect(dto.levels.rows[1]?.survivesAt).toEqual(['debug', 'info']);
    expect(dto.replay.requests[1]?.status).toBe(404);
    expect(dto.reasoning).toContain('data');
  });

  it('keeps optional correlation fields undefined when the line did not parse', () => {
    const dto = obsViewerDtoFromJson(fullPayload);

    expect(dto.replay.lines[0]?.level).toBe('WARN');
    expect(dto.replay.lines[0]?.requestId).toBe('rid-obs-1');
    expect(dto.replay.lines[1]?.level).toBeUndefined();
    expect(dto.replay.lines[1]?.requestId).toBeUndefined();
    expect(dto.replay.lines[1]?.raw).toContain('not json');
  });

  it('rejects a payload missing the level matrix', () => {
    const { levels: _levels, ...withoutLevels } = fullPayload;

    expect(() => obsViewerDtoFromJson(withoutLevels)).toThrow();
  });

  it('rejects an anatomy field without a note', () => {
    const broken = {
      ...fullPayload,
      anatomy: {
        ...fullPayload.anatomy,
        fields: [{ ...fullPayload.anatomy.fields[0], note: 42 }],
      },
    };

    expect(() => obsViewerDtoFromJson(broken)).toThrow();
  });

  it('rejects a replayed request whose status is not a number', () => {
    const broken = {
      ...fullPayload,
      replay: {
        ...fullPayload.replay,
        requests: [{ ...fullPayload.replay.requests[0], status: 'ok' }],
      },
    };

    expect(() => obsViewerDtoFromJson(broken)).toThrow();
  });
});
