import { vi } from 'vitest';

/**
 * URL-dispatching fetch stub for jsdom specs: each route matches an exact
 * request URL and answers as JSON (body), or as a broken-JSON response whose
 * json() rejects (the non-JSON error path). Any unstubbed URL answers with a
 * 404 error envelope so a missing mock fails loudly instead of hanging.
 * Callers must `vi.unstubAllGlobals()` in afterEach — this file deliberately
 * does not own test lifecycle hooks.
 */

export interface StubRoute {
  url: string;
  /** Match only requests with this method (uppercase verb); omit to match any. */
  method?: string;
  status?: number;
  /** JSON body — the stub answers with json(). */
  body?: unknown;
  /**
   * Response headers for the stand-in (plain record, lowercase keys — the
   * guards collect both this shape and a real Headers instance).
   */
  headers?: Record<string, string>;
  /**
   * Raw text body (e.g. an NDJSON stream) — the stub answers with text().
   * json() rejects, so specs exercising text-consuming adapters stay honest.
   */
  rawBody?: string;
  /** Answers with a response whose json() rejects (the non-JSON error path). */
  brokenJson?: boolean;
  /** Delay the answer by ms — for stale-response race tests. */
  delayMs?: number;
}

export function stubApiFetch(routes: readonly StubRoute[]): void {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === 'string' ? input : input.toString();
      const method = init?.method ?? 'GET';
      const route = routes.find(
        (candidate) =>
          candidate.url === url &&
          (candidate.method === undefined || candidate.method === method),
      );
      if (route === undefined) {
        return jsonResponse(
          {
            error: {
              code: 'not_found',
              message: `no fetch stub for ${method} ${url}`,
              requestId: 'stub',
            },
          },
          404,
        );
      }
      if (route.delayMs !== undefined) {
        await new Promise((resolve) => setTimeout(resolve, route.delayMs));
      }
      if (route.rawBody !== undefined) {
        return textResponse(route.rawBody, route.status ?? 200, route.headers ?? {});
      }
      if (route.brokenJson === true) {
        return brokenJsonResponse(route.status ?? 200);
      }
      return jsonResponse(route.body, route.status ?? 200, route.headers ?? {});
    }),
  );
}

/** A fetch that never leaves the machine: exercises the status-0 error path. */
export function stubNetworkFailure(): void {
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => {
      throw new TypeError('Failed to fetch');
    }),
  );
}

/**
 * Minimal Response stand-ins: the API layer only reads `ok`, `status`,
 * `headers` (duck-typed: a Headers instance or a plain record), `json()`,
 * `text()`, and (duck-typed) `body`, so plain objects keep the stub
 * independent of whether the test environment provides the real
 * Response global.
 */
function jsonResponse(body: unknown, status: number, headers: Record<string, string> = {}) {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers,
    json: async () => body,
    text: async () => JSON.stringify(body),
  };
}

/**
 * Response carrying raw text (an NDJSON stream fixture) — json() rejects.
 * Carries headers like the JSON stand-in does: raw-round adapters (binary
 * downloads, streams) read their contract from headers.
 */
function textResponse(raw: string, status: number, headers: Record<string, string> = {}) {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers,
    json: async () => {
      throw new SyntaxError('Unexpected token in JSON');
    },
    text: async () => raw,
  };
}

/** Response whose body is not JSON (json() rejects) at the given status. */
function brokenJsonResponse(status: number) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => {
      throw new SyntaxError('Unexpected token < in JSON');
    },
  };
}
