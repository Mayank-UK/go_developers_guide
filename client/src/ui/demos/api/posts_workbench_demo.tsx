import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetPostsWorkbenchDemoUseCase,
  POSTS_WORKBENCH_DEMO_USE_CASE,
} from '../../../domain/demo/get-posts-workbench-demo-usecase';
import type { PostsWorkbenchDemoData } from '../../../domain/demo/posts-workbench';
import type { CreatePostUseCase } from '../../../domain/posts/create-post-usecase';
import type { DeletePostUseCase } from '../../../domain/posts/delete-post-usecase';
import type { ListPostsUseCase } from '../../../domain/posts/list-posts-usecase';
import type { ReplacePostUseCase } from '../../../domain/posts/replace-post-usecase';
import { POST_SORTS, type Post, type PostInput, type PostPage, type PostSort } from '../../../domain/posts/post';

/**
 * PostsWorkbenchDemo — the api-design topic's live demo (demo block id
 * `api:posts-workbench`). Two halves, one screen:
 *
 * 1. THE CONTRACT CARD — GET /api/demo/api/posts-workbench: the route
 *    table (method, path, purpose, success status, every error case), the
 *    pagination rules, the filtering semantics, the versioning stance,
 *    the recorded DELETE decision, and the error-envelope anatomy with
 *    RFC 9457 beside it.
 * 2. THE WORKBENCH — a REAL CRUD client over the live /api/posts routes:
 *    list with pagination + tag filtering + sort, create (201), edit via
 *    full-replace PUT, delete (204 → strict 404 on repeat). Every button
 *    drives the server through the client's own pipeline (repository port
 *    → use cases → container); failures render honestly — the server's
 *    own envelope code and message, never a pretend error.
 *
 * Dependency flow mirrors the views: hooks resolve use cases through the
 * container (default parameters are the spec seam — tests substitute
 * fakes through the same tokens), expected failures arrive as typed
 * Failures inside Results, and every mutation ends by reloading the page
 * so the table always shows the server's truth.
 */

/* ── the contract card ─────────────────────────────────────────────── */

type CardStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: PostsWorkbenchDemoData }
  | { state: 'error'; failure: Failure };

export function usePostsWorkbenchCard(
  useCase: GetPostsWorkbenchDemoUseCase = container.resolve(POSTS_WORKBENCH_DEMO_USE_CASE),
): { status: CardStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<CardStatus>({ state: 'idle' });

  useEffect(() => {
    let cancelled = false;
    setStatus({ state: 'loading' });
    void useCase.execute().then((result) => {
      if (cancelled) return;
      if (result.ok) {
        setStatus({ state: 'ready', data: result.value });
      } else {
        setStatus({ state: 'error', failure: result.failure });
      }
    });
    return () => {
      cancelled = true;
    };
  }, [useCase, attempt]);

  return { status, retry: () => setAttempt((current) => current + 1) };
}

function ContractCard({ data }: { data: PostsWorkbenchDemoData }) {
  return (
    <section data-testid="pw-card" className="flex flex-col gap-sm">
      <h4 className="font-display text-title-sm text-on-surface">
        The contract card — served from the same binary as the routes
      </h4>
      <div className="overflow-x-auto rounded-md border border-outline">
        <table className="w-full text-left font-mono text-caption">
          <thead className="bg-surface-tint text-on-surface-variant">
            <tr>
              <th scope="col" className="px-sm py-xxs">method</th>
              <th scope="col" className="px-sm py-xxs">path</th>
              <th scope="col" className="px-sm py-xxs">success</th>
              <th scope="col" className="px-sm py-xxs">errors</th>
            </tr>
          </thead>
          <tbody>
            {data.routes.map((route) => (
              <tr key={route.id} className="border-t border-outline align-top">
                <td className="px-sm py-xxs text-secondary">{route.method}</td>
                <td className="px-sm py-xxs text-on-surface">
                  {route.path}
                  <span className="block whitespace-normal font-sans text-caption text-on-surface-variant">
                    {route.purpose}
                  </span>
                </td>
                <td className="px-sm py-xxs text-on-surface">{route.success.status}</td>
                <td className="px-sm py-xxs text-on-surface-variant">
                  {route.errors.map((errorCase) => (
                    <span key={`${route.id}-${errorCase.status}`} className="block whitespace-normal">
                      <span className="text-error">{errorCase.status}</span> {errorCase.code} — {errorCase.when}
                    </span>
                  ))}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <dl className="grid grid-cols-1 gap-xs md:grid-cols-2">
        <div className="rounded-md border border-outline bg-surface-tint px-sm py-xxs">
          <dt className="font-mono text-caption uppercase tracking-wide text-on-surface-variant">pagination</dt>
          <dd className="text-body-sm text-on-surface">
            defaults {data.pagination.defaultPage}/{data.pagination.defaultLimit} · max {data.pagination.maxLimit} · sorts{' '}
            {data.pagination.sortOptions.join(' | ')}
          </dd>
        </div>
        <div className="rounded-md border border-outline bg-surface-tint px-sm py-xxs">
          <dt className="font-mono text-caption uppercase tracking-wide text-on-surface-variant">filtering</dt>
          <dd className="text-body-sm text-on-surface">?{data.filtering.param}= — {data.filtering.semantics}</dd>
        </div>
        <div className="rounded-md border border-outline bg-surface-tint px-sm py-xxs">
          <dt className="font-mono text-caption uppercase tracking-wide text-on-surface-variant">delete stance</dt>
          <dd className="text-body-sm text-on-surface">{data.deleteSemantics.chosen} — {data.deleteSemantics.rationale}</dd>
        </div>
        <div className="rounded-md border border-outline bg-surface-tint px-sm py-xxs">
          <dt className="font-mono text-caption uppercase tracking-wide text-on-surface-variant">versioning</dt>
          <dd className="text-body-sm text-on-surface">{data.versioning.strategy}</dd>
        </div>
      </dl>
      <div className="rounded-md border border-outline px-sm py-xxs">
        <p className="font-mono text-caption text-secondary">{data.envelope.wireShape}</p>
        <ul className="mt-xxs flex flex-col gap-xxs">
          {data.envelope.members.map((member) => (
            <li key={member.name} className="text-body-sm text-on-surface-variant">
              <span className="font-mono text-on-surface">{member.name}</span> — {member.meaning}
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}

/* ── the workbench state machine ───────────────────────────────────── */

interface WorkbenchControls {
  page: number;
  limit: number;
  sort: PostSort;
  tag: string;
}

type ListStatus =
  | { state: 'loading' }
  | { state: 'ready'; page: PostPage }
  | { state: 'error'; failure: Failure };

type MutationNotice = { tone: 'ok' | 'error'; text: string; code?: string } | null;

export interface WorkbenchActions {
  status: ListStatus;
  notice: MutationNotice;
  editing: Post | null;
  busy: boolean;
  controls: WorkbenchControls;
  setSort: (sort: PostSort) => void;
  setLimit: (limit: number) => void;
  applyFilter: (tag: string) => void;
  goToPage: (page: number) => void;
  create: (input: PostInput) => void;
  startEdit: (post: Post) => void;
  submitEdit: (input: PostInput) => void;
  cancelEdit: () => void;
  remove: (id: string) => void;
  retry: () => void;
}

export function usePostsWorkbench(
  listPosts: ListPostsUseCase = container.listPostsUseCase,
  createPosts: CreatePostUseCase = container.createPostUseCase,
  replacePosts: ReplacePostUseCase = container.replacePostUseCase,
  deletePosts: DeletePostUseCase = container.deletePostUseCase,
): WorkbenchActions {
  const [controls, setControls] = useState<WorkbenchControls>({ page: 1, limit: 5, sort: 'newest', tag: '' });
  const [status, setStatus] = useState<ListStatus>({ state: 'loading' });
  const [notice, setNotice] = useState<MutationNotice>(null);
  const [editing, setEditing] = useState<Post | null>(null);
  const [busy, setBusy] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setStatus({ state: 'loading' });
    void listPosts
      .execute({ page: controls.page, limit: controls.limit, sort: controls.sort, tag: controls.tag === '' ? undefined : controls.tag })
      .then((result) => {
        if (cancelled) return;
        if (result.ok) {
          setStatus({ state: 'ready', page: result.value });
        } else {
          setStatus({ state: 'error', failure: result.failure });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [listPosts, controls, reloadKey]);

  const reload = () => setReloadKey((current) => current + 1);

  const runMutation = (
    promise: Promise<{ ok: true } | { ok: false; failure: Failure }>,
    okText: () => string,
  ) => {
    setBusy(true);
    void promise.then((result) => {
      setBusy(false);
      if (result.ok) {
        setNotice({ tone: 'ok', text: okText() });
        reload();
      } else {
        setNotice({
          tone: 'error',
          text: result.failure.message,
          code: result.failure.code,
        });
      }
    });
  };

  return {
    status,
    notice,
    editing,
    busy,
    controls,
    setSort: (sort) => setControls((current) => ({ ...current, sort, page: 1 })),
    setLimit: (limit) => setControls((current) => ({ ...current, limit, page: 1 })),
    applyFilter: (tag) => setControls((current) => ({ ...current, tag: tag.trim(), page: 1 })),
    goToPage: (page) => setControls((current) => ({ ...current, page })),
    create: (input) => {
      setEditing(null);
      runMutation(createPosts.execute(input), () => '201 Created — the post is live on the server');
    },
    startEdit: (post) => setEditing(post),
    submitEdit: (input) => {
      const target = editing;
      setEditing(null);
      if (target === null) return;
      runMutation(replacePosts.execute(target.id, input), () => `200 Replaced ${target.id} — the whole next state`);
    },
    cancelEdit: () => setEditing(null),
    remove: (id) => {
      setEditing(null);
      runMutation(deletePosts.execute(id), () => `204 No Content — ${id} is gone`);
    },
    retry: reload,
  };
}

/* ── the workbench view ────────────────────────────────────────────── */

function PostForm({
  initial,
  busy,
  submitLabel,
  onSubmit,
  onCancel,
}: {
  initial?: Post;
  busy: boolean;
  submitLabel: string;
  onSubmit: (input: PostInput) => void;
  onCancel?: () => void;
}) {
  const [title, setTitle] = useState(initial?.title ?? '');
  const [body, setBody] = useState(initial?.body ?? '');
  const [tags, setTags] = useState(initial?.tags.join(', ') ?? '');

  const submit = () => {
    onSubmit({
      title,
      body,
      tags: tags
        .split(',')
        .map((tag) => tag.trim())
        .filter((tag) => tag !== ''),
    });
  };

  return (
    <form
      data-testid={initial === undefined ? 'pw-create-form' : 'pw-edit-form'}
      className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-sm py-sm"
      onSubmit={(event) => {
        event.preventDefault();
        submit();
      }}
    >
      <label className="flex flex-col gap-xxs text-caption text-on-surface-variant">
        title
        <input
          value={title}
          onChange={(event) => setTitle(event.target.value)}
          className="rounded-md border border-outline bg-surface px-sm py-xxs font-mono text-body-sm text-on-surface"
          placeholder="A post from the workbench"
        />
      </label>
      <label className="flex flex-col gap-xxs text-caption text-on-surface-variant">
        body
        <textarea
          value={body}
          onChange={(event) => setBody(event.target.value)}
          rows={3}
          className="rounded-md border border-outline bg-surface px-sm py-xxs font-mono text-body-sm text-on-surface"
          placeholder="The whole next state of this post."
        />
      </label>
      <label className="flex flex-col gap-xxs text-caption text-on-surface-variant">
        tags (comma-separated — normalized on write)
        <input
          value={tags}
          onChange={(event) => setTags(event.target.value)}
          className="rounded-md border border-outline bg-surface px-sm py-xxs font-mono text-body-sm text-on-surface"
          placeholder="api, design"
        />
      </label>
      <div className="flex gap-xs">
        <button
          type="submit"
          disabled={busy}
          className="rounded-pill border border-outline px-sm py-xxs font-mono text-caption uppercase tracking-wide text-on-surface-variant transition-colors hover:border-primary hover:text-primary disabled:opacity-50"
        >
          {submitLabel}
        </button>
        {onCancel !== undefined && (
          <button
            type="button"
            onClick={onCancel}
            className="rounded-pill border border-outline px-sm py-xxs font-mono text-caption uppercase tracking-wide text-on-surface-variant transition-colors hover:border-primary hover:text-primary"
          >
            Cancel
          </button>
        )}
      </div>
    </form>
  );
}

function Workbench({ actions }: { actions: WorkbenchActions }) {
  const [tagDraft, setTagDraft] = useState(actions.controls.tag);

  // The ready branch's page is extracted into its own component so the
  // narrowed discriminated union survives into the onClick closures
  // (TypeScript narrowing does not cross function boundaries).
  const readyPage = actions.status.state === 'ready' ? actions.status.page : null;

  return (
    <section data-testid="pw-workbench" className="flex flex-col gap-sm">
      <h4 className="font-display text-title-sm text-on-surface">
        The workbench — every button is a real request to /api/posts
      </h4>

      <div data-testid="pw-controls" className="flex flex-wrap items-center gap-xs">
        <form
          className="flex items-center gap-xxs"
          onSubmit={(event) => {
            event.preventDefault();
            actions.applyFilter(tagDraft);
          }}
        >
          <input
            value={tagDraft}
            onChange={(event) => setTagDraft(event.target.value)}
            aria-label="tag filter"
            className="w-32 rounded-md border border-outline bg-surface px-sm py-xxs font-mono text-caption text-on-surface"
            placeholder="tag filter"
          />
          <button
            type="submit"
            className="rounded-pill border border-outline px-sm py-xxs font-mono text-caption uppercase tracking-wide text-on-surface-variant transition-colors hover:border-primary hover:text-primary"
          >
            Filter
          </button>
        </form>
        <label className="flex items-center gap-xxs text-caption text-on-surface-variant">
          sort
          <select
            value={actions.controls.sort}
            onChange={(event) => actions.setSort(event.target.value as PostSort)}
            className="rounded-md border border-outline bg-surface px-sm py-xxs font-mono text-caption text-on-surface"
          >
            {POST_SORTS.map((sort) => (
              <option key={sort} value={sort}>
                {sort}
              </option>
            ))}
          </select>
        </label>
        <label className="flex items-center gap-xxs text-caption text-on-surface-variant">
          limit
          <select
            value={String(actions.controls.limit)}
            onChange={(event) => actions.setLimit(Number(event.target.value))}
            className="rounded-md border border-outline bg-surface px-sm py-xxs font-mono text-caption text-on-surface"
          >
            {[3, 5, 10].map((limit) => (
              <option key={limit} value={limit}>
                {limit}
              </option>
            ))}
          </select>
        </label>
      </div>

      {actions.status.state === 'loading' ? (
        <p data-testid="pw-loading" className="text-body-sm text-on-surface-variant">
          loading page {actions.controls.page}…
        </p>
      ) : actions.status.state === 'error' ? (
        <ErrorPanel subject="the posts workbench" failure={actions.status.failure} onRetry={actions.retry} />
      ) : readyPage !== null ? (
        <>
          <p data-testid="pw-page-meta" className="font-mono text-caption text-on-surface-variant">
            page {readyPage.page} of {Math.max(readyPage.totalPages, 1)} · {readyPage.totalItems} items
            {actions.controls.tag !== '' && ` · tag:${actions.controls.tag}`}
          </p>
          <div data-testid="pw-table" className="overflow-x-auto rounded-md border border-outline">
            <table className="w-full text-left text-caption">
              <thead className="bg-surface-tint text-on-surface-variant">
                <tr>
                  <th scope="col" className="px-sm py-xxs">id</th>
                  <th scope="col" className="px-sm py-xxs">title</th>
                  <th scope="col" className="px-sm py-xxs">tags</th>
                  <th scope="col" className="px-sm py-xxs">updated</th>
                  <th scope="col" className="px-sm py-xxs" />
                </tr>
              </thead>
              <tbody>
                {readyPage.items.length === 0 ? (
                  <tr className="border-t border-outline">
                    <td colSpan={5} className="px-sm py-xxs text-on-surface-variant">
                      items: [] — an honest empty page (past the end or filtered to nothing)
                    </td>
                  </tr>
                ) : (
                  readyPage.items.map((post) => (
                    <tr key={post.id} className="border-t border-outline">
                      <td className="px-sm py-xxs font-mono text-secondary">{post.id}</td>
                      <td className="px-sm py-xxs text-on-surface">
                        {post.title}
                        <span className="block whitespace-normal text-caption text-on-surface-variant">{post.body}</span>
                      </td>
                      <td className="px-sm py-xxs font-mono text-on-surface-variant">
                        {post.tags.map((tag) => (
                          <span key={tag} className="mr-xxs rounded border border-outline px-xxs py-xxs">
                            {tag}
                          </span>
                        ))}
                      </td>
                      <td className="px-sm py-xxs font-mono text-on-surface-variant">{post.updatedAt.slice(0, 10)}</td>
                      <td className="px-sm py-xxs">
                        <div className="flex gap-xxs">
                          <button
                            type="button"
                            onClick={() => actions.startEdit(post)}
                            className="rounded-pill border border-outline px-sm py-xxs font-mono uppercase tracking-wide text-on-surface-variant transition-colors hover:border-primary hover:text-primary"
                          >
                            Edit
                          </button>
                          <button
                            type="button"
                            disabled={actions.busy}
                            onClick={() => actions.remove(post.id)}
                            className="rounded-pill border border-outline px-sm py-xxs font-mono uppercase tracking-wide text-on-surface-variant transition-colors hover:border-error hover:text-error disabled:opacity-50"
                          >
                            Delete
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
          <div className="flex items-center gap-xs">
            <button
              type="button"
              disabled={!readyPage.hasPrevPage}
              onClick={() => actions.goToPage(readyPage.page - 1)}
              className="rounded-pill border border-outline px-sm py-xxs font-mono text-caption uppercase tracking-wide text-on-surface-variant transition-colors hover:border-primary hover:text-primary disabled:opacity-50"
            >
              ← prev
            </button>
            <button
              type="button"
              disabled={!readyPage.hasNextPage}
              onClick={() => actions.goToPage(readyPage.page + 1)}
              className="rounded-pill border border-outline px-sm py-xxs font-mono text-caption uppercase tracking-wide text-on-surface-variant transition-colors hover:border-primary hover:text-primary disabled:opacity-50"
            >
              next →
            </button>
          </div>
        </>
      ) : null}

      {actions.notice !== null && (
        <p
          data-testid="pw-notice"
          className={`rounded-md border px-sm py-xxs font-mono text-caption ${
            actions.notice.tone === 'ok'
              ? 'border-outline text-on-surface-variant'
              : 'border-error text-error'
          }`}
        >
          {actions.notice.tone === 'error' && actions.notice.code !== undefined && `${actions.notice.code} — `}
          {actions.notice.text}
        </p>
      )}

      {actions.editing !== null ? (
        <PostForm
          key={actions.editing.id}
          initial={actions.editing}
          busy={actions.busy}
          submitLabel={`PUT /api/posts/${actions.editing.id}`}
          onSubmit={actions.submitEdit}
          onCancel={actions.cancelEdit}
        />
      ) : (
        <PostForm busy={actions.busy} submitLabel="POST /api/posts" onSubmit={actions.create} />
      )}
    </section>
  );
}

/* ── the demo component ────────────────────────────────────────────── */

export function PostsWorkbenchDemo() {
  const card = usePostsWorkbenchCard();
  const workbench = usePostsWorkbench();

  return (
    <div className="flex flex-col gap-md">
      {card.status.state === 'idle' || card.status.state === 'loading' ? (
        <p data-testid="pw-card-loading" className="text-body-sm text-on-surface-variant">
          loading the contract card…
        </p>
      ) : card.status.state === 'error' ? (
        <ErrorPanel subject="the contract card" failure={card.status.failure} onRetry={card.retry} />
      ) : (
        <ContractCard data={card.status.data} />
      )}
      <Workbench actions={workbench} />
    </div>
  );
}

export default PostsWorkbenchDemo;
