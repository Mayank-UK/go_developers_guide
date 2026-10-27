import { describe, expect, it } from 'vitest';
import { obsTraceDtoFromJson } from './obs-trace-dto';

/**
 * ObsTraceDTO spec — the fromJson whitelist: a full payload builds the
 * domain shape (acceptance panel, pre-order tree, malformed table);
 * every violation (missing tree, wrong types) throws so parseDto can
 * upgrade it to an InvalidResponseException.
 */

const fullPayload = {
  accepted: {
    inbound: '00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01',
    inboundOrigin: 'the request\'s own header, adopted verbatim',
    version: '00',
    traceId: '4bf92f3577b34da6a3ce929d0e0e4736',
    parentId: '00f067aa0ba902b7',
    flags: '01',
    sampled: true,
    tracestate: 'go=guide',
    responseEcho: '00-4bf92f3577b34da6a3ce929d0e0e4736-96a123b077498b49-01',
    note: 'the response carries the server span',
  },
  tree: [
    {
      name: 'GET /api/posts (traced walk)',
      layer: 'middleware',
      spanId: '96a123b077498b49',
      parentId: '00f067aa0ba902b7',
      traceId: '4bf92f3577b34da6a3ce929d0e0e4736',
      depth: 0,
      startOffsetMs: 0,
      durationMs: 1.4,
      note: 'the graft point',
    },
    {
      name: 'InMemoryRepository.List',
      layer: 'data',
      spanId: 'c1f0e2d3b4a59687',
      parentId: '96a123b077498b49',
      traceId: '4bf92f3577b34da6a3ce929d0e0e4736',
      depth: 3,
      startOffsetMs: 0.9,
      durationMs: 0.2,
      note: 'the leaf that answered',
    },
  ],
  malformed: [
    {
      header: '00-00000000000000000000000000000000-00f067aa0ba902b7-01',
      problem: 'traceparent …: trace-id must not be all zeros',
      handledAs: 'rejected; fresh root; debug log',
    },
  ],
  sdk: 'what the OpenTelemetry SDK adds: samplers, exporters, resources',
  reasoning: 'one header became one tree',
};

describe('obsTraceDtoFromJson', () => {
  it('builds the domain payload from a valid wire body', () => {
    const dto = obsTraceDtoFromJson(fullPayload);

    expect(dto.accepted.traceId).toBe('4bf92f3577b34da6a3ce929d0e0e4736');
    expect(dto.accepted.sampled).toBe(true);
    expect(dto.accepted.tracestate).toBe('go=guide');
    expect(dto.tree).toHaveLength(2);
    expect(dto.tree[0]?.depth).toBe(0);
    expect(dto.tree[1]?.durationMs).toBe(0.2);
    expect(dto.malformed[0]?.problem).toContain('all zeros');
    expect(dto.sdk).toContain('OpenTelemetry');
  });

  it('keeps tracestate optional (absent when the caller sent none)', () => {
    const { tracestate: _state, ...accepted } = fullPayload.accepted;
    const dto = obsTraceDtoFromJson({ ...fullPayload, accepted: { ...accepted } });

    expect(dto.accepted.tracestate).toBeUndefined();
  });

  it('rejects a payload missing the span tree', () => {
    const { tree: _tree, ...withoutTree } = fullPayload;

    expect(() => obsTraceDtoFromJson(withoutTree)).toThrow();
  });

  it('rejects a span row whose depth is not a number', () => {
    const broken = {
      ...fullPayload,
      tree: [{ ...fullPayload.tree[0], depth: 'zero' }],
    };

    expect(() => obsTraceDtoFromJson(broken)).toThrow();
  });

  it('rejects a non-boolean sampled flag', () => {
    const broken = {
      ...fullPayload,
      accepted: { ...fullPayload.accepted, sampled: 'yes' },
    };

    expect(() => obsTraceDtoFromJson(broken)).toThrow();
  });
});
