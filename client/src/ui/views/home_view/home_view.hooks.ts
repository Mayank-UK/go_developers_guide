import { useCallback, useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import type {
  ContentIndex,
  TopicGroup,
  TopicSummary,
} from '../../../domain/content/content';
import type { ListContentUseCase } from '../../../domain/content/list-content-usecase';
import type { GetHealthUseCase } from '../../../domain/health/get-health-usecase';

/* ---- health chip (hero) — the Phase 0 pipeline, now container-wired ---- */

export type HealthChipState =
  | { state: 'checking' }
  | { state: 'ok'; status: string; env: string }
  | { state: 'fail'; failure: Failure };

/**
 * useHealth — probes the server liveness endpoint once per mount and narrows
 * the Result into a three-state chip model. The use case is injected as a
 * default parameter (react-guide `usePosts` style) so specs can substitute
 * fakes; production resolves it from the container, never by importing data/.
 */
export function useHealth(
  getHealthUseCase: GetHealthUseCase = container.getHealthUseCase,
): HealthChipState {
  const [health, setHealth] = useState<HealthChipState>({ state: 'checking' });

  useEffect(() => {
    let cancelled = false;

    void getHealthUseCase.execute().then((result) => {
      if (cancelled) return;
      if (result.ok) {
        setHealth({ state: 'ok', status: result.value.status, env: result.value.env });
      } else {
        setHealth({ state: 'fail', failure: result.failure });
      }
    });

    return () => {
      cancelled = true;
    };
  }, [getHealthUseCase]);

  return health;
}

/* ---- content index (topic map) ---- */

/**
 * The topic map's render statuses. `idle` is the pre-request state (before the
 * load effect first runs); `empty` is a VALID served state — an index with
 * zero topics means the registry is still being authored, not that anything
 * failed.
 */
export type TopicMapStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; index: ContentIndex }
  | { state: 'empty' }
  | { state: 'error'; failure: Failure };

export interface TopicMapState {
  status: TopicMapStatus;
  /** Re-runs the index load (error recovery). */
  retry: () => void;
}

/**
 * useContentIndex — loads the curriculum index for the home screen's topic
 * map and narrows the Result into render statuses. Expected failures arrive as
 * typed Failures inside Results — the hook never throws — and `retry` bumps
 * the attempt counter to re-run the load. The use case is injected as a
 * default parameter (react-guide `usePosts` style) so specs can substitute
 * fakes.
 *
 * A ready index is rendered exactly as served — the authored curriculum is
 * the whole map; an EMPTY served registry is its own distinct landing-soon
 * state.
 */
export function useContentIndex(
  listContentUseCase: ListContentUseCase = container.listContentUseCase,
): TopicMapState {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<TopicMapStatus>({ state: 'idle' });

  useEffect(() => {
    let cancelled = false;
    setStatus({ state: 'loading' });

    void listContentUseCase.execute().then((result) => {
      if (cancelled) return;
      if (result.ok) {
        setStatus(
          result.value.topics.length === 0
            ? { state: 'empty' }
            : { state: 'ready', index: result.value },
        );
      } else {
        setStatus({ state: 'error', failure: result.failure });
      }
    });

    return () => {
      cancelled = true;
    };
  }, [listContentUseCase, attempt]);

  const retry = useCallback(() => setAttempt((current) => current + 1), []);

  return { status, retry };
}

export interface GroupedTopics {
  group: TopicGroup;
  topics: TopicSummary[];
}

/**
 * Groups the flat topic list under its group, in display order. The server
 * already sends both ordered; sorting here keeps the client's rendering
 * contract explicit instead of trusting wire order.
 */
export function groupTopics(index: ContentIndex): GroupedTopics[] {
  const byGroup = new Map<string, TopicSummary[]>();
  for (const topic of index.topics) {
    const bucket = byGroup.get(topic.group) ?? [];
    bucket.push(topic);
    byGroup.set(topic.group, bucket);
  }

  return [...index.groups]
    .sort((a, b) => a.order - b.order)
    .map((group) => ({
      group,
      topics: (byGroup.get(group.id) ?? []).sort((a, b) => a.order - b.order),
    }));
}
