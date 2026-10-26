import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, guardedSendJson, parseDto } from '../../core/error-handling/guards';
import type { ScenarioPatch } from '../../domain/scenario/scenario';
import { scenarioDtoFromJson, serializeScenarioPatch, type ScenarioDto } from './scenario-dto';

/**
 * ScenarioApi — the transport for the demo scenario endpoints, in the same
 * style as data/health/health-api.ts: resolve the request against the
 * configured API base URL, send it through the shared guarded fetch pipeline
 * (network down / non-JSON / non-2xx error envelope all become typed
 * AppExceptions — see core/error-handling/guards.ts), then validate the JSON
 * payload into a DTO. Converting the typed exceptions into Failed Results is
 * the repository adapter's job, so domain code never sees a thrown transport
 * error. Both endpoints answer with the same bare snapshot object.
 */

const SCENARIO_PATH = '/demo/scenario';

export class ScenarioApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/demo/scenario — the store's current snapshot. */
  async getScenario(): Promise<ScenarioDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, SCENARIO_PATH);
      return parseDto(body, status, scenarioDtoFromJson);
    });
  }

  /**
   * PATCH {apiBaseUrl}/demo/scenario — a partial update (absent keys mean
   * unchanged) answered with the resulting full snapshot.
   */
  async patchScenario(patch: ScenarioPatch): Promise<ScenarioDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedSendJson(this.config, SCENARIO_PATH, {
        method: 'PATCH',
        headers: { 'content-type': 'application/json' },
        body: serializeScenarioPatch(patch),
      });
      return parseDto(body, status, scenarioDtoFromJson);
    });
  }
}
