import { describe, expect, it } from 'vitest';
import { httpHeadersDtoFromJson, toHttpHeadersDemo } from './http-headers-dto';
import {
  REDACTED_HEADER_VALUE,
  type HttpHeadersDemoData,
} from '../../domain/demo/http-headers';

/**
 * HttpHeadersDTO spec — hostile-input discipline: only the exact wire
 * shape parses, anything else throws (and therefore surfaces upstream as
 * an invalid_response Failure, never as a guessed default). Optional wire
 * keys (`query`, `contentType`, `note`) are OMITTED by the server when
 * absent — they must land as undefined, never as null or ''. A header
 * value of `""` is data (a valueless header), not absence — it passes.
 */

const wellFormed: HttpHeadersDemoData = {
  method: 'GET',
  path: '/api/demo/http/headers',
  query: 'verbose=1',
  proto: 'HTTP/1.1',
  requestID: 'req-17',
  contentType: 'application/json',
  headers: [
    {
      name: 'User-Agent',
      value: 'curl/8.7.1',
      scope: 'end-to-end',
      role: 'identity',
      note: 'identifies the calling software; log it, rarely branch on it',
    },
    { name: 'Authorization', value: REDACTED_HEADER_VALUE, scope: 'end-to-end', role: 'security' },
    { name: 'Connection', value: 'keep-alive', scope: 'hop-by-hop', role: 'transport' },
  ],
  hopByHop: [
    'Connection',
    'Keep-Alive',
    'Proxy-Authenticate',
    'Proxy-Authorization',
    'Te',
    'Trailer',
    'Transfer-Encoding',
    'Upgrade',
  ],
};

describe('httpHeadersDtoFromJson', () => {
  it('parses a well-formed payload round-trip', () => {
    expect(httpHeadersDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('keeps an empty header value as data (a valueless header echoes as "")', () => {
    const parsed = httpHeadersDtoFromJson({
      ...wellFormed,
      headers: [{ name: 'X-Empty', value: '', scope: 'end-to-end', role: 'extension' }],
    });

    expect(parsed.headers[0]?.value).toBe('');
  });

  it('keeps optional facts absent (omitted on the wire, undefined here)', () => {
    const minimal = {
      method: 'GET',
      path: '/',
      proto: 'HTTP/1.1',
      requestID: 'req-1',
      headers: [{ name: 'X-Custom', value: 'v', scope: 'end-to-end', role: 'extension' }],
      hopByHop: ['Connection'],
    };

    const parsed = httpHeadersDtoFromJson(minimal);

    expect(parsed.query).toBeUndefined();
    expect(parsed.contentType).toBeUndefined();
    expect(parsed.headers[0]?.note).toBeUndefined();
  });

  it.each([
    ['null', null],
    ['an array', [wellFormed]],
    ['a string', 'nope'],
    ['missing method', { ...wellFormed, method: undefined }],
    ['empty method', { ...wellFormed, method: '' }],
    ['missing path', { ...wellFormed, path: undefined }],
    ['missing proto', { ...wellFormed, proto: undefined }],
    ['missing requestID', { ...wellFormed, requestID: undefined }],
    ['missing headers', { ...wellFormed, headers: undefined }],
    ['headers not an array', { ...wellFormed, headers: 'nope' }],
    ['a non-object echoed header', { ...wellFormed, headers: ['nope'] }],
    ['missing scope', { ...wellFormed, headers: [{ name: 'Accept', value: '*/*', role: 'negotiation' }] }],
    ['missing role', { ...wellFormed, headers: [{ name: 'Accept', value: '*/*', scope: 'end-to-end' }] }],
    ['unknown scope vocabulary', { ...wellFormed, headers: [{ name: 'Accept', value: '*/*', scope: 'cosmic', role: 'negotiation' }] }],
    ['unknown role vocabulary', { ...wellFormed, headers: [{ name: 'Accept', value: '*/*', scope: 'end-to-end', role: 'magic' }] }],
    ['missing value', { ...wellFormed, headers: [{ name: 'Accept', scope: 'end-to-end', role: 'negotiation' }] }],
    ['empty header name', { ...wellFormed, headers: [{ name: '', value: 'v', scope: 'end-to-end', role: 'other' }] }],
    ['null query (null is not omission)', { ...wellFormed, query: null }],
    ['missing hopByHop', { ...wellFormed, hopByHop: undefined }],
    ['a non-string hop-by-hop entry', { ...wellFormed, hopByHop: ['Connection', 42] }],
  ])('throws for %s', (_label, body) => {
    expect(() => httpHeadersDtoFromJson(body)).toThrow();
  });

  it('drops unknown wire keys (whitelist, not pass-through)', () => {
    const parsed = httpHeadersDtoFromJson({
      ...wellFormed,
      extra: 'ignored',
      headers: wellFormed.headers.map((header) => ({ ...header, sneaky: true })),
    });

    expect(parsed).toEqual(wellFormed);
  });
});

describe('toHttpHeadersDemo', () => {
  it('maps the DTO onto the domain model (identity while 1:1)', () => {
    expect(toHttpHeadersDemo(wellFormed)).toBe(wellFormed);
  });
});
