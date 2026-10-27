import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { JobsConsoleDemo } from './console_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetJobsConsoleDemoUseCase,
  JOBS_CONSOLE_DEMO_USE_CASE,
} from '../../../domain/demo/get-jobs-console-demo-usecase';
import type { JobsConsoleDemoData } from '../../../domain/demo/jobs-console';

/**
 * JobsConsoleDemo spec — fixture-driven (happy + failure): the component
 * resolves its use case through the container token (the documented
 * register-a-singleton seam), so the spec swaps that registration
 * instead of stubbing fetch. Behavior, not implementation: the task
 * ladders render with their attempt sequence and verdicts, the beats
 * with their commands and outcomes, the cron cadence and the two
 * observability numbers render, and failures — including the server's
 * honest 503 for an unreachable Redis or a not-running worker — degrade
 * to the shared error panel with recovery.
 */

const fixture: JobsConsoleDemoData = {
  namespace: 'gdg:demo:jobs:0123456789ab',
  queues: [
    { name: 'demos', weight: 6 },
    { name: 'low', weight: 1 },
  ],
  commands: ['ENQUEUE', 'RETRY', 'ARCHIVE', 'RUN', 'DELETE', 'REGISTER', 'STATS'],
  tasks: [
    {
      id: 'gdg:demo:jobs:0123456789ab:flaky',
      type: 'demo:flaky',
      queue: 'demos',
      maxRetry: 2,
      attempts: [
        { attempt: 1, outcome: 'failed' },
        { attempt: 2, outcome: 'failed' },
        { attempt: 3, outcome: 'succeeded' },
      ],
      verdict: 'succeeded after 3 attempt(s) — the retry ladder worked as designed',
    },
    {
      id: 'gdg:demo:jobs:0123456789ab:poison',
      type: 'demo:poison',
      queue: 'low',
      maxRetry: 1,
      attempts: [
        { attempt: 1, outcome: 'failed' },
        { attempt: 2, outcome: 'failed' },
      ],
      verdict: 'failed every attempt (2 before the replay, 1 after) — archived, replayed once, re-archived, then deleted',
    },
  ],
  beats: [
    {
      beat: 'enqueue',
      commands: ['ENQUEUE demo:flaky … Queue(demos) MaxRetry(2)'],
      outcome: 'two tasks accepted by the broker.',
    },
    {
      beat: 'replay',
      commands: ['RUN gdg:demo:jobs:0123456789ab:poison (archived → pending)'],
      outcome: 'the archived task was replayed once and re-executed for real.',
    },
  ],
  cron: { spec: '@every 1s', ticks: 2 },
  observability: { peakDepth: 3, peakOldestAgeMs: 640, finalDepth: 0, finalOldestAgeMs: 0 },
  proofs: {
    retry: 'the flaky task failed 2 attempt(s) and succeeded on attempt 3.',
    archive: 'the archive now holds the poison task with its final error on record.',
    replay: 'the ledger grew by 1 attempt — replay is a real re-execution.',
    cron: 'the ledger observed 2 tick task(s) fired.',
    observability: 'depth peaked at 3; at rest the queues answer 0.',
  },
  reasoning: 'failure is a first-class state — the task comes back with a growing delay.',
};

function stubUseCase(execute: () => Promise<Result<JobsConsoleDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    JOBS_CONSOLE_DEMO_USE_CASE,
    { execute } as unknown as GetJobsConsoleDemoUseCase,
  );
}

describe('JobsConsoleDemo', () => {
  it('renders loading first, then the whole choreography', async () => {
    stubUseCase(async () => success(fixture));

    render(<JobsConsoleDemo />);

    expect(screen.getByTestId('jobs-console-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('jobs-console-ready')).toBeInTheDocument());

    // The task ladders, each with its attempt sequence and verdict.
    const flaky = screen.getByTestId('jobs-console-task-gdg-demo-jobs-0123456789ab-flaky');
    expect(flaky).toHaveTextContent('1:failed → 2:failed → 3:succeeded');
    expect(flaky).toHaveTextContent('the retry ladder worked as designed');
    const poison = screen.getByTestId('jobs-console-task-gdg-demo-jobs-0123456789ab-poison');
    expect(poison).toHaveTextContent('1:failed → 2:failed');
    expect(poison).toHaveTextContent('re-archived, then deleted');

    // The beats, with their commands and outcomes.
    expect(screen.getByTestId('jobs-console-beat-enqueue')).toHaveTextContent(
      'Queue(demos) MaxRetry(2)',
    );
    expect(screen.getByTestId('jobs-console-beat-replay')).toHaveTextContent(
      're-executed for real',
    );

    // The cron cadence and the two numbers, then the proofs.
    expect(screen.getByTestId('jobs-console-cron')).toHaveTextContent('@every 1s');
    expect(screen.getByTestId('jobs-console-cron')).toHaveTextContent('2');
    expect(screen.getByTestId('jobs-console-observability')).toHaveTextContent('depth peak 3');
    expect(screen.getByTestId('jobs-console-observability')).toHaveTextContent('at rest 0');
    expect(screen.getByTestId('jobs-console-proofs')).toHaveTextContent('succeeded on attempt 3');
    expect(screen.getByTestId('jobs-console-proofs')).toHaveTextContent('real re-execution');
    expect(screen.getByTestId('jobs-console-reasoning')).toHaveTextContent('first-class state');
  });

  it('degrades to the error panel on the server\u2019s honest 503, with recovery', async () => {
    const failure = new ServerFailure(
      'The jobs console demo runs a real choreography against the guide\u2019s Redis, which is not reachable right now.',
      { code: 'service_unavailable', status: 503, requestId: 'rid-jobs-503' },
    );
    stubUseCase(async () => failed(failure));

    render(<JobsConsoleDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('service_unavailable');
    expect(panel).toHaveTextContent('not reachable right now');

    // Retry re-runs the load through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('jobs-console-ready')).toBeInTheDocument());
  });
});
