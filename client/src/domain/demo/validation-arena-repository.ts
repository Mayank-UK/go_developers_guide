import type { Result } from '../../core/result';
import type { ValidationArenaDemoData } from './validation-arena';

/**
 * ValidationArenaRepository — the domain-facing port for the validation
 * arena demo payload. Demo components depend on this interface only; the
 * HTTP adapter lives in data/demo and is wired at the composition root.
 */
export interface ValidationArenaRepository {
  /** The judged candidate roster: both layers' verdicts and the real 422s. */
  get(): Promise<Result<ValidationArenaDemoData>>;
}
