import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { SchemaExplorerRepository } from '../../domain/demo/schema-explorer-repository';
import type { SchemaExplorerDemoData } from '../../domain/demo/schema-explorer';
import { toSchemaExplorerDemo } from './schema-explorer-dto';
import type { SchemaExplorerApi } from './schema-explorer-api';

/**
 * SchemaExplorerRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload) is
 * guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class SchemaExplorerRepositoryImpl implements SchemaExplorerRepository {
  private readonly api: SchemaExplorerApi;

  constructor(api: SchemaExplorerApi) {
    this.api = api;
  }

  async get(): Promise<Result<SchemaExplorerDemoData>> {
    return safeRepositoryCall(async () => toSchemaExplorerDemo(await this.api.get()));
  }
}
