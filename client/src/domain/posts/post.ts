/**
 * Domain models for the posts resource — the client-side mirror of the
 * api-design topic's real /api/posts API. Field names match the server's
 * camelCase wire contract 1:1 (the DTO is the seam where wire and domain
 * may diverge later). Timestamps stay RFC 3339 strings end to end: the
 * client formats them, it never does date arithmetic on them.
 */

/** Sort vocabulary — the server's closed set (an unknown value is a 422). */
export type PostSort = 'newest' | 'oldest' | 'title';

/** The sort options in wire order (default first). */
export const POST_SORTS: readonly PostSort[] = ['newest', 'oldest', 'title'];

/** One post, exactly as the wire serves it. */
export interface Post {
  id: string;
  title: string;
  body: string;
  /** Normalized on write by the server: trimmed, lowercase, never null. */
  tags: string[];
  /** RFC 3339 UTC instant. */
  createdAt: string;
  /** RFC 3339 UTC instant. */
  updatedAt: string;
}

/** The create/replace input — the full draft the server validates. */
export interface PostInput {
  title: string;
  body: string;
  tags: string[];
}

/** List controls; every field optional (the server applies its defaults). */
export interface PostsListQuery {
  page?: number;
  limit?: number;
  /** Exact-match tag filter; omit for no filter. */
  tag?: string;
  sort?: PostSort;
}

/** One honest page of the collection with its navigation metadata. */
export interface PostPage {
  items: Post[];
  page: number;
  limit: number;
  totalItems: number;
  totalPages: number;
  hasNextPage: boolean;
  hasPrevPage: boolean;
}
