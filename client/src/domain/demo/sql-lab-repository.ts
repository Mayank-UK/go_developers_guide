import type { Result } from '../../core/result';
import type { SQLLabDemoData } from './sql-lab';

/** SQLLabRepository — the domain-facing port for the SQL lab payload. */
export interface SQLLabRepository {
  /** The CTE and window-function exhibits over the seeded posts. */
  get(): Promise<Result<SQLLabDemoData>>;
}
