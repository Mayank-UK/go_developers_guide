import type { Result } from '../../core/result';
import type { ScenarioPatch, ScenarioState } from './scenario';

/**
 * ScenarioRepository — the domain-facing port for the demo scenario toggle.
 * Use cases and the header panel depend on this interface only; the adapter
 * that talks HTTP lives in data/scenario and is wired at the composition root
 * (core/container.ts).
 */
export interface ScenarioRepository {
  /** The store's current snapshot. */
  getScenario(): Promise<Result<ScenarioState>>;

  /**
   * Apply a partial update (absent keys mean unchanged) and answer with the
   * resulting full state. An invalid value never mutates the store — the
   * server rejects it as a 422 and this resolves to a Failed Result.
   */
  patchScenario(patch: ScenarioPatch): Promise<Result<ScenarioState>>;
}
