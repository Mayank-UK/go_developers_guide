import { useCallback, useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ServerFailure } from '../../../core/error-handling/failures';
import type { TopicDetail } from '../../../domain/content/content';
import type { GetTopicDetailUseCase } from '../../../domain/content/get-topic-detail-usecase';

/**
 * The topic screen's render statuses. `idle` is the pre-request state (before
 * the load effect first runs); a detail fetch has no empty variant — an
 * unknown id is the server's 404 envelope, i.e. an error with a friendly
 * panel.
 */
export type TopicViewStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; topic: TopicDetail }
  | { state: 'error'; failure: Failure };

export interface TopicViewState {
  status: TopicViewStatus;
  /** Re-runs the detail load (error recovery). */
  retry: () => void;
}

/**
 * Loads the topic named by the route's :topicId param. Re-fetches when the id
 * changes (navigating between topics reuses the same screen): the effect
 * resets to loading and a `cancelled` flag drops the stale response, so a slow
 * first fetch can never overwrite the second topic's screen. A missing param
 * degrades to a friendly error instead of a request with "undefined" in the
 * URL. Expected failures arrive as typed Failures inside Results — the hook
 * never throws — and `retry` bumps the attempt counter to re-run the load.
 * The use case is injected as a default parameter (react-guide `usePosts`
 * style) so specs can substitute fakes.
 */
export function useTopicDetail(
  topicId: string | undefined,
  getTopicDetailUseCase: GetTopicDetailUseCase = container.getTopicDetailUseCase,
): TopicViewState {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<TopicViewStatus>({ state: 'idle' });

  useEffect(() => {
    if (topicId === undefined) {
      setStatus({
        state: 'error',
        failure: new ServerFailure('No topic id in this URL.', {
          code: 'not_found',
          status: 404,
        }),
      });
      return;
    }

    let cancelled = false;
    setStatus({ state: 'loading' });

    void getTopicDetailUseCase.execute(topicId).then((result) => {
      if (cancelled) return;
      if (result.ok) {
        setStatus({ state: 'ready', topic: result.value });
      } else {
        setStatus({ state: 'error', failure: result.failure });
      }
    });

    return () => {
      cancelled = true;
    };
  }, [topicId, getTopicDetailUseCase, attempt]);

  const retry = useCallback(() => setAttempt((current) => current + 1), []);

  return { status, retry };
}
