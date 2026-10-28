import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import type {
  ArchSagaLedgerRow,
  ArchSagaRunPanel,
  ArchSagaDemoData,
} from '../../../domain/demo/arch-saga';
import {
  GetArchSagaDemoUseCase,
  ARCH_SAGA_DEMO_USE_CASE,
} from '../../../domain/demo/get-arch-saga-demo-usecase';

/**
 * ArchSagaDemo — the advanced-architecture topic's saga demo (demo
 * block id `arch:saga`). It renders the payload of
 * GET /api/demo/arch/saga: the fixed order-fulfillment script, the
 * three observed runs of the REAL patterns.SagaRunner — success,
 * injected failure with the idempotent re-compensation replay, and
 * mid-saga cancellation whose undo pass survives the dead context —
 * each with its full ledger, the world's log of effects, and the
 * counted totals, then the observed semantics, the machinery's
 * exported surface, and the honesty note. The runs are deterministic
 * functions of the fixed script, so the replay button re-serves
 * identical ledgers — which is itself the idempotency lesson. The
 * client never teaches; it renders what the server observed.
 */

type SagaStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: ArchSagaDemoData }
  | { state: 'error'; failure: Failure };

/** outcomeTone colors a ledger outcome for scanning. */
function outcomeTone(outcome: string): string {
  if (outcome === 'executed' || outcome === 'compensated') return 'text-primary';
  if (outcome === 'failed' || outcome === 'compensation-failed') return 'text-error';
  if (outcome === 'skipped') return 'text-on-surface-variant';
  return 'text-secondary'; // already-compensated no-ops, nothing-to-undo
}

/** One ledger row: the sequence, the step, the phase, the outcome, the detail. */
function LedgerLine({ row }: { row: ArchSagaLedgerRow }) {
  return (
    <div className="flex flex-col gap-xxs" data-testid={`arch-saga-row-${String(row.seq)}`}>
      <p className="font-mono text-caption text-on-surface">
        <span className="text-on-surface-variant">{row.seq}.</span> {row.step}{' '}
        <span className="text-on-surface-variant">{row.phase}</span>{' '}
        <span className={`uppercase ${outcomeTone(row.outcome)}`}>{row.outcome}</span>
      </p>
      <p className="font-mono text-caption text-on-surface-variant pl-md">{row.detail}</p>
    </div>
  );
}

/** One observed run: status, trigger, ledger, world log, counts, replay. */
function RunPanel({ run }: { run: ArchSagaRunPanel }) {
  return (
    <section
      data-testid={`arch-saga-run-${run.id}`}
      className="flex flex-col gap-xs rounded-md border border-outline px-md py-sm"
    >
      <p className="text-body-sm font-medium text-on-surface">
        {run.title} — <span className="font-mono">{run.status}</span>
      </p>
      <p className="text-caption text-on-surface-variant">{run.lead}</p>
      <p className="font-mono text-caption text-on-surface-variant">{run.script}</p>
      {run.trigger !== undefined && (
        <p className="font-mono text-caption text-error" data-testid={`arch-saga-trigger-${run.id}`}>
          trigger: {run.trigger}
        </p>
      )}

      <div className="flex flex-col gap-xxs" data-testid={`arch-saga-ledger-${run.id}`}>
        <p className="text-caption font-medium text-on-surface">the outcomes ledger</p>
        {run.ledger.map((row) => (
          <LedgerLine key={row.seq} row={row} />
        ))}
      </div>

      <div className="flex flex-col gap-xxs" data-testid={`arch-saga-world-${run.id}`}>
        <p className="text-caption font-medium text-on-surface">the world's log (in order)</p>
        {run.worldLog.map((line) => (
          <p key={line} className="font-mono text-caption text-on-surface-variant pl-md">
            {line}
          </p>
        ))}
      </div>

      {run.replay !== undefined && (
        <div className="flex flex-col gap-xxs" data-testid={`arch-saga-run-replay-${run.id}`}>
          <p className="text-caption font-medium text-on-surface">
            the idempotent re-compensation replay
          </p>
          {run.replay.map((row) => (
            <LedgerLine key={row.seq} row={row} />
          ))}
        </div>
      )}

      {run.counts.map((count) => (
        <p
          key={count.label}
          className="font-mono text-caption text-on-surface pl-md"
          data-testid={`arch-saga-count-${run.id}`}
        >
          {String(count.value)}× {count.label}
        </p>
      ))}

      <p className="text-caption text-on-surface-variant">{run.note}</p>
    </section>
  );
}

export function useArchSagaDemo(
  useCase: GetArchSagaDemoUseCase = container.resolve(ARCH_SAGA_DEMO_USE_CASE),
): { status: SagaStatus; replay: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<SagaStatus>({ state: 'idle' });

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

  const replay = () => setAttempt((current) => current + 1);
  return { status, replay };
}

export function ArchSagaDemo() {
  const { status, replay } = useArchSagaDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="arch-saga-loading" className="text-body-sm text-on-surface-variant">
        driving the saga runs…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the saga demo" failure={status.failure} onRetry={replay} />;
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid="arch-saga-ready">
      <p className="text-caption text-on-surface-variant" data-testid="arch-saga-mode">
        {data.mode}
      </p>

      <section
        data-testid="arch-saga-script"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="text-body-sm font-medium text-on-surface">
          The fixed script ({String(data.steps.length)} steps — source {data.source})
        </p>
        {data.steps.map((step) => (
          <div key={step.name} className="flex flex-col gap-xxs" data-testid={`arch-saga-step-${String(step.order)}`}>
            <p className="font-mono text-caption text-on-surface">
              {String(step.order)} · {step.name}
            </p>
            <p className="text-caption text-on-surface-variant pl-md">does: {step.forward}</p>
            <p className="text-caption text-on-surface-variant pl-md">
              undoes: {step.undo === '' ? '— nothing to undo (an externalized fact, placed last)' : step.undo}
            </p>
          </div>
        ))}
      </section>

      {data.runs.map((run) => (
        <RunPanel key={run.id} run={run} />
      ))}

      <section
        data-testid="arch-saga-semantics"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <p className="text-body-sm font-medium text-on-surface">The observed semantics</p>
        {data.semantics.map((rule) => (
          <p key={rule} className="text-caption text-on-surface-variant">
            {rule}
          </p>
        ))}
      </section>

      <section
        data-testid="arch-saga-surface"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <p className="text-body-sm font-medium text-on-surface">
          The machinery's exported surface ({String(data.surface.length)} ids)
        </p>
        {data.surface.map((row) => (
          <p key={row.id} className="font-mono text-caption text-on-surface-variant">
            {row.id} ({row.kind}) — {row.role}
          </p>
        ))}
      </section>

      <p data-testid="arch-saga-honesty" className="text-body-sm text-on-surface-variant">
        {data.honestyNote}
      </p>

      <button
        type="button"
        onClick={replay}
        data-testid="arch-saga-replay-button"
        className="self-start rounded-md border border-outline px-md py-xxs text-caption text-on-surface hover:bg-surface-tint"
      >
        re-drive the saga
      </button>
    </div>
  );
}

export default ArchSagaDemo;
