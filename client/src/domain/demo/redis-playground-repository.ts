import type { Result } from '../../core/result';
import type { RedisPlaygroundDemoData } from './redis-playground';

/**
 * RedisPlaygroundRepository — the domain-facing port for the redis
 * topic's live-store demo payload. Demo components depend on this
 * interface only; the HTTP adapter lives in data/demo and is wired at the
 * composition root.
 */
export interface RedisPlaygroundRepository {
  /** The live walk's rows: structures, counter, TTL, pub/sub, cleanup. */
  get(): Promise<Result<RedisPlaygroundDemoData>>;
}
