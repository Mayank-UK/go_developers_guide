import type { Result } from '../../core/result';
import type { RaceDetectorDemoData } from './race-detector';

/**
 * RaceDetectorRepository — the domain-facing port for the race-detector
 * demo payload. Demo components depend on this interface only; the HTTP
 * adapter lives in data/demo and is wired at the composition root.
 */
export interface RaceDetectorRepository {
  /** The pinned verdict plus the live synchronized totals and hygiene rows. */
  get(): Promise<Result<RaceDetectorDemoData>>;
}
