import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { schemaExplorerDtoFromJson, type SchemaExplorerDemoDto } from './schema-explorer-dto';

/**
 * SchemaExplorerApi — the transport for GET /api/demo/db/schema-explorer,
 * in the same style as data/content/content-api.ts: the request goes
 * through the shared guarded fetch pipeline (network down / non-JSON /
 * non-2xx error envelope all become typed AppExceptions — the server's
 * honest 503 for an unreachable PostgreSQL included), then the JSON
 * payload is validated into a DTO.
 */

const DEMO_PATH = '/demo/db/schema-explorer';

export class SchemaExplorerApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/db/schema-explorer — the live catalog's own
   * account of the schema. */
  async get(): Promise<SchemaExplorerDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, schemaExplorerDtoFromJson);
    });
  }
}
