import { describe, expect, it } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { PostsWorkbenchDemo } from './posts_workbench_demo';
import { container, ContainerTokens } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetPostsWorkbenchDemoUseCase,
  POSTS_WORKBENCH_DEMO_USE_CASE,
} from '../../../domain/demo/get-posts-workbench-demo-usecase';
import type { PostsWorkbenchDemoData } from '../../../domain/demo/posts-workbench';
import type { CreatePostUseCase } from '../../../domain/posts/create-post-usecase';
import type { DeletePostUseCase } from '../../../domain/posts/delete-post-usecase';
import type { ListPostsUseCase } from '../../../domain/posts/list-posts-usecase';
import type { ReplacePostUseCase } from '../../../domain/posts/replace-post-usecase';
import type { Post, PostInput, PostPage, PostsListQuery } from '../../../domain/posts/post';

/**
 * PostsWorkbenchDemo spec — fixture-driven through the container seams
 * (the four resource use cases under their ContainerTokens, the contract
 * card under its own symbol token). Behavior, not implementation: the
 * contract card renders the route table; the workbench lists with honest
 * page metadata; create/edit/delete drive their use cases and surface the
 * server's own envelope verbatim on failure — the honest error paths —
 * and filter/pagination controls reach the list use case with the exact
 * query the wire contract specifies.
 */

const seedPage: PostPage = {
  items: [
    {
      id: 'p-6',
      title: 'Structured logging with slog',
      body: 'One key-value record at a time.',
      tags: ['go', 'observability'],
      createdAt: '2026-01-18T16:30:00Z',
      updatedAt: '2026-01-18T16:30:00Z',
    },
    {
      id: 'p-5',
      title: 'Pagination that tells the truth',
      body: 'totalItems and totalPages, honestly.',
      tags: ['api', 'design'],
      createdAt: '2026-01-15T11:50:00Z',
      updatedAt: '2026-01-15T11:50:00Z',
    },
  ],
  page: 1,
  limit: 5,
  totalItems: 6,
  totalPages: 2,
  hasNextPage: true,
  hasPrevPage: false,
};

const card: PostsWorkbenchDemoData = {
  resource: { name: 'posts', collectionPath: '/api/posts', itemPath: '/api/posts/{id}', wireShape: '{"id"}' },
  routes: [
    {
      id: 'create',
      method: 'POST',
      path: '/api/posts',
      purpose: 'create a post from the full draft body',
      success: { status: 201, note: 'the stored post + Location' },
      errors: [{ status: 422, code: 'validation_failed', when: 'a field breaks a rule' }],
    },
    {
      id: 'delete',
      method: 'DELETE',
      path: '/api/posts/{id}',
      purpose: 'remove a post under the strict stance',
      success: { status: 204, note: 'no body' },
      errors: [{ status: 404, code: 'not_found', when: 'already gone' }],
    },
  ],
  pagination: {
    defaultLimit: 10,
    maxLimit: 50,
    defaultPage: 1,
    defaultSort: 'newest',
    sortOptions: ['newest', 'oldest', 'title'],
    envelopeFields: ['items', 'page', 'limit', 'totalItems'],
    emptyPageRule: 'a past-the-end page is valid and empty',
  },
  filtering: { param: 'tag', semantics: 'exact match', example: '/api/posts?tag=go' },
  versioning: { prefix: '/api', strategy: 'unversioned', contentExample: 'version 1', whenToVersion: 'on break' },
  deleteSemantics: {
    onMiss: 404,
    chosen: 'strict — DELETE of a vanished post answers 404',
    rationale: 'the response reports the world',
    alternative: 'idempotent 204',
  },
  envelope: {
    wireShape: '{"error":{"code","message","requestId","details?"}}',
    members: [{ name: 'code', meaning: 'the taxonomy bucket' }],
    rfc9457Verdict: 'kindred bones',
  },
};

interface Recorder {
  listQueries: PostsListQuery[];
  created: PostInput[];
  replaced: { id: string; input: PostInput }[];
  removed: string[];
  listResult?: () => Result<PostPage>;
  createResult?: () => Result<Post>;
  replaceResult?: () => Result<Post>;
  removeResult?: () => Result<void>;
}

function stubWorkbench(recorder: Recorder): void {
  container.initialize(createAppConfig());
  container.register(POSTS_WORKBENCH_DEMO_USE_CASE, {
    execute: () => Promise.resolve(success(card)),
  } as unknown as GetPostsWorkbenchDemoUseCase);
  container.register(ContainerTokens.ListPostsUseCase, {
    execute: (query: PostsListQuery) => {
      recorder.listQueries.push(query);
      return Promise.resolve(recorder.listResult ? recorder.listResult() : success(seedPage));
    },
  } as unknown as ListPostsUseCase);
  container.register(ContainerTokens.CreatePostsUseCase, {
    execute: (input: PostInput) => {
      recorder.created.push(input);
      return Promise.resolve(recorder.createResult ? recorder.createResult() : success(createdPost));
    },
  } as unknown as CreatePostUseCase);
  container.register(ContainerTokens.ReplacePostsUseCase, {
    execute: (id: string, input: PostInput) => {
      recorder.replaced.push({ id, input });
      const seedPost = seedPage.items[0];
      return Promise.resolve(recorder.replaceResult ? recorder.replaceResult() : success(seedPost ?? createdPost));
    },
  } as unknown as ReplacePostUseCase);
  container.register(ContainerTokens.DeletePostsUseCase, {
    execute: (id: string) => {
      recorder.removed.push(id);
      return Promise.resolve(recorder.removeResult ? recorder.removeResult() : success(undefined));
    },
  } as unknown as DeletePostUseCase);
}

const createdPost: Post = {
  id: 'p-7',
  title: 'From the workbench',
  body: 'Written by the spec, stored by the fake pipeline.',
  tags: ['api'],
  createdAt: '2026-02-02T10:00:00Z',
  updatedAt: '2026-02-02T10:00:00Z',
};

describe('PostsWorkbenchDemo', () => {
  it('renders the contract card and the seeded list with honest page metadata', async () => {
    stubWorkbench({ listQueries: [], created: [], replaced: [], removed: [] });

    render(<PostsWorkbenchDemo />);

    await waitFor(() => expect(screen.getByTestId('pw-table')).toBeInTheDocument());

    const cardSection = screen.getByTestId('pw-card');
    expect(cardSection).toHaveTextContent('POST');
    expect(cardSection).toHaveTextContent('/api/posts');
    expect(cardSection).toHaveTextContent('201');
    expect(cardSection).toHaveTextContent('validation_failed');
    expect(cardSection).toHaveTextContent('strict — DELETE of a vanished post answers 404');

    expect(screen.getByTestId('pw-table')).toHaveTextContent('p-6');
    expect(screen.getByTestId('pw-table')).toHaveTextContent('Structured logging with slog');
    expect(screen.getByTestId('pw-page-meta')).toHaveTextContent('page 1 of 2');
    expect(screen.getByTestId('pw-page-meta')).toHaveTextContent('6 items');
  });

  it('drives a create through the pipeline and reloads the list', async () => {
    const recorder: Recorder = {
      listQueries: [],
      created: [],
      replaced: [],
      removed: [],
      listResult: () => success({ ...seedPage, items: [createdPost, ...seedPage.items] }),
    };
    stubWorkbench(recorder);

    render(<PostsWorkbenchDemo />);
    await waitFor(() => expect(screen.getByTestId('pw-table')).toBeInTheDocument());

    const createForm = screen.getByTestId('pw-create-form');
    fireEvent.change(createForm.querySelector('input')!, { target: { value: 'From the workbench' } });
    fireEvent.submit(createForm);

    await waitFor(() => expect(recorder.created).toHaveLength(1));
    expect(recorder.created[0]?.title).toBe('From the workbench');
    await waitFor(() => expect(screen.getByTestId('pw-notice')).toHaveTextContent('201 Created'));
    // The list reloaded: two list calls (initial + after create).
    expect(recorder.listQueries.length).toBe(2);
  });

  it('renders the 422 envelope honestly when the server refuses a draft', async () => {
    const recorder: Recorder = {
      listQueries: [],
      created: [],
      replaced: [],
      removed: [],
      createResult: () =>
        failed(
          new ServerFailure('Invalid post body.', {
            code: 'validation_failed',
            status: 422,
            requestId: 'req-40',
          }),
        ),
    };
    stubWorkbench(recorder);

    render(<PostsWorkbenchDemo />);
    await waitFor(() => expect(screen.getByTestId('pw-table')).toBeInTheDocument());

    fireEvent.submit(screen.getByTestId('pw-create-form'));

    await waitFor(() => expect(screen.getByTestId('pw-notice')).toBeInTheDocument());
    expect(screen.getByTestId('pw-notice')).toHaveTextContent('validation_failed');
    expect(screen.getByTestId('pw-notice')).toHaveTextContent('Invalid post body.');
  });

  it('edits through a full-replace PUT prefilled from the row', async () => {
    const recorder: Recorder = { listQueries: [], created: [], replaced: [], removed: [] };
    stubWorkbench(recorder);

    render(<PostsWorkbenchDemo />);
    await waitFor(() => expect(screen.getByTestId('pw-table')).toBeInTheDocument());

    fireEvent.click(screen.getAllByRole('button', { name: 'Edit' })[0]!);
    const editForm = screen.getByTestId('pw-edit-form');
    expect(editForm).toHaveTextContent('PUT /api/posts/p-6');
    // Prefilled from the row: the input holds the post's title.
    expect(editForm.querySelector('input')!.value).toBe('Structured logging with slog');

    fireEvent.submit(editForm);

    await waitFor(() => expect(recorder.replaced).toHaveLength(1));
    expect(recorder.replaced[0]?.id).toBe('p-6');
    expect(screen.queryByTestId('pw-edit-form')).not.toBeInTheDocument();
  });

  it('deletes through the pipeline and shows the strict-stance 404 verbatim', async () => {
    const recorder: Recorder = {
      listQueries: [],
      created: [],
      replaced: [],
      removed: [],
      removeResult: () =>
        failed(
          new ServerFailure('Post "p-6" not found.', {
            code: 'not_found',
            status: 404,
            requestId: 'req-41',
          }),
        ),
    };
    stubWorkbench(recorder);

    render(<PostsWorkbenchDemo />);
    await waitFor(() => expect(screen.getByTestId('pw-table')).toBeInTheDocument());

    fireEvent.click(screen.getAllByRole('button', { name: 'Delete' })[0]!);

    await waitFor(() => expect(recorder.removed).toEqual(['p-6']));
    await waitFor(() => expect(screen.getByTestId('pw-notice')).toHaveTextContent('not_found'));
    expect(screen.getByTestId('pw-notice')).toHaveTextContent('Post "p-6" not found.');
  });

  it('applies the tag filter and pagination through the list use case', async () => {
    const recorder: Recorder = { listQueries: [], created: [], replaced: [], removed: [] };
    stubWorkbench(recorder);

    render(<PostsWorkbenchDemo />);
    await waitFor(() => expect(screen.getByTestId('pw-table')).toBeInTheDocument());

    fireEvent.change(screen.getByLabelText('tag filter'), { target: { value: 'go' } });
    fireEvent.click(screen.getByRole('button', { name: 'Filter' }));

    await waitFor(() =>
      expect(recorder.listQueries.some((query) => query.tag === 'go' && query.page === 1)).toBe(true),
    );

    fireEvent.click(screen.getByRole('button', { name: /next/ }));
    await waitFor(() =>
      expect(recorder.listQueries.some((query) => query.page === 2 && query.tag === 'go')).toBe(true),
    );
  });

  it('degrades the list to the shared error panel with recovery', async () => {
    let calls = 0;
    const recorder: Recorder = {
      listQueries: [],
      created: [],
      replaced: [],
      removed: [],
      listResult: () => {
        calls += 1;
        return calls === 1
          ? failed(
              new ServerFailure('The demo endpoint is down.', {
                code: 'service_unavailable',
                status: 503,
                requestId: 'req-31',
              }),
            )
          : success(seedPage);
      },
    };
    stubWorkbench(recorder);

    render(<PostsWorkbenchDemo />);

    await waitFor(() => expect(screen.getAllByTestId('error-panel').length).toBeGreaterThan(0));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('pw-table')).toBeInTheDocument());
  });
});
