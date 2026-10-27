import { describe, expect, it } from 'vitest';
import { debugStallDtoFromJson } from './debug-stall-dto';

/**
 * DebugStallDTO spec — the fromJson whitelist: a full payload builds
 * the domain shape (workload, capture provenance, the stalled stack
 * with frames, the contrast row, the live endpoints); every violation
 * (missing panels, wrong types, a stack without frames) throws so
 * parseDto can upgrade it to an InvalidResponseException.
 */

const fullPayload = {
  mode: 'runtime-derived: the stalled stack below was captured from THIS process',
  workload: {
    workers: 5,
    healthyDone: 4,
    wedge: 'one worker of the set blocks forever on a channel receive nobody serves',
  },
  capture: {
    profile: 'goroutine',
    goroutinesSeen: 9,
    captureMs: 0.31,
    captureTries: 1,
    curl: "curl -s 'localhost:8100/internal/debug/pprof/goroutine?debug=2'",
  },
  stalled: {
    id: 42,
    state: 'chan receive',
    wait: '1 minutes',
    frames: [
      {
        function: 'guide/internal/domain/demos.debugStallWedge(0x0)',
        location: 'server/internal/domain/demos/debug_stall.go:118',
      },
    ],
    createdBy: 'guide/internal/domain/demos.RunDebugStall',
    createdAt: 'server/internal/domain/demos/debug_stall.go:157',
  },
  contrast: {
    id: 1,
    state: 'running',
    frames: [
      {
        function: 'guide/internal/domain/demos.RunDebugStall(0x0)',
        location: 'server/internal/domain/demos/debug_stall.go:201',
      },
    ],
  },
  endpoints: [
    {
      path: '/internal/debug/pprof/goroutine?debug=2',
      profile: 'goroutine',
      note: 'every live goroutine with its full stack',
    },
  ],
  reasoning: 'the runtime names the wedge — id, state, frames, and the created-by chain',
};

describe('debugStallDtoFromJson', () => {
  it('builds the domain payload from a valid wire body', () => {
    const dto = debugStallDtoFromJson(fullPayload);

    expect(dto.workload.workers).toBe(5);
    expect(dto.workload.healthyDone).toBe(4);

    expect(dto.capture.profile).toBe('goroutine');
    expect(dto.capture.goroutinesSeen).toBe(9);

    expect(dto.stalled.id).toBe(42);
    expect(dto.stalled.state).toBe('chan receive');
    expect(dto.stalled.wait).toBe('1 minutes');
    expect(dto.stalled.frames).toHaveLength(1);
    expect(dto.stalled.frames[0]?.location).toBe(
      'server/internal/domain/demos/debug_stall.go:118',
    );
    expect(dto.stalled.createdBy).toContain('RunDebugStall');

    // Optional fields absent on the contrast row read as undefined.
    expect(dto.contrast.wait).toBeUndefined();
    expect(dto.contrast.createdBy).toBeUndefined();

    expect(dto.endpoints).toHaveLength(1);
    expect(dto.endpoints[0]?.path).toBe('/internal/debug/pprof/goroutine?debug=2');
  });

  it('rejects a payload missing the stalled stack', () => {
    const { stalled: _stalled, ...withoutStalled } = fullPayload;

    expect(() => debugStallDtoFromJson(withoutStalled)).toThrow();
  });

  it('rejects a stalled stack whose frames are not an array', () => {
    expect(() =>
      debugStallDtoFromJson({
        ...fullPayload,
        stalled: { ...fullPayload.stalled, frames: 'none' },
      }),
    ).toThrow();
  });

  it('rejects a workload with a non-number worker count', () => {
    expect(() =>
      debugStallDtoFromJson({
        ...fullPayload,
        workload: { ...fullPayload.workload, workers: 'five' },
      }),
    ).toThrow();
  });

  it('rejects a frame without its source location', () => {
    expect(() =>
      debugStallDtoFromJson({
        ...fullPayload,
        stalled: {
          ...fullPayload.stalled,
          frames: [{ function: 'guide/internal/domain/demos.debugStallWedge(0x0)' }],
        },
      }),
    ).toThrow();
  });

  it('rejects a non-object body outright', () => {
    expect(() => debugStallDtoFromJson('stalled')).toThrow();
  });
});
