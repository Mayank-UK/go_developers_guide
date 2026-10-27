import type { Result } from '../../core/result';
import type { ArchTracerDemoData } from './arch-tracer';

/**
 * ArchTracerRepository — the domain-facing port for the architecture
 * topic's tracer demo payload. Demo components depend on this interface
 * only; the HTTP adapter lives in data/demo and is wired at the
 * composition root.
 */
export interface ArchTracerRepository {
  /** The traced timeline (current scenario mode) + counterexample panel. */
  get(): Promise<Result<ArchTracerDemoData>>;
}
