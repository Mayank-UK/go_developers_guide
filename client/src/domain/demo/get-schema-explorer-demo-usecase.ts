import type { Result } from '../../core/result';
import type { SchemaExplorerRepository } from './schema-explorer-repository';
import type { SchemaExplorerDemoData } from './schema-explorer';

/**
 * SCHEMA_EXPLORER_DEMO_USE_CASE — the container token this demo's use
 * case is registered under. Lives here (not in container.ts's
 * ContainerTokens) so the demo component has its resolution seam from day
 * one; the composition root registers the real pipeline under this exact
 * symbol:
 *
 *   container.register(SCHEMA_EXPLORER_DEMO_USE_CASE, new GetSchemaExplorerDemoUseCase(repository));
 */
export const SCHEMA_EXPLORER_DEMO_USE_CASE = Symbol('GetSchemaExplorerDemoUseCase');

/**
 * GetSchemaExplorerDemoUseCase — loads the sql-schema topic's live-schema
 * introspection payload. Thin orchestration seam over the repository port
 * (react-guide use-case style); parameterless because the demo has no
 * user inputs — the payload describes the database, not the fetch.
 */
export class GetSchemaExplorerDemoUseCase {
  private readonly repository: SchemaExplorerRepository;

  constructor(repository: SchemaExplorerRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<SchemaExplorerDemoData>> {
    return this.repository.get();
  }
}
