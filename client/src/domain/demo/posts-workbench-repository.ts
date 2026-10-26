import type { Result } from '../../core/result';
import type { PostsWorkbenchDemoData } from './posts-workbench';

/**
 * PostsWorkbenchRepository — the domain-facing port for the posts
 * workbench contract-card payload. Demo components depend on this
 * interface only; the HTTP adapter lives in data/demo and is wired at the
 * composition root.
 */
export interface PostsWorkbenchRepository {
  /** The api-design topic's contract card (route table + rules). */
  get(): Promise<Result<PostsWorkbenchDemoData>>;
}
