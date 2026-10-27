import type { Result } from '../../core/result';
import type { EnvInspectorDemoData } from './env-inspector';

/**
 * EnvInspectorRepository — the domain-facing port for the environments
 * topic's env inspector demo payload. Demo components depend on this
 * interface only; the HTTP adapter lives in data/demo and is wired at
 * the composition root.
 */
export interface EnvInspectorRepository {
  /** This process's resolved config, the scorecard, and the live flags. */
  get(): Promise<Result<EnvInspectorDemoData>>;
}
