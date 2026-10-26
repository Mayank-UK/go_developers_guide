import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { routingExplorerDtoFromJson, type RoutingExplorerDemoDto } from './routing-explorer-dto';

/**
 * RoutingExplorerApi — the transport for GET /api/demo/routing/explorer,
 * in the same style as data/content/content-api.ts: the request goes
 * through the shared guarded fetch pipeline (network down / non-JSON /
 * non-2xx error envelope all become typed AppExceptions), then the JSON
 * payload is validated into a DTO. Converting typed exceptions into
 * Failed Results is the repository adapter's job.
 */

const DEMO_PATH = '/demo/routing/explorer';

export class RoutingExplorerApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/routing/explorer — the route table under test. */
  async get(): Promise<RoutingExplorerDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, DEMO_PATH);
      return parseDto(body, status, routingExplorerDtoFromJson);
    });
  }
}
