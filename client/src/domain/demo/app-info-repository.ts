import type { Result } from '../../core/result';
import type { AppInfoDemoData } from './app-info';

/**
 * AppInfoRepository — the domain-facing port for the go-http-fundamentals
 * topic's app-info demo payload. Demo components depend on this interface
 * only; the HTTP adapter lives in data/demo and is wired at the
 * composition root.
 */
export interface AppInfoRepository {
  /** The running server's self-introduction, computed live by the server. */
  get(): Promise<Result<AppInfoDemoData>>;
}
