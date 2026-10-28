import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import type {
  ArchEventstoreInventoryState,
  ArchEventstoreRow,
  ArchEventstoreDemoData,
} from '../../../domain/demo/arch-eventstore';
import {
  GetArchEventstoreDemoUseCase,
  ARCH_EVENTSTORE_DEMO_USE_CASE,
} from '../../../domain/demo/get-arch-eventstore-demo-usecase';

/**
 * ArchEventstoreDemo — the advanced-architecture topic's event-store
 * exhibit (demo block id `arch:eventstore`). It renders the payload of
 * GET /api/demo/arch/eventstore: the store's API surface (the real
 * guarantees — append-only, versioned, optimistically concurrent,
 * structurally immutable), the stream's three acts (the command path,
 * the conflict and both halves of its recovery, the rebuild proofs)
 * with every row's observed outcome and stream version, the replay
 * evidence block (the fold order, the rebuilt state beside the naive
 * ledger, the byte-identical double rebuild, the absent stream's
 * zero-state fold), and the honesty note. The exhibit is a pure
 * function of fixed inputs, so the replay button re-serves identical
 * bytes — which is the demo's own determinism claim, observed on every
 * click. The client never teaches; it renders what the server observed.
 */

type PanelStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: ArchEventstoreDemoData }
  | { state: 'error'; failure: Failure };

/** outcomeTone colors an observed outcome for scanning. */
function outcomeTone(outcome: string): string {
  if (
    outcome.startsWith('appended') ||
    outcome.startsWith('loaded') ||
    outcome.startsWith('rebuilt') ||
    outcome.startsWith('byte-identical') ||
    outcome.startsWith('matches-') ||
    outcome.startsWith('zero-state')
  ) {
    return 'text-primary';
  }
  if (outcome.startsWith('refused')) return 'text-error';
  return 'text-on-surface-variant';
}

/** formatState renders one rebuilt inventory state compactly. */
function formatState(state: ArchEventstoreInventoryState): string {
  const refs = Object.entries(state.reservations)
    .map(([ref, qty]) => `${ref}:${qty}`)
    .sort()
    .join(', ');
  return `onHand ${state.onHand} · reserved ${state.reserved} · available ${state.available} · open {${refs}}`;
}

/** One observed exhibit row: the call, the outcome, the version after, the detail. */
function RunLine({ row }: { row: ArchEventstoreRow }) {
  return (
    <div className="flex flex-col gap-xxs" data-testid={`es-row-${row.step}`}>
      <p className="text-body-sm text-on-surface">
        <span className="font-mono text-caption text-on-surface-variant">{row.step}.</span>{' '}
        <span className="font-mono text-caption">{row.call}</span>{' '}
        <span className={`font-mono text-caption uppercase ${outcomeTone(row.outcome)}`}>{row.outcome}</span>{' '}
        <span className="font-mono text-caption text-on-surface-variant">v{row.version}</span>
      </p>
      <p className="text-caption text-on-surface-variant pl-md">{row.input}</p>
      <p className="font-mono text-caption text-on-surface-variant pl-md">{row.detail}</p>
      {row.expectedVersion !== undefined && row.actualVersion !== undefined && (
        <p className="font-mono text-caption text-error pl-md">
          expected version {row.expectedVersion}, actual {row.actualVersion} — nothing landed
        </p>
      )}
    </div>
  );
}

export function useArchEventstoreDemo(
  useCase: GetArchEventstoreDemoUseCase = container.resolve(ARCH_EVENTSTORE_DEMO_USE_CASE),
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

export function ArchEventstoreDemo() {
  const { status, replay } = useArchEventstoreDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="arch-eventstore-loading" className="text-body-sm text-on-surface-variant">
        driving the event store…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the event-store exhibit" failure={status.failure} onRetry={replay} />;
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid="arch-eventstore-ready">
      <section
        data-testid="arch-eventstore-store"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="text-caption text-on-surface-variant">{data.mode}</p>
        <p className="font-mono text-caption text-on-surface">
          stream {data.streamId} — the real internal/domain/patterns EventStore
        </p>
        {data.store.map((line) => (
          <p key={line} className="font-mono text-caption text-on-surface-variant pl-md">
            {line}
          </p>
        ))}
      </section>

      {data.panels.map((panel) => (
        <section
          key={panel.id}
          data-testid={`es-panel-${panel.id}`}
          className="flex flex-col gap-xs rounded-md border border-outline px-md py-sm"
        >
          <p className="text-body-sm font-medium text-on-surface">{panel.title}</p>
          <p className="text-caption text-on-surface-variant">{panel.lead}</p>
          {panel.config.map((line) => (
            <p key={line} className="text-caption text-on-surface-variant pl-md">
              {line}
            </p>
          ))}
          {panel.rows.map((row) => (
            <RunLine key={row.step} row={row} />
          ))}
          {panel.counts.map((count) => (
            <p
              key={count.label}
              className="font-mono text-caption text-on-surface pl-md"
              data-testid={`es-count-${panel.id}`}
            >
              {count.value}× {count.label}
            </p>
          ))}
        </section>
      ))}

      <section
        data-testid="arch-eventstore-evidence"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <p className="text-body-sm font-medium text-on-surface">
          The rebuild evidence ({data.replay.foldedEvents} events folded, in order)
        </p>
        {data.replay.appliedInOrder.map((line) => (
          <p key={line} className="font-mono text-caption text-on-surface-variant pl-md">
            {line}
          </p>
        ))}
        <p className="font-mono text-caption text-on-surface">
          rebuilt: {formatState(data.replay.state)}
        </p>
        <p className="font-mono text-caption text-on-surface-variant pl-md">
          naive ledger (no fold, no store): {formatState(data.replay.naiveExpected)} — agrees
        </p>
        <p className="font-mono text-caption text-on-surface pl-md">
          {data.replay.rebuildsRun} rebuilds, byte-identical={String(data.replay.byteIdentical)}
        </p>
        <p className="font-mono text-caption text-on-surface-variant pl-md">
          absent stream {data.replay.absentStreamId} folds to {formatState(data.replay.absentState)} at v
          {data.replay.absentVersion}
        </p>
        <p className="text-caption text-on-surface-variant pl-md">{data.replay.note}</p>
      </section>

      <p data-testid="arch-eventstore-honesty" className="text-body-sm text-on-surface-variant">
        {data.honestyNote}
      </p>

      <button
        type="button"
        onClick={replay}
        data-testid="arch-eventstore-replay"
        className="self-start rounded-md border border-outline px-md py-xxs text-caption text-on-surface hover:bg-surface-tint"
      >
        replay the exhibit
      </button>
    </div>
  );
}

export default ArchEventstoreDemo;
