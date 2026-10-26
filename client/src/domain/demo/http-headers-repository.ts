import type { Result } from '../../core/result';
import type { HttpHeadersDemoData } from './http-headers';

/**
 * HttpHeadersRepository — the domain-facing port for the headers echo
 * demo payload. Demo components depend on this interface only; the HTTP
 * adapter lives in data/demo and is wired at the composition root.
 */
export interface HttpHeadersRepository {
  /** The echo of this very fetch's request, classified by the server. */
  get(): Promise<Result<HttpHeadersDemoData>>;
}
