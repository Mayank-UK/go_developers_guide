import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetAuditDemoUseCase,
  DB_AUDIT_DEMO_USE_CASE,
} from '../../../domain/demo/get-db-audit-demo-usecase';
import type { AuditDemoData } from '../../../domain/demo/db-audit';

/**
 * AuditDemo — the databases-sql topic's in-transaction audit demo (demo
 * block id `db:audit`): the commit phase whose change and history land
 * together, the rollback phase whose doomed audit row vanishes with its
 * change — both verdicts derived from the observed world.
 */

type Status =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: AuditDemoData }
  | { state: 'error'; failure: Failure };

export function useAuditDemo(
  useCase: GetAuditDemoUseCase = container.resolve(DB_AUDIT_DEMO_USE_CASE),
): { status: Status; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<Status>({ state: 'idle' });

  useEffect(() => {
    let cancelled = false;
    setStatus({ state: 'loading' });
    void useCase.execute().then((result) => {
      if (cancelled) return;
      if (result.ok) setStatus({ state: 'ready', data: result.value });
      else setStatus({ state: 'error', failure: result.failure });
    });
    return () => {
      cancelled = true;
    };
  }, [useCase, attempt]);

  const retry = () => setAttempt((current) => current + 1);
  return { status, retry };
}

export function AuditDemo() {
  const { status, retry } = useAuditDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="audit-loading" className="text-body-sm text-on-surface-variant">
        committing and rolling back, together…
      </p>
    );
  }
  if (status.state === 'error') {
    return <ErrorPanel subject="the audit demo" failure={status.failure} onRetry={retry} />;
  }

  const { steps, committedTogether, rolledBackTogether, summary } = status.data;
  return (
    <div className="flex flex-col gap-md" data-testid="audit-ready">
      <section data-testid="audit-verdicts" className="flex flex-wrap gap-xs">
        <span className={`rounded px-xs py-xxs font-mono text-caption ${committedTogether ? 'bg-secondary text-surface' : 'bg-surface-tint text-on-surface-variant'}`}>
          {committedTogether ? '✓ committed together' : '✗ commit proof failed'}
        </span>
        <span className={`rounded px-xs py-xxs font-mono text-caption ${rolledBackTogether ? 'bg-secondary text-surface' : 'bg-surface-tint text-on-surface-variant'}`}>
          {rolledBackTogether ? '✓ rolled back together' : '✗ rollback proof failed'}
        </span>
      </section>

      <ol data-testid="audit-steps" className="flex flex-col gap-xs">
        {steps.map((step) => (
          <li key={step.phase} className="flex flex-col gap-xxs rounded-md border border-outline px-sm py-xxs">
            <div className="flex flex-wrap items-baseline gap-xs">
              <code className="rounded bg-surface-tint px-xs py-xxs font-mono text-caption text-on-surface">{step.action}</code>
              <span className="font-mono text-body-sm text-on-surface">title: {step.liveTitle}</span>
              <span data-testid="audit-count" className="ml-auto rounded bg-surface-tint px-xs py-xxs font-mono text-caption text-secondary">
                audit rows = {step.auditRows}
              </span>
            </div>
            <p className="text-caption text-on-surface-variant">{step.note}</p>
          </li>
        ))}
      </ol>

      <p className="text-body-sm text-on-surface-variant">{summary}</p>
    </div>
  );
}

export default AuditDemo;
