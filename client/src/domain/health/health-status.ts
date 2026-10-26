/**
 * HealthStatus — the pure domain model of GET /api/health, the liveness probe
 * behind the home view's live status readout. The server contract is settled
 * (Phase 0.B): `status` is the literal "ok" and `env` is the environment the
 * server booted in (e.g. "dev").
 */
export interface HealthStatus {
  status: string;
  env: string;
}
