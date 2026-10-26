import { describe, expect, it } from 'vitest';
import { middlewarePipelineDtoFromJson, toMiddlewarePipelineDemo } from './middleware-pipeline-dto';
import type { MiddlewarePipelineDemoData } from '../../domain/demo/middleware-pipeline';

/**
 * MiddlewarePipelineDTO spec — hostile-input discipline: only the exact
 * wire shape parses, anything else throws (and therefore surfaces
 * upstream as an invalid_response Failure, never as a guessed default).
 * `requestIDSeen: ''` is DATA (the beat fired before the request-id layer
 * assigned anything) — present-but-empty passes; `body` fields likewise
 * carry verbatim server bytes where empty would still be data.
 */

/**
 * The REAL builder output, captured from the Go side
 * (demos.BuildMiddlewarePipelinePayload, marshaled with encoding/json) —
 * the DTO is validated against the server's actual bytes, not a
 * hand-transcribed wish. Deterministic by construction: same builder
 * call, same bytes.
 */
const realBuilderOutput: MiddlewarePipelineDemoData = {
  chain: [
    {
      name: 'request-id',
      position: 0,
      role: 'assigns or honors one id per request, stamps the response header, and puts the id in the request context',
      mirrors: 'presentation.RequestID',
    },
    {
      name: 'request-logger',
      position: 1,
      role: 'one structured access-log line per request at completion, status and duration included',
      mirrors: 'presentation.RequestLogger',
    },
    {
      name: 'recover',
      position: 2,
      role: 'a deferred recover that converts handler panics into the 500 error envelope plus an error log',
      mirrors: 'presentation.Recover',
    },
    {
      name: 'terminal handler',
      position: 3,
      role: 'the mux and the endpoint: routing (topic 07) and the response itself — innermost, wrapped by everything',
      mirrors: 'http.ServeMux + the demo handlers',
    },
  ],
  trace: [
    {
      layer: 'request-id',
      phase: 'before',
      sequence: 0,
      requestIDSeen: '',
      note: 'no id in the context yet — assigning one is this layer’s job',
    },
    {
      layer: 'request-logger',
      phase: 'before',
      sequence: 1,
      requestIDSeen: 'middleware-demo-42',
      note: 'the id is already in the context — the access log can cite it on every line',
    },
    {
      layer: 'recover',
      phase: 'before',
      sequence: 2,
      requestIDSeen: 'middleware-demo-42',
      note: 'the deferred recover is armed — nothing happens unless the inner stack panics',
    },
    {
      layer: 'terminal handler',
      phase: 'handler',
      sequence: 3,
      requestIDSeen: 'middleware-demo-42',
      note: 'the endpoint runs exactly once, in the middle — every layer’s before-code has run, no after-code has',
    },
    {
      layer: 'recover',
      phase: 'after',
      sequence: 4,
      requestIDSeen: 'middleware-demo-42',
      note: 'no panic on this request — the after-beat simply unwinds',
    },
    {
      layer: 'request-logger',
      phase: 'after',
      sequence: 5,
      requestIDSeen: 'middleware-demo-42',
      note: 'the request is logged at completion — everything beneath this layer is inside the duration',
    },
    {
      layer: 'request-id',
      phase: 'after',
      sequence: 6,
      requestIDSeen: 'middleware-demo-42',
      note: 'the response header was stamped before the inner stack ran, so even the 500 envelope carries the id',
    },
  ],
  requestSideOrder: ['request-id', 'request-logger', 'recover', 'terminal handler'],
  responseSideOrder: ['terminal handler', 'recover', 'request-logger', 'request-id'],
  requestID: {
    headerName: 'x-request-id',
    inbound: 'middleware-demo-42',
    seenAtLayers: [
      'request-logger (before)',
      'recover (before)',
      'terminal handler (handler)',
      'recover (after)',
      'request-logger (after)',
      'request-id (after)',
    ],
    responseHeader: 'middleware-demo-42',
    sameIDEverywhere: true,
  },
  panicRecovery: {
    path: '/api/demo/middleware/pipeline',
    cause: 'assignment to entry in nil map',
    status: 500,
    body: '{"error":{"code":"internal","message":"Internal server error.","requestId":"middleware-demo-42"}}\n',
    loggedLine: 'level=ERROR msg="panic recovered" request_id=middleware-demo-42 panic="assignment to entry in nil map"',
    requestIDKept: true,
    trace: [
      {
        layer: 'request-id',
        phase: 'before',
        sequence: 0,
        requestIDSeen: '',
        note: 'no id in the context yet — assigning one is this layer’s job',
      },
      {
        layer: 'request-logger',
        phase: 'before',
        sequence: 1,
        requestIDSeen: 'middleware-demo-42',
        note: 'the id is already in the context — the access log can cite it on every line',
      },
      {
        layer: 'recover',
        phase: 'before',
        sequence: 2,
        requestIDSeen: 'middleware-demo-42',
        note: 'the deferred recover is armed — nothing happens unless the inner stack panics',
      },
      {
        layer: 'terminal handler',
        phase: 'handler',
        sequence: 3,
        requestIDSeen: 'middleware-demo-42',
        note: 'the deliberate bug: a write to a nil map mid-handler',
      },
      {
        layer: 'recover',
        phase: 'after',
        sequence: 4,
        requestIDSeen: 'middleware-demo-42',
        note: 'the panic was converted: 500 envelope written, error logged with the same id',
      },
      {
        layer: 'request-logger',
        phase: 'after',
        sequence: 5,
        requestIDSeen: 'middleware-demo-42',
        note: 'the request is logged at completion — everything beneath this layer is inside the duration',
      },
      {
        layer: 'request-id',
        phase: 'after',
        sequence: 6,
        requestIDSeen: 'middleware-demo-42',
        note: 'the response header was stamped before the inner stack ran, so even the 500 envelope carries the id',
      },
    ],
  },
  timeoutBeat: {
    wrapper: 'http.TimeoutHandler',
    budgetMs: 30,
    handlerSleepMs: 120,
    observedStatus: 503,
    body: '<html><head><title>Timeout</title></head><body><h1>Timeout</h1></body></html>',
    slowHandlerFinished: true,
    shape: 'the wrapper races the handler against the budget, derives a timeout context so r.Context() is cancelled, and answers 503 with its own body; the abandoned handler keeps running — its writes are discarded, and this payload waited (bounded) for it to finish',
  },
  reasoning:
    'One request, three middleware layers, one terminal handler: request-side code runs outermost-first because each layer wraps the next, the handler runs exactly once in the middle, and response-side code unwinds innermost-first because code after next.ServeHTTP only runs once the inner stack has finished — even when what comes back is a recovered panic. The trace is a real in-process run of real middleware over httptest, and the live proof rides on this very response: its x-request-id header was stamped by the production RequestID after this payload was built — check them agree with curl -i.',
};

describe('middlewarePipelineDtoFromJson', () => {
  it('parses the real builder output round-trip', () => {
    expect(middlewarePipelineDtoFromJson(realBuilderOutput)).toEqual(realBuilderOutput);
  });

  it('keeps the pre-assignment trace beat as data (requestIDSeen passes as "")', () => {
    const parsed = middlewarePipelineDtoFromJson(realBuilderOutput);

    expect(parsed.trace[0]?.requestIDSeen).toBe('');
    expect(parsed.panicRecovery.trace[0]?.requestIDSeen).toBe('');
  });

  it.each([
    ['null', null],
    ['an array', [realBuilderOutput]],
    ['a string', 'nope'],
    ['missing chain', { ...realBuilderOutput, chain: undefined }],
    ['chain not an array', { ...realBuilderOutput, chain: 'nope' }],
    ['a non-object chain layer', { ...realBuilderOutput, chain: ['nope'] }],
    ['missing trace', { ...realBuilderOutput, trace: undefined }],
    ['unknown phase vocabulary', { ...realBuilderOutput, trace: [{ ...realBuilderOutput.trace[1]!, phase: 'midway' }] }],
    ['missing requestIDSeen (null is not empty-data)', { ...realBuilderOutput, trace: [{ ...realBuilderOutput.trace[1]!, requestIDSeen: null }] }],
    ['missing requestID beat', { ...realBuilderOutput, requestID: undefined }],
    ['missing sameIDEverywhere', { ...realBuilderOutput, requestID: { ...realBuilderOutput.requestID, sameIDEverywhere: undefined } }],
    ['missing panicRecovery', { ...realBuilderOutput, panicRecovery: undefined }],
    ['panic status not a number', { ...realBuilderOutput, panicRecovery: { ...realBuilderOutput.panicRecovery, status: '500' } }],
    ['missing panic trace', { ...realBuilderOutput, panicRecovery: { ...realBuilderOutput.panicRecovery, trace: undefined } }],
    ['missing timeoutBeat', { ...realBuilderOutput, timeoutBeat: undefined }],
    ['budgetMs not finite', { ...realBuilderOutput, timeoutBeat: { ...realBuilderOutput.timeoutBeat, budgetMs: Number.POSITIVE_INFINITY } }],
    ['missing slowHandlerFinished', { ...realBuilderOutput, timeoutBeat: { ...realBuilderOutput.timeoutBeat, slowHandlerFinished: undefined } }],
    ['missing requestSideOrder', { ...realBuilderOutput, requestSideOrder: undefined }],
    ['an empty layer name in requestSideOrder', { ...realBuilderOutput, requestSideOrder: ['request-id', ''] }],
    ['missing reasoning', { ...realBuilderOutput, reasoning: undefined }],
  ])('throws for %s', (_label, body) => {
    expect(() => middlewarePipelineDtoFromJson(body)).toThrow();
  });

  it('drops unknown wire keys (whitelist, not pass-through)', () => {
    const parsed = middlewarePipelineDtoFromJson({
      ...realBuilderOutput,
      extra: 'ignored',
      trace: realBuilderOutput.trace.map((event) => ({ ...event, sneaky: true })),
    });

    expect(parsed).toEqual(realBuilderOutput);
  });
});

describe('toMiddlewarePipelineDemo', () => {
  it('maps the DTO onto the domain model (identity while 1:1)', () => {
    expect(toMiddlewarePipelineDemo(realBuilderOutput)).toBe(realBuilderOutput);
  });
});
