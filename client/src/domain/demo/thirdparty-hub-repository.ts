import type { Result } from '../../core/result';
import type { ThirdPartyHubDemoData } from './thirdparty-hub';

/**
 * ThirdPartyHubRepository — the domain-facing port for the
 * third-party topic's hub payload. Demo components depend on this
 * interface only; the HTTP adapter lives in data/demo and is wired at
 * the composition root.
 */
export interface ThirdPartyHubRepository {
  /** The adapter-surface hub: wiring, vendors, taxonomy map. */
  get(): Promise<Result<ThirdPartyHubDemoData>>;
}
