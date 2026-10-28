import type { Result } from '../../core/result';
import type { ScalingStatelessDemoData } from './scaling-stateless';

/**
 * ScalingStatelessRepository — the domain-facing port for the scaling
 * topic's statelessness payload. Demo components depend on this
 * interface only; the HTTP adapter lives in data/demo and is wired at
 * the composition root.
 */
export interface ScalingStatelessRepository {
  /** The measured statelessness round: distribution, steps, verdicts. */
  get(): Promise<Result<ScalingStatelessDemoData>>;
}
