import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetTestingRunnerDemoUseCase,
  TESTING_RUNNER_DEMO_USE_CASE,
} from '../../../domain/demo/get-testing-runner-demo-usecase';
import type { TestingRunnerDemoData } from '../../../domain/demo/testing-runner';

/**
 * TestingRunnerDemo — the testing topic's runner demo (demo block id
 * `testing:runner`). It renders the payload of
 * GET /api/demo/testing/runner: the run panel (lifecycle, the exact
 * command, wall-clock age, parsed totals), the package rows (outcome +
 * counts + duration), the test timeline (subtests indented by depth,
 * failed rows carrying their excerpt), the suite's counted shape (files,
 * packages, functions, t.Parallel calls, the table-test trio located by
 * symbol), and the mode line that states honestly WHEN the run happened.
 */

type RunnerStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: TestingRunnerDemoData }
  | { state: 'error'; failure: Failure };

export function useTestingRunnerDemo(
  useCase: GetTestingRunnerDemoUseCase = container.resolve(TESTING_RUNNER_DEMO_USE_CASE),
): { status: RunnerStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<RunnerStatus>({ state: 'idle' });

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

function statusClass(status: string): string {
  switch (status) {
    case 'pass':
      return 'text-secondary';
    case 'fail':
      return 'text-primary';
    case 'skip':
      return 'text-on-surface-variant';
    default:
      return 'text-on-surface-variant';
  }
}

export function TestingRunnerDemo() {
  const { status, retry } = useTestingRunnerDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="testing-runner-loading" className="text-body-sm text-on-surface-variant">
        loading the boot-cached run's parsed timeline…
      </p>
    );
  }

  if (status.state === 'error') {
    return (
      <ErrorPanel subject="the testing runner demo" failure={status.failure} onRetry={retry} />
    );
  }

  const { data } = status;
  const ageLabel =
    data.run.state === 'done'
      ? `ran at boot · ${String(Math.round(data.run.ageSeconds))}s ago · ${String(Math.round(data.run.durationMs))}ms`
      : data.run.state === 'running'
        ? 'the boot run is still in flight'
        : 'the run could not execute';

  return (
    <div className="flex flex-col gap-md" data-testid="testing-runner-ready">
      <section
        data-testid="testing-runner-run"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="text-caption text-on-surface-variant">{data.mode}</p>
        <p className="font-mono text-caption text-on-surface">
          state <span className={statusClass(data.run.state === 'done' ? 'pass' : 'skip')}>{data.run.state}</span>{' '}
          · {ageLabel}
        </p>
        <p className="font-mono text-caption text-on-surface-variant">{data.run.command}</p>
        {data.run.state === 'done' ? (
          <p className="text-caption text-on-surface">
            totals parsed from events: <span className="text-secondary">{String(data.run.totalPassed)} passed</span> ·{' '}
            {String(data.run.totalFailed)} failed · {String(data.run.totalSkipped)} skipped
          </p>
        ) : null}
        {data.run.reason ? (
          <p className="text-caption text-on-surface-variant">{data.run.reason}</p>
        ) : null}
      </section>

      <section
        data-testid="testing-runner-packages"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        {data.packages.map((pkg) => (
          <p key={pkg.package} className="font-mono text-caption">
            <span className={statusClass(pkg.outcome)}>{pkg.outcome}</span>{' '}
            <span className="text-on-surface">{pkg.package}</span> · {String(pkg.passed)}/
            {String(pkg.failed)}/{String(pkg.skipped)} · {String(Math.round(pkg.durationMs * 10) / 10)}ms
          </p>
        ))}
      </section>

      <section
        data-testid="testing-runner-timeline"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        {data.tests.map((test) => (
          <div
            key={`${test.package}#${test.test}`}
            className="flex flex-col gap-xxs"
            style={{ marginLeft: `${test.depth * 1.25}rem` }}
          >
            <p className="font-mono text-caption">
              <span className={statusClass(test.status)}>{test.status}</span>{' '}
              <span className="text-on-surface">{test.test}</span> ·{' '}
              {String(Math.round(test.durationMs * 10) / 10)}ms
            </p>
            {test.output ? (
              <pre className="overflow-x-auto rounded-sm bg-surface-tint px-sm py-xxs font-mono text-caption text-on-surface-variant">
                {test.output}
              </pre>
            ) : null}
          </div>
        ))}
        {data.tests.length === 0 ? (
          <p className="text-caption text-on-surface-variant">
            the timeline is empty until the run lands — no invented rows ever fill it
          </p>
        ) : null}
      </section>

      <section
        data-testid="testing-runner-shape"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <p className="text-caption text-on-surface">
          the suite's shape, counted from the tree: {String(data.shape.testPackages)} packages ·{' '}
          {String(data.shape.testFiles)} files · {String(data.shape.testFunctions)} functions ·{' '}
          {String(data.shape.parallelCalls)} t.Parallel calls
        </p>
        {data.shape.tableExamples.map((example) => (
          <p
            key={example.symbol}
            className="font-mono text-caption text-on-surface-variant"
          >
            {example.present ? '✓' : '✗'} {example.symbol} — {example.file}
          </p>
        ))}
        <p className="text-caption text-on-surface-variant">{data.shape.provenance}</p>
      </section>

      <p data-testid="testing-runner-reasoning" className="text-body-sm text-on-surface-variant">
        {data.reasoning}
      </p>
    </div>
  );
}

export default TestingRunnerDemo;
