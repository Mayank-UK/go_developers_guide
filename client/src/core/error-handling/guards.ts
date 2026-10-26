import type { AppConfig } from '../config/app-config';
import {
  AppException,
  InvalidResponseException,
  NetworkException,
  ServerException,
} from './exceptions';
import {
  Failure,
  NetworkFailure,
  ServerFailure,
  UnknownFailure,
} from './failures';
import { failed, success, type Result } from '../result';

/**
 * Guards — the heart of the error pipeline (express-guide client shape,
 * carried over by the spring guide; trimmed to this client's fetch-based data
 * layer). Nothing past a repository ever throws for an expected failure;
 * everything arrives as a typed Failure wrapped in a Result.
 *
 *  1. `guardedGetJson`    : the shared fetch pipeline. Every transport failure
 *                           is upgraded to a typed AppException:
 *                             fetch rejected           → NetworkException (status 0)
 *                             body is not JSON         → ServerException (unknown_error)
 *                             non-2xx + error envelope → ServerException (envelope
 *                                                      code/message/requestId/status)
 *  2. `parseDto`          : runs a DTO `fromJson` and upgrades a shape
 *                           violation to InvalidResponseException.
 *  3. `guardedApiCall`    : wraps a data-source call, re-classifying any
 *                           leftover thrown value as a typed AppException.
 *  4. `safeRepositoryCall`: the repository's guarantee — a thrown AppException
 *                           maps to a Failure in a Failed Result; the method
 *                           itself never throws.
 *
 * The module also owns the raw-round primitives the few adapters whose
 * outcomes cannot ride the JSON pipeline (binary bodies, 304/429 rounds,
 * NDJSON streams) share so their failures land in the same taxonomy:
 * `collectHeaders`, `readHeader`, `errorEnvelopeException`,
 * `responseEnvelopeException`.
 */

export interface JsonResponse {
  readonly body: unknown;
  readonly status: number;
  /**
   * Response headers, names lowercased. Real fetch provides a `Headers`
   * instance; the fetch stub answers with a plain record (duck-typed) —
   * both are collected, so Location on 201s is readable either way.
   */
  readonly headers: Record<string, string>;
}

/** GET {apiBaseUrl}{path} as JSON with every transport failure guarded. */
export async function guardedGetJson(
  config: AppConfig,
  path: string,
): Promise<JsonResponse> {
  let response: Response;
  try {
    response = await fetch(`${config.apiBaseUrl}${path}`, {
      headers: { accept: 'application/json' },
    });
  } catch (error) {
    throw new NetworkException('Could not reach the server.', { cause: error });
  }
  return processJsonResponse(response);
}

/** A JSON request the caller shapes: method, extra headers, optional body. */
export interface JsonRequest {
  readonly method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  /** Extra request headers (an invalid header name lets fetch throw → Network). */
  readonly headers?: Record<string, string>;
  /** Serialized request body; omit for a bodyless request. */
  readonly body?: string;
}

/** Send {apiBaseUrl}{path} as JSON with every transport failure guarded. */
export async function guardedSendJson(
  config: AppConfig,
  path: string,
  request: JsonRequest,
): Promise<JsonResponse> {
  let response: Response;
  try {
    response = await fetch(`${config.apiBaseUrl}${path}`, {
      method: request.method,
      headers: { accept: 'application/json', ...request.headers },
      ...(request.body === undefined ? {} : { body: request.body }),
    });
  } catch (error) {
    throw new NetworkException('Could not reach the server.', { cause: error });
  }
  return processJsonResponse(response);
}

/** Shared tail of the JSON pipeline: parse, guard non-2xx envelopes. */
async function processJsonResponse(response: Response): Promise<JsonResponse> {
  if (response.status === 204 || response.status === 205) {
    // No Content by definition — there is no body to parse. The caller's
    // DTO decides what a null body means (204 is a success, not an error).
    return { body: null, status: response.status, headers: collectHeaders(response) };
  }
  let body: unknown;
  try {
    body = await response.json();
  } catch (error) {
    throw unknownStatusException(response.status, error);
  }

  if (!response.ok) {
    throw errorEnvelopeException(body, response.status);
  }

  return { body, status: response.status, headers: collectHeaders(response) };
}

/**
 * Collect response headers as a lowercase-name record. Tolerant of both a
 * real `Headers` instance (forEach) and the fetch stub's plain records —
 * the stub's duck-typed stand-ins deliberately avoid the Response global.
 */
export function collectHeaders(response: Response): Record<string, string> {
  const collected: Record<string, string> = {};
  const headers: unknown = response.headers;
  if (headers !== null && typeof headers === 'object') {
    if (typeof (headers as Headers).forEach === 'function') {
      (headers as Headers).forEach((value, key) => {
        collected[key] = value;
      });
    } else {
      for (const [key, value] of Object.entries(headers as Record<string, unknown>)) {
        if (typeof value === 'string') collected[key] = value;
      }
    }
  }
  return collected;
}

/**
 * Read one header, case-insensitively, from a `Headers` instance or a plain
 * lowercase record (the fetch stub's stand-in). The raw-round adapters share
 * this for their outcome shaping; `undefined` means the header was absent.
 */
export function readHeader(
  headers: Response['headers'] | Record<string, string>,
  name: string,
): string | undefined {
  const lower = name.toLowerCase();
  const candidate: unknown = headers;
  if (
    candidate !== null &&
    typeof candidate === 'object' &&
    typeof (candidate as Headers).get === 'function'
  ) {
    const value = (candidate as Headers).get(lower);
    return value === null ? undefined : value;
  }
  const record = candidate as Record<string, string>;
  const key = Object.keys(record).find((entry) => entry.toLowerCase() === lower);
  return key === undefined ? undefined : record[key];
}

/**
 * Parse the server's `{ error: { code, message, requestId, …details } }`
 * envelope. Only the canonical code/message/requestId survive — extra merged
 * detail keys are ignored. Non-envelope bodies degrade to unknown_error with a
 * status-derived message.
 */
export function errorEnvelopeException(body: unknown, status: number): ServerException {
  if (isRecord(body) && isRecord(body.error)) {
    const code = typeof body.error.code === 'string' ? body.error.code : 'unknown_error';
    const message =
      typeof body.error.message === 'string'
        ? body.error.message
        : `Request failed with status ${String(status)}.`;
    const requestId =
      typeof body.error.requestId === 'string' ? body.error.requestId : undefined;
    return new ServerException(message, { code, status, requestId });
  }
  return unknownStatusException(status, undefined);
}

/**
 * The response-level form of {@link errorEnvelopeException}: read the body as
 * JSON (an unparseable body degrades to the status-derived fallback) and
 * classify it into the shared taxonomy. Raw-round adapters whose outcomes
 * cannot ride the JSON pipeline (binary bodies, 204/304/429 rounds) still
 * guard their failures through this — one envelope, one taxonomy.
 */
export async function responseEnvelopeException(response: Response): Promise<ServerException> {
  let body: unknown = null;
  try {
    body = await response.json();
  } catch {
    body = null;
  }
  return errorEnvelopeException(body, response.status);
}

function unknownStatusException(status: number, cause: unknown): ServerException {
  return new ServerException(`Request failed with status ${String(status)}.`, {
    code: 'unknown_error',
    status,
    cause,
  });
}

/**
 * Validate a (2xx) JSON body into a DTO: a `fromJson` that throws on a shape
 * violation is upgraded to an InvalidResponseException carrying the response
 * status, so malformed server data surfaces as a typed server failure instead
 * of "[object Object]" strings reaching the UI.
 */
export function parseDto<T>(
  body: unknown,
  status: number,
  fromJson: (body: unknown) => T,
): T {
  try {
    return fromJson(body);
  } catch (error) {
    throw new InvalidResponseException(status, { cause: error });
  }
}

/** Wrap a low-level data-source operation. Re-throws typed AppExceptions. */
export async function guardedApiCall<T>(call: () => Promise<T>): Promise<T> {
  try {
    return await call();
  } catch (e) {
    throw toAppException(e);
  }
}

/** Convert any thrown value into a typed AppException (idempotent if already one). */
export function toAppException(error: unknown): AppException {
  if (error instanceof AppException) return error;

  if (error instanceof TypeError) {
    // fetch()/network failures surface as TypeError ("Failed to fetch").
    return new NetworkException('Could not reach the server.', { cause: error });
  }

  return new ServerException('Unexpected error', { cause: error });
}

/** Map an AppException to a Failure, preserving the machine diagnostics. */
export function mapExceptionToFailure(e: AppException): Failure {
  const diagnostics = { code: e.code, status: e.status, requestId: e.requestId };

  if (e instanceof NetworkException) {
    return new NetworkFailure(e.message, diagnostics);
  }
  if (e instanceof ServerException) {
    return new ServerFailure(e.message, diagnostics);
  }

  return new UnknownFailure(e.message, diagnostics);
}

/**
 * Wrap a repository method and return a Result. This is the layer's one
 * guarantee: expected failures (network down, error envelope, malformed
 * payload) AND unexpected engine errors all resolve as Failed Results — the
 * caller never needs try/catch and no rejection goes unhandled.
 */
export async function safeRepositoryCall<T>(call: () => Promise<T>): Promise<Result<T>> {
  try {
    return success(await call());
  } catch (e) {
    return failed(mapExceptionToFailure(toAppException(e)));
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}
