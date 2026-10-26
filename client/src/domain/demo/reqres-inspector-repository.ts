import type { Result } from '../../core/result';
import type { ReqresInspectorDemoData } from './reqres-inspector';

/**
 * ReqresInspectorRepository — the domain-facing port for the request-
 * inspector demo payload. Demo components depend on this interface only;
 * the HTTP adapter lives in data/demo and is wired at the composition
 * root.
 */
export interface ReqresInspectorRepository {
  /** The live request's facts beside the decoder discipline's verdicts. */
  get(): Promise<Result<ReqresInspectorDemoData>>;
}
