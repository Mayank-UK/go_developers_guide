import { describe, expect, it } from 'vitest';
import { reqresMethodsDtoFromJson, toReqresMethodsDemo } from './reqres-methods-dto';
import type { ReqresMethodsDemoData } from '../../domain/demo/reqres-methods';

/**
 * ReqresMethodsDTO spec — hostile-input discipline: only the exact wire
 * shape parses, anything else throws (and therefore surfaces upstream as
 * an invalid_response Failure, never as a guessed default). A method
 * row's `body` of "" is data (204/HEAD/OPTIONS carry none by rule) and
 * passes as requiredText; statuses must be real HTTP statuses.
 */

/** A structurally complete wire payload (mirrors the Go builder's output). */
const wellFormed: ReqresMethodsDemoData = {
  resource: '/api/demo/reqres/methods/exhibit',
  allow: 'GET, HEAD, OPTIONS, POST, PUT, PATCH, DELETE',
  methods: [
    {
      method: 'GET',
      status: 200,
      statusText: 'OK',
      headers: [
        { name: 'Content-Type', value: 'application/json' },
        { name: 'Cache-Control', value: 'no-store' },
        { name: 'Set-Cookie', value: 'reqres_visit=1; Path=/api/demo/reqres; HttpOnly; SameSite=Lax' },
        { name: 'X-Reqres-Method', value: 'GET' },
      ],
      bodyShape: "one JSON object — the resource's current state",
      body: '{"resource":"methods-exhibit","state":"steady"}',
    },
    {
      method: 'PATCH',
      status: 204,
      statusText: 'No Content',
      headers: [{ name: 'X-Reqres-Method', value: 'PATCH' }],
      bodyShape: 'no body — 204 means the headers are the whole answer',
      body: '',
    },
    {
      method: 'TRACE',
      status: 405,
      statusText: 'Method Not Allowed',
      headers: [
        { name: 'Allow', value: 'GET, HEAD, OPTIONS, POST, PUT, PATCH, DELETE' },
        { name: 'X-Reqres-Method', value: 'TRACE' },
      ],
      bodyShape: 'one JSON object — the error note; Allow states what IS served',
      body: '{"error":"TRACE","served":"GET, HEAD, OPTIONS, POST, PUT, PATCH, DELETE"}',
    },
  ],
  sse: {
    contentType: 'text/event-stream',
    events: [
      { name: 'beat', data: '{"sequence":1,"state":"tick"}' },
      { name: 'beat', data: '{"sequence":2,"state":"tick"}' },
      { name: 'beat', data: '{"sequence":3,"state":"tick"}' },
    ],
    flushes: 3,
    framing: 'each event block ends with a blank line — the delimiter a client parser waits for',
  },
};

describe('reqresMethodsDtoFromJson', () => {
  it('parses a well-formed payload round-trip', () => {
    expect(reqresMethodsDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('keeps empty bodies as data (204/HEAD/OPTIONS carry none by rule)', () => {
    const parsed = reqresMethodsDtoFromJson(wellFormed);

    expect(parsed.methods[1]?.body).toBe('');
    expect(parsed.methods[1]?.status).toBe(204);
  });

  it.each([
    ['null', null],
    ['an array', [wellFormed]],
    ['a string', 'nope'],
    ['missing resource', { ...wellFormed, resource: undefined }],
    ['missing allow', { ...wellFormed, allow: undefined }],
    ['missing methods', { ...wellFormed, methods: undefined }],
    ['methods not an array', { ...wellFormed, methods: 'nope' }],
    ['a non-object method row', { ...wellFormed, methods: ['nope'] }],
    ['missing status', { ...wellFormed, methods: [{ ...wellFormed.methods[0], status: undefined }] }],
    ['a non-HTTP status', { ...wellFormed, methods: [{ ...wellFormed.methods[0], status: 42 }] }],
    ['a fractional status', { ...wellFormed, methods: [{ ...wellFormed.methods[0], status: 200.5 }] }],
    ['missing statusText', { ...wellFormed, methods: [{ ...wellFormed.methods[0], statusText: undefined }] }],
    ['missing headers', { ...wellFormed, methods: [{ ...wellFormed.methods[0], headers: undefined }] }],
    ['a non-object observed header', { ...wellFormed, methods: [{ ...wellFormed.methods[0], headers: ['x'] }] }],
    ['an empty observed header name', { ...wellFormed, methods: [{ ...wellFormed.methods[0], headers: [{ name: '', value: 'v' }] }] }],
    ['missing body', { ...wellFormed, methods: [{ ...wellFormed.methods[0], body: undefined }] }],
    ['missing bodyShape', { ...wellFormed, methods: [{ ...wellFormed.methods[0], bodyShape: undefined }] }],
    ['missing sse', { ...wellFormed, sse: undefined }],
    ['missing sse events', { ...wellFormed, sse: { ...wellFormed.sse, events: undefined } }],
    ['a non-object sse event', { ...wellFormed, sse: { ...wellFormed.sse, events: ['x'] } }],
    ['a negative flush count', { ...wellFormed, sse: { ...wellFormed.sse, flushes: -1 } }],
    ['a fractional flush count', { ...wellFormed, sse: { ...wellFormed.sse, flushes: 2.5 } }],
    ['missing framing', { ...wellFormed, sse: { ...wellFormed.sse, framing: undefined } }],
  ])('throws for %s', (_label, body) => {
    expect(() => reqresMethodsDtoFromJson(body)).toThrow();
  });

  it('drops unknown wire keys (whitelist, not pass-through)', () => {
    const parsed = reqresMethodsDtoFromJson({
      ...wellFormed,
      extra: 'ignored',
      methods: wellFormed.methods.map((row) => ({ ...row, sneaky: true })),
    });

    expect(parsed).toEqual(wellFormed);
  });
});

describe('toReqresMethodsDemo', () => {
  it('maps the DTO onto the domain model (identity while 1:1)', () => {
    expect(toReqresMethodsDemo(wellFormed)).toBe(wellFormed);
  });
});
