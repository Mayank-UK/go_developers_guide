import type { AppConfig } from '../../core/config/app-config';
import { guardedGetJson, parseDto } from '../../core/error-handling/guards';

/**
 * HealthApi — the transport for GET /api/health: resolve the request against
 * the configured API base URL, send it through the shared guarded fetch
 * pipeline (network down / non-JSON / non-2xx error envelope all become typed
 * AppExceptions — see core/error-handling/guards.ts), then validate the JSON
 * payload into a DTO. Converting the typed exceptions into Failed Results is
 * the repository adapter's job, so domain code never sees a thrown error.
 */

/** Wire shape of GET /api/health (the server's published contract). */
export interface HealthApiPayload {
  status: string;
  env: string;
}

export class HealthApi {
  private readonly config: AppConfig;

  constructor(config: AppConfig) {
    this.config = config;
  }

  /** GET {apiBaseUrl}/health — the Go server's liveness probe. */
  async getHealth(): Promise<HealthApiPayload> {
    const { body, status } = await guardedGetJson(this.config, '/health');
    return parseDto(body, status, parseHealthPayload);
  }
}

/** Deserialize + validate the health payload (input is hostile). */
export function parseHealthPayload(body: unknown): HealthApiPayload {
  if (typeof body !== 'object' || body === null) {
    throw new Error('Health payload must be an object.');
  }
  const record = body as Record<string, unknown>;
  const status = record.status;
  if (typeof status !== 'string' || status.length === 0) {
    throw new Error('Health payload is missing a non-empty string "status".');
  }
  const env = record.env;
  if (typeof env !== 'string' || env.length === 0) {
    throw new Error('Health payload is missing a non-empty string "env".');
  }
  return { status, env };
}
