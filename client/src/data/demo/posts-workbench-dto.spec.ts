import { describe, expect, it } from 'vitest';
import { postsWorkbenchDtoFromJson } from './posts-workbench-dto';
import type { PostsWorkbenchDemoData } from '../../domain/demo/posts-workbench';

/**
 * PostsWorkbenchDTO spec — hostile-input discipline: only the exact wire
 * shape parses (mirroring the Go builder's payload), anything else throws
 * and therefore surfaces upstream as an invalid_response Failure.
 */

const card: PostsWorkbenchDemoData = {
  resource: {
    name: 'posts',
    collectionPath: '/api/posts',
    itemPath: '/api/posts/{id}',
    wireShape: '{"id","title","body","tags","createdAt","updatedAt"}',
  },
  routes: [
    {
      id: 'create',
      method: 'POST',
      path: '/api/posts',
      purpose: 'create a post from the full draft body',
      success: { status: 201, note: 'the stored post + Location: /api/posts/{id}' },
      errors: [
        { status: 422, code: 'validation_failed', when: 'body is not one strict JSON object' },
      ],
    },
  ],
  pagination: {
    defaultLimit: 10,
    maxLimit: 50,
    defaultPage: 1,
    defaultSort: 'newest',
    sortOptions: ['newest', 'oldest', 'title'],
    envelopeFields: ['items', 'page', 'limit', 'totalItems', 'totalPages', 'hasNextPage', 'hasPrevPage'],
    emptyPageRule: 'a page past the end is a valid empty page',
  },
  filtering: {
    param: 'tag',
    semantics: 'exact match against write-normalized tags',
    example: '/api/posts?tag=go&limit=3',
  },
  versioning: {
    prefix: '/api',
    strategy: 'unversioned prefix; version the day a breaking change ships',
    contentExample: 'the content endpoints carry a version field',
    whenToVersion: 'the moment you cannot change a response without breaking a client',
  },
  deleteSemantics: {
    onMiss: 404,
    chosen: 'strict — DELETE of a vanished post answers 404',
    rationale: 'the response reports the world',
    alternative: 'idempotent 204-on-any-miss trades honesty for retry-friendliness',
  },
  envelope: {
    wireShape: '{"error":{"code","message","requestId","details?"}}',
    members: [
      { name: 'code', meaning: 'the taxonomy bucket — a closed set' },
      { name: 'requestId', meaning: 'the join key for logs' },
    ],
    rfc9457Verdict: 'RFC 9457 problem+json shares the shape\u0027s bones',
  },
};

describe('postsWorkbenchDtoFromJson', () => {
  it('parses a well-formed contract card round-trip', () => {
    expect(postsWorkbenchDtoFromJson(card)).toEqual(card);
  });

  it('throws when any top-level section is missing or not an object', () => {
    for (const broken of [
      null,
      'card',
      { ...card, routes: undefined },
      { ...card, pagination: 'fast' },
      { ...card, envelope: null },
    ]) {
      expect(() => postsWorkbenchDtoFromJson(broken)).toThrow();
    }
  });

  it('throws when a route row is malformed', () => {
    expect(() =>
      postsWorkbenchDtoFromJson({ ...card, routes: [{ id: 'x', method: 'GET' }] }),
    ).toThrow();
  });

  it('throws when the pagination numbers drift into strings', () => {
    expect(() =>
      postsWorkbenchDtoFromJson({ ...card, pagination: { ...card.pagination, defaultLimit: '10' } }),
    ).toThrow();
  });

  it('throws when an envelope member loses its meaning', () => {
    expect(() => postsWorkbenchDtoFromJson({ ...card, envelope: { ...card.envelope, members: [{ name: 'code' }] } })).toThrow();
  });
});
