import type { Result } from '../../core/result';
import type { GRPCConsoleDemoData } from './grpc-console';

/**
 * GRPCConsoleRepository — the domain-facing port for the grpc console
 * demo payload. Demo components depend on this interface only; the HTTP
 * adapter lives in data/demo and is wired at the composition root.
 */
export interface GRPCConsoleRepository {
  /** The six exhibit reports, fresh from a real in-process round trip. */
  get(): Promise<Result<GRPCConsoleDemoData>>;
}
