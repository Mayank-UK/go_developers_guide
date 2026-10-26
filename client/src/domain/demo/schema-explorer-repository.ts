import type { Result } from '../../core/result';
import type { SchemaExplorerDemoData } from './schema-explorer';

/**
 * SchemaExplorerRepository — the domain-facing port for the sql-schema
 * topic's live-schema demo payload. Demo components depend on this
 * interface only; the HTTP adapter lives in data/demo and is wired at the
 * composition root.
 */
export interface SchemaExplorerRepository {
  /** The live PostgreSQL schema as the catalog itself describes it. */
  get(): Promise<Result<SchemaExplorerDemoData>>;
}
