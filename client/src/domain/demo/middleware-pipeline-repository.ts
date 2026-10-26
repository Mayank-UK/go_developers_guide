import type { Result } from '../../core/result';
import type { MiddlewarePipelineDemoData } from './middleware-pipeline';

/**
 * MiddlewarePipelineRepository — the domain-facing port for the
 * middleware pipeline demo payload. Demo components depend on this
 * interface only; the HTTP adapter lives in data/demo and is wired at the
 * composition root.
 */
export interface MiddlewarePipelineRepository {
  /** The traced run of the exhibit chain: order, ids, recovery, timeout. */
  get(): Promise<Result<MiddlewarePipelineDemoData>>;
}
