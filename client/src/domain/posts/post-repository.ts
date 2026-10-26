import type { Result } from '../../core/result';
import type { Post, PostInput, PostPage, PostsListQuery } from './post';

/**
 * PostsRepository — the domain-facing port for the posts resource CRUD
 * client. The workbench demo and any future posts views depend on this
 * interface only; the HTTP adapter lives in data/posts and is wired at the
 * composition root. Expected failures (422 issue envelopes, 404 misses,
 * network) arrive as Failed Results — implementors never throw.
 */
export interface PostsRepository {
  /** One page of the collection for the query controls. */
  list(query: PostsListQuery): Promise<Result<PostPage>>;
  /** Create a post; the stored post (with id + timestamps) returns. */
  create(input: PostInput): Promise<Result<Post>>;
  /** Full replacement (PUT semantics): the input is the whole next state. */
  replace(id: string, input: PostInput): Promise<Result<Post>>;
  /** Delete under the server's strict stance (a miss is a Failed Result). */
  remove(id: string): Promise<Result<void>>;
}
