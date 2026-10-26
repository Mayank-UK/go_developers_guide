import type { Result } from '../../core/result';
import type { RuntimeTypesDemoData } from './runtime-types';

/**
 * RuntimeTypesRepository — the domain-facing port for the runtime-types
 * demo payload. Demo components depend on this interface only; the HTTP
 * adapter lives in data/demo and is wired at the composition root.
 */
export interface RuntimeTypesRepository {
  /** The full runtime-types proof payload, computed live by the server. */
  get(): Promise<Result<RuntimeTypesDemoData>>;
}
