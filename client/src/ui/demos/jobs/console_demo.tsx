import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetJobsConsoleDemoUseCase,
  JOBS_CONSOLE_DEMO_USE_CASE,
} from '../../../domain/demo/get-jobs-console-demo-usecase';
import type {
  JobsAttempt,
  JobsConsoleBeat,
  JobsConsoleDemoData,
  JobsTask,
} from '../../../domain/demo/jobs-console';

/**
 * JobsConsoleDemo — the background-jobs topic's live demo (demo block
 * id `jobs:console`). It renders the payload of GET
 * /api/demo/jobs/console: one scripted walk of the whole choreography
 * against the RUNNING stack Redis and the hosted worker — the flaky
 * task's fail-fail-succeed attempt ladder under exponential backoff,
 * the poison task exhausting its budget into the dead-letter archive, a
 * manual replay that re-runs it for real and re-archives, the delayed
 * digest maturing out of the scheduled state, per-second cron ticks,
 * and the two observability numbers sampled while it all ran.
 *
 * Every load mints a fresh throwaway namespace server-side, so the
 * Retry button walks a genuinely new choreography; an unreachable Redis
 * (or a not-running worker) degrades to the typed error panel (the
 * server's honest 503), never to a stale answer.
 */

type ConsoleStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: JobsConsoleDemoData }
  | { state: 'error'; failure: Failure };

export function useJobsConsoleDemo(
  useCase: GetJobsConsoleDemoUseCase = container.resolve(JOBS_CONSOLE_DEMO_USE_CASE),
): { status: ConsoleStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<ConsoleStatus>({ state: 'idle' });

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

  const retry = () => setAttempt((current) => current + 1);
  return { status, retry };
}

export function JobsConsoleDemo() {
  const { status, retry } = useJobsConsoleDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="jobs-console-loading" className="text-body-sm text-on-surface-variant">
        walking the choreography against the live broker and worker…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the jobs console demo" failure={status.failure} onRetry={retry} />;
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid="jobs-console-ready">
      <p className="font-mono text-caption text-on-surface-variant">
        run <span className="text-secondary">{data.namespace}</span> over asynq and the guide&apos;s
        Redis — queues{' '}
        {data.queues.map((queue, index) => (
          <span key={queue.name}>
            {index > 0 ? ' · ' : ''}
            <span className="text-secondary">{queue.name}</span> (weight {queue.weight})
          </span>
        ))}{' '}
        — throwaway per run, removed on the way out
      </p>

      {data.tasks.map((task) => (
        <TaskLadder key={task.id} task={task} />
      ))}

      {data.beats.map((beat) => (
        <ConsoleBeat key={beat.beat} beat={beat} />
      ))}

      <p data-testid="jobs-console-cron" className="font-mono text-caption text-on-surface-variant">
        scheduler <span className="text-secondary">{data.cron.spec}</span> fired{' '}
        <span className="text-secondary">{String(data.cron.ticks)}</span> tick task(s) while the
        entry lived
      </p>

      <section
        data-testid="jobs-console-observability"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-primary">the two numbers that page</p>
        <p className="font-mono text-caption text-on-surface">
          depth peak <span className="text-secondary">{String(data.observability.peakDepth)}</span>{' '}
          (oldest {String(data.observability.peakOldestAgeMs)} ms) · at rest{' '}
          <span className={data.observability.finalDepth === 0 ? 'text-primary' : 'text-error'}>
            {String(data.observability.finalDepth)}
          </span>{' '}
          (oldest {String(data.observability.finalOldestAgeMs)} ms)
        </p>
      </section>

      <section
        data-testid="jobs-console-proofs"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-primary">the five derived proofs</p>
        <p className="text-caption text-on-surface">{data.proofs.retry}</p>
        <p className="text-caption text-on-surface">{data.proofs.archive}</p>
        <p className="text-caption text-on-surface">{data.proofs.replay}</p>
        <p className="text-caption text-on-surface">{data.proofs.cron}</p>
        <p className="text-caption text-on-surface">{data.proofs.observability}</p>
      </section>

      <p data-testid="jobs-console-reasoning" className="text-body-sm text-on-surface-variant">
        {data.reasoning}
      </p>
    </div>
  );
}

function TaskLadder({ task }: { task: JobsTask }) {
  return (
    <section
      data-testid={`jobs-console-task-${slug(task.id)}`}
      className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
    >
      <p className="font-mono text-caption text-on-surface">
        <span className="text-secondary">{task.type}</span> on {task.queue} · MaxRetry{' '}
        {String(task.maxRetry)} · <span className="text-secondary">{ladderOf(task.attempts)}</span>
      </p>
      <p className="text-caption text-on-surface">{task.verdict}</p>
    </section>
  );
}

function ConsoleBeat({ beat }: { beat: JobsConsoleBeat }) {
  return (
    <section
      data-testid={`jobs-console-beat-${slug(beat.beat)}`}
      className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
    >
      <p className="font-mono text-caption text-on-surface">
        <span className="text-secondary">{beat.beat}</span>
      </p>
      {beat.commands.map((command) => (
        <p key={command} className="font-mono text-caption text-on-surface-variant">
          {command}
        </p>
      ))}
      <p className="font-mono text-caption text-on-surface">{beat.outcome}</p>
    </section>
  );
}

/** ladderOf renders an attempt ladder: 1:failed → 2:failed → 3:succeeded. */
function ladderOf(attempts: JobsAttempt[]): string {
  if (attempts.length === 0) return 'no attempts recorded';
  return attempts.map((a) => `${String(a.attempt)}:${a.outcome}`).join(' → ');
}

/** slug turns a task or beat label into a stable testid fragment. */
function slug(label: string): string {
  return label
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

export default JobsConsoleDemo;
