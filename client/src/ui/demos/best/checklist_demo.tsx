import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import type {
  BestChecklistDemoData,
  BestChecklistItemRow,
} from '../../../domain/demo/best-checklist';
import {
  GetBestChecklistDemoUseCase,
  BEST_CHECKLIST_DEMO_USE_CASE,
} from '../../../domain/demo/get-best-checklist-demo-usecase';

/**
 * BestChecklistDemo — the best-practices topic's self-audit (demo
 * block id `best:checklist`). It renders the payload of
 * GET /api/demo/best/checklist: the summary line (pass / fail /
 * manual, counted not spun), the verdict rows grouped by practice
 * area — each with its check kind and the real evidence that produced
 * it, manual rows carrying their why instead — the anti-checklist of
 * deliberate not-dos with reasons and artifacts, and the honesty note.
 * The audit is boot-cached on the server, so the replay button
 * re-serves the identical snapshot: the lesson replays, the verdicts
 * do not drift. The client never teaches; it renders what the server
 * computed.
 */

type PanelStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: BestChecklistDemoData }
  | { state: 'error'; failure: Failure };

/** verdictTone colors a verdict for scanning. */
function verdictTone(verdict: string): string {
  if (verdict === 'pass') return 'text-primary';
  if (verdict === 'fail') return 'text-error';
  return 'text-on-surface-variant';
}

/** kindLabel renders a row's check kind compactly. */
function kindLabel(kind: string): string {
  if (kind === 'exec') return 'tool run';
  return kind;
}

/** One verdict row: the practice, the verdict, the evidence lines. */
function ItemLine({ row }: { row: BestChecklistItemRow }) {
  return (
    <div className="flex flex-col gap-xxs" data-testid={`bp-row-${row.id}`}>
      <p className="text-body-sm text-on-surface">
        <span className={`font-mono text-caption uppercase ${verdictTone(row.verdict)}`}>
          {row.verdict}
        </span>{' '}
        <span className="text-on-surface-variant">[{kindLabel(row.checkKind)}]</span> {row.practice}
      </p>
      {row.evidence.map((line) => (
        <p key={line} className="font-mono text-caption text-on-surface-variant pl-md">
          {line}
        </p>
      ))}
      {row.whyManual !== undefined && (
        <p className="text-caption text-on-surface-variant pl-md">why manual: {row.whyManual}</p>
      )}
    </div>
  );
}

export function useBestChecklistDemo(
  useCase: GetBestChecklistDemoUseCase = container.resolve(BEST_CHECKLIST_DEMO_USE_CASE),
): { status: PanelStatus; replay: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<PanelStatus>({ state: 'idle' });

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

export function BestChecklistDemo() {
  const { status, replay } = useBestChecklistDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="best-checklist-loading" className="text-body-sm text-on-surface-variant">
        auditing the running app…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the self-audit" failure={status.failure} onRetry={replay} />;
  }

  const { data } = status;
  const computing = data.areas.length === 0;

  return (
    <div className="flex flex-col gap-md" data-testid="best-checklist-ready">
      <section
        data-testid="best-checklist-summary"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="text-caption text-on-surface-variant">{data.mode}</p>
        <p className="font-mono text-caption text-on-surface">
          {data.summary.passed} pass · {data.summary.failed} fail · {data.summary.manual} manual ·
          of {data.summary.items} rows
          {data.computedAt !== '' ? ` · computed at ${data.computedAt}` : ''}
        </p>
      </section>

      {computing && (
        <p data-testid="best-checklist-computing" className="text-body-sm text-on-surface-variant">
          the boot audit is still executing on the server — replay in a moment for the verdicts.
        </p>
      )}

      {data.areas.map((area) => (
        <section
          key={area.id}
          data-testid={`bp-area-${area.id}`}
          className="flex flex-col gap-xs rounded-md border border-outline px-md py-sm"
        >
          <p className="text-body-sm font-medium text-on-surface">{area.title}</p>
          <p className="text-caption text-on-surface-variant">{area.note}</p>
          {area.items.map((row) => (
            <ItemLine key={row.id} row={row} />
          ))}
        </section>
      ))}

      {data.notDones.length > 0 && (
        <section
          data-testid="best-checklist-not-dones"
          className="flex flex-col gap-xs rounded-md border border-outline px-md py-sm"
        >
          <p className="text-body-sm font-medium text-on-surface">The anti-checklist</p>
          {data.notDones.map((notDone) => (
            <div key={notDone.id} className="flex flex-col gap-xxs" data-testid={`bp-not-done-${notDone.id}`}>
              <p className="text-body-sm text-on-surface">not done: {notDone.notDone}</p>
              <p className="text-caption text-on-surface-variant">{notDone.reason}</p>
            </div>
          ))}
        </section>
      )}

      <p data-testid="best-checklist-honesty" className="text-body-sm text-on-surface-variant">
        {data.honestyNote}
      </p>

      <button
        type="button"
        onClick={replay}
        data-testid="best-checklist-replay"
        className="self-start rounded-md border border-outline px-md py-xxs text-caption text-on-surface hover:bg-surface-tint"
      >
        replay the audit
      </button>
    </div>
  );
}

export default BestChecklistDemo;
