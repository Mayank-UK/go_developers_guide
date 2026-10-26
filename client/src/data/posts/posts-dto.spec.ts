import { describe, expect, it } from 'vitest';
import {
  asPostSort,
  postFromJson,
  postInputToJson,
  postPageFromJson,
  postsListQueryString,
} from './posts-dto';
import type { PostPage } from '../../domain/posts/post';

/**
 * PostsDTO spec — hostile-input discipline: only the exact wire shape
 * parses, anything else throws (and therefore surfaces upstream as an
 * invalid_response Failure, never as a guessed default). A blank
 * `title`/`body` passes (`requiredText`): present-but-empty is DATA the
 * server's 422s discuss. The query-string builder mirrors the terminal
 * tour's URL contract: omitted controls stay omitted.
 */

const wirePost = {
  id: 'p-7',
  title: 'A post from the spec',
  body: 'The stored body.',
  tags: ['go', 'api'],
  createdAt: '2026-02-01T10:00:00Z',
  updatedAt: '2026-02-01T10:00:00Z',
};

const wirePage = {
  items: [wirePost],
  page: 2,
  limit: 3,
  totalItems: 7,
  totalPages: 3,
  hasNextPage: true,
  hasPrevPage: true,
};

describe('postFromJson', () => {
  it('parses a well-formed post round-trip', () => {
    expect(postFromJson(wirePost)).toEqual(wirePost);
  });

  it('keeps present-but-empty title and body as data (the 422 contract discusses them)', () => {
    const parsed = postFromJson({ ...wirePost, title: '', body: '' });
    expect(parsed.title).toBe('');
    expect(parsed.body).toBe('');
  });

  it('accepts an empty tags array (normalized on write) but rejects null and non-strings', () => {
    expect(postFromJson({ ...wirePost, tags: [] }).tags).toEqual([]);
    expect(() => postFromJson({ ...wirePost, tags: null })).toThrow();
    expect(() => postFromJson({ ...wirePost, tags: ['ok', 3] })).toThrow();
  });

  it('throws on every missing or mistyped member', () => {
    for (const broken of [
      null,
      'string',
      { ...wirePost, id: '' },
      { ...wirePost, createdAt: 12345 },
      { ...wirePost, updatedAt: undefined },
    ]) {
      expect(() => postFromJson(broken)).toThrow();
    }
  });
});

describe('postPageFromJson', () => {
  it('parses a well-formed page with its honest metadata', () => {
    const parsed: PostPage = postPageFromJson(wirePage);
    expect(parsed.page).toBe(2);
    expect(parsed.totalPages).toBe(3);
    expect(parsed.hasNextPage).toBe(true);
    expect(parsed.items).toHaveLength(1);
  });

  it('parses the past-the-end empty page (items: [], totals truthful)', () => {
    const parsed = postPageFromJson({
      items: [],
      page: 9,
      limit: 4,
      totalItems: 6,
      totalPages: 2,
      hasNextPage: false,
      hasPrevPage: true,
    });
    expect(parsed.items).toEqual([]);
    expect(parsed.totalItems).toBe(6);
  });

  it('throws when items entries are not posts (nested validation bites)', () => {
    expect(() => postPageFromJson({ ...wirePage, items: [{ id: 'p-1' }] })).toThrow();
  });
});

describe('postInputToJson', () => {
  it('serializes exactly the three draft fields', () => {
    expect(postInputToJson({ title: 't', body: 'b', tags: ['x'] })).toBe(
      '{"title":"t","body":"b","tags":["x"]}',
    );
  });
});

describe('postsListQueryString', () => {
  it('omits every control when none is set (the server defaults apply)', () => {
    expect(postsListQueryString({})).toBe('');
  });

  it('renders sent controls in a stable order and trims the tag filter', () => {
    expect(postsListQueryString({ page: 2, limit: 3, tag: ' go ', sort: 'oldest' })).toBe(
      '?page=2&limit=3&tag=go&sort=oldest',
    );
  });

  it('drops an empty/whitespace tag (empty string means no filter)', () => {
    expect(postsListQueryString({ tag: '   ' })).toBe('');
    expect(postsListQueryString({ tag: '' })).toBe('');
  });

  it('refuses an unknown sort before it ever reaches the network', () => {
    expect(() => postsListQueryString({ sort: 'newest-first' })).toThrow(/unknown post sort/);
  });
});

describe('asPostSort', () => {
  it('narrows the closed vocabulary and rejects everything else', () => {
    expect(asPostSort('newest')).toBe('newest');
    expect(asPostSort('title')).toBe('title');
    expect(asPostSort('NEWEST')).toBeUndefined();
    expect(asPostSort(3)).toBeUndefined();
    expect(asPostSort(undefined)).toBeUndefined();
  });
});
