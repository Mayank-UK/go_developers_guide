import type { Result } from '../../core/result';
import type { BestChecklistDemoData } from './best-checklist';

/**
 * BestChecklistRepository — the domain-facing port for the
 * best-practices topic's self-audit payload. Demo components depend on
 * this interface only; the HTTP adapter lives in data/demo and is
 * wired at the composition root.
 */
export interface BestChecklistRepository {
  /** The boot-cached self-audit: areas, not-dos, summary. */
  get(): Promise<Result<BestChecklistDemoData>>;
}
