import type { Result } from '../../core/result';
import type { ArchSagaDemoData } from './arch-saga';

/**
 * ArchSagaRepository — the domain-facing port for the
 * advanced-architecture topic's saga payload. Demo components depend on
 * this interface only; the HTTP adapter lives in data/demo and is wired
 * at the composition root.
 */
export interface ArchSagaRepository {
  /** The compensation ledger: the fixed script, the three observed runs, the machinery surface. */
  get(): Promise<Result<ArchSagaDemoData>>;
}
