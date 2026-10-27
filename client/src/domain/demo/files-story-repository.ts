import type { Result } from '../../core/result';
import type { FilesStoryDemoData } from './files-story';

/**
 * FilesStoryRepository — the domain-facing port for the files topic's
 * workbench story demo. The demo component depends on this interface
 * only; the HTTP adapter lives in data/demo and is wired at the
 * composition root.
 */
export interface FilesStoryRepository {
  /** The story's beats, proofs, and reasoning. */
  get(): Promise<Result<FilesStoryDemoData>>;
}
