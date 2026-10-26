import { describe, expect, it } from 'vitest';
import { reqresInspectorDtoFromJson, toReqresInspectorDemo } from './reqres-inspector-dto';
import type { ReqresInspectorDemoData } from '../../domain/demo/reqres-inspector';

/**
 * ReqresInspectorDTO spec — hostile-input discipline: only the exact wire
 * shape parses, anything else throws (and therefore surfaces upstream as
 * an invalid_response Failure, never as a guessed default). Optional wire
 * keys (`contentType`, `error`, `field`, …) are OMITTED by the server
 * when absent — they must land as undefined, never null. `decoded` and
 * `body` of "" are data (a truncated body decodes nothing), not absence —
 * they pass as requiredText.
 */

/** A structurally complete wire payload (mirrors the Go builder's output). */
const wellFormed: ReqresInspectorDemoData = {
  request: {
    method: 'GET',
    path: '/api/demo/reqres/inspector',
    contentType: 'application/json',
    bodyBytes: 38,
    bodyLimit: 4096,
    bodyLimitExceeded: true,
  },
  discipline: {
    decoder: 'json.NewDecoder over the request body',
    unknownFields: 'rejected — DisallowUnknownFields',
    trailing: 'exactly one JSON value — a second Decode must return io.EOF',
    firstError: 'reported verbatim; decoding stops at the first offense',
  },
  exhibits: [
    {
      label: 'valid body',
      body: '{"title":"Read the body as a stream","tags":["request","json"]}',
      outcome: 'accepted',
      decoded: 'title="Read the body as a stream" tags=["request" "json"]',
    },
    {
      label: 'truncated mid-array',
      body: '{"title":"complete","tags":["a"',
      outcome: 'rejected',
      error: 'unexpected EOF',
      errorType: '*errors.errorString',
      unexpectedEOF: true,
      decoded: 'title="" tags=[]',
    },
    {
      label: 'wrong-typed field',
      body: '{"title":42,"tags":[]}',
      outcome: 'rejected',
      error: 'json: cannot unmarshal number into Go struct field reqresBody.title of type string',
      errorType: '*json.UnmarshalTypeError',
      field: 'title',
      decoded: 'title="" tags=[]',
    },
  ],
  liveBody: {
    sent: true,
    body: '{"title":"sent along","tags":["live"]}',
    outcome: 'accepted',
    decoded: 'title="sent along" tags=["live"]',
  },
};

describe('reqresInspectorDtoFromJson', () => {
  it('parses a well-formed payload round-trip', () => {
    expect(reqresInspectorDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('keeps empty decoded state as data (truncation decodes nothing)', () => {
    const parsed = reqresInspectorDtoFromJson(wellFormed);

    expect(parsed.exhibits[1]?.decoded).toBe('title="" tags=[]');
    expect(parsed.exhibits[1]?.unexpectedEOF).toBe(true);
  });

  it('keeps optional facts absent (omitted on the wire, undefined here)', () => {
    const minimal = {
      request: {
        method: 'GET',
        path: '/api/demo/reqres/inspector',
        bodyBytes: 0,
        bodyLimit: 4096,
      },
      discipline: {
        decoder: 'json.NewDecoder over the request body',
        unknownFields: 'rejected — DisallowUnknownFields',
        trailing: 'exactly one JSON value — a second Decode must return io.EOF',
        firstError: 'reported verbatim; decoding stops at the first offense',
      },
      exhibits: [
        {
          label: 'valid body',
          body: '{"title":"t","tags":[]}',
          outcome: 'accepted',
          decoded: 'title="t" tags=[]',
        },
      ],
      liveBody: { sent: false, outcome: 'none' },
    };

    const parsed = reqresInspectorDtoFromJson(minimal);

    expect(parsed.request.contentType).toBeUndefined();
    expect(parsed.request.bodyLimitExceeded).toBeUndefined();
    expect(parsed.exhibits[0]?.error).toBeUndefined();
    expect(parsed.exhibits[0]?.unexpectedEOF).toBeUndefined();
    expect(parsed.liveBody.body).toBeUndefined();
    expect(parsed.liveBody.decoded).toBeUndefined();
  });

  it.each([
    ['null', null],
    ['an array', [wellFormed]],
    ['a string', 'nope'],
    ['missing request', { ...wellFormed, request: undefined }],
    ['missing discipline', { ...wellFormed, discipline: undefined }],
    ['missing exhibits', { ...wellFormed, exhibits: undefined }],
    ['exhibits not an array', { ...wellFormed, exhibits: 'nope' }],
    ['a non-object exhibit', { ...wellFormed, exhibits: ['nope'] }],
    ['missing liveBody', { ...wellFormed, liveBody: undefined }],
    ['missing outcome', { ...wellFormed, exhibits: [{ label: 'l', body: 'b', decoded: 'd' }] }],
    ['unknown outcome vocabulary', { ...wellFormed, exhibits: [{ ...wellFormed.exhibits[0], outcome: 'maybe' }] }],
    ['unknown live outcome vocabulary', { ...wellFormed, liveBody: { sent: false, outcome: 'void' } }],
    ['missing decoded', { ...wellFormed, exhibits: [{ ...wellFormed.exhibits[0], decoded: undefined }] }],
    ['decoded null (null is not omission)', { ...wellFormed, exhibits: [{ ...wellFormed.exhibits[0], decoded: null }] }],
    ['missing bodyBytes', { ...wellFormed, request: { ...wellFormed.request, bodyBytes: undefined } }],
    ['NaN bodyLimit', { ...wellFormed, request: { ...wellFormed.request, bodyLimit: Number.NaN } }],
    ['missing sent', { ...wellFormed, liveBody: { outcome: 'none' } }],
    ['sent not a boolean', { ...wellFormed, liveBody: { ...wellFormed.liveBody, sent: 1 } }],
    ['unexpectedEOF not a boolean', { ...wellFormed, exhibits: [{ ...wellFormed.exhibits[0], unexpectedEOF: 'yes' }] }],
    ['null error (null is not omission)', { ...wellFormed, exhibits: [{ ...wellFormed.exhibits[0], error: null }] }],
  ])('throws for %s', (_label, body) => {
    expect(() => reqresInspectorDtoFromJson(body)).toThrow();
  });

  it('drops unknown wire keys (whitelist, not pass-through)', () => {
    const parsed = reqresInspectorDtoFromJson({
      ...wellFormed,
      extra: 'ignored',
      exhibits: wellFormed.exhibits.map((exhibit) => ({ ...exhibit, sneaky: true })),
    });

    expect(parsed).toEqual(wellFormed);
  });
});

describe('toReqresInspectorDemo', () => {
  it('maps the DTO onto the domain model (identity while 1:1)', () => {
    expect(toReqresInspectorDemo(wellFormed)).toBe(wellFormed);
  });
});
