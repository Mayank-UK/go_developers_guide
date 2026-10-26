import type { Result } from '../../core/result';
import type { MongoConsoleDemoData } from './mongo-console';

/**
 * MongoConsoleRepository — the domain-facing port for the mongodb
 * topic's console payload. Demo components depend on this interface
 * only; the HTTP adapter lives in data/demo and is wired at the
 * composition root.
 */
export interface MongoConsoleRepository {
  /** The whole scripted console run against the live MongoDB. */
  get(): Promise<Result<MongoConsoleDemoData>>;
}
