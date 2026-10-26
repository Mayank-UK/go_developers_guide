import type { Post, PostInput, PostPage, PostSort } from '../../domain/posts/post';
import {
  isRecord,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * PostsDTO — the wire/transport shapes of the posts resource, mirrored
 * from the server's presentation layer. `fromJson` builds ONLY the listed
 * fields (whitelist) and throws on any shape violation, which parseDto
 * upgrades to an InvalidResponseException — hostile or drifting server
 * data never reaches the domain as "[object Object]" titles or NaN pages.
 *
 * `title` and `body` are `requiredText` (present-but-empty passes): the
 * server's 422s name those cases, so a client-side parser must not
 * pre-reject what the contract can legitimately discuss. `tags` may be an
 * empty array (normalized on write) but never null and never non-strings.
 */

export type PostDto = Post;
export type PostPageDto = PostPage;

const POST_SORTS: readonly string[] = ['newest', 'oldest', 'title'];

/** Deserialize + validate one post from the wire. */
export function postFromJson(body: unknown): PostDto {
  if (!isRecord(body)) throw new Error('post must be an object');
  return {
    id: requiredString(body, 'id'),
    title: requiredText(body, 'title'),
    body: requiredText(body, 'body'),
    tags: requiredArray(body, 'tags').map((tag) => {
      if (typeof tag !== 'string' || tag === '') {
        throw new Error('expected non-empty string tags');
      }
      return tag;
    }),
    createdAt: requiredString(body, 'createdAt'),
    updatedAt: requiredString(body, 'updatedAt'),
  };
}

/** Deserialize + validate a page envelope. */
export function postPageFromJson(body: unknown): PostPageDto {
  if (!isRecord(body)) throw new Error('post page must be an object');
  return {
    items: requiredArray(body, 'items').map(postFromJson),
    page: requiredNumber(body, 'page'),
    limit: requiredNumber(body, 'limit'),
    totalItems: requiredNumber(body, 'totalItems'),
    totalPages: requiredNumber(body, 'totalPages'),
    hasNextPage: requiredBoolean(body, 'hasNextPage'),
    hasPrevPage: requiredBoolean(body, 'hasPrevPage'),
  };
}

/** Serialize a create/replace input — exactly the three draft fields. */
export function postInputToJson(input: PostInput): string {
  return JSON.stringify({
    title: input.title,
    body: input.body,
    tags: input.tags,
  });
}

/**
 * Build the list query string (empty controls are omitted so the server's
 * defaults apply — the same contract the terminal tour exercises).
 */
export function postsListQueryString(query: {
  page?: number;
  limit?: number;
  tag?: string;
  sort?: string;
}): string {
  const params = new URLSearchParams();
  if (query.page !== undefined) params.set('page', String(query.page));
  if (query.limit !== undefined) params.set('limit', String(query.limit));
  const tag = query.tag?.trim();
  if (tag !== undefined && tag !== '') params.set('tag', tag);
  if (query.sort !== undefined) {
    if (!POST_SORTS.includes(query.sort)) {
      throw new Error(`unknown post sort "${query.sort}"`);
    }
    params.set('sort', query.sort);
  }
  const encoded = params.toString();
  return encoded === '' ? '' : `?${encoded}`;
}

/** Narrow an unknown value to a known sort (closed vocabulary guard). */
export function asPostSort(value: unknown): PostSort | undefined {
  return typeof value === 'string' && POST_SORTS.includes(value)
    ? (value as PostSort)
    : undefined;
}
