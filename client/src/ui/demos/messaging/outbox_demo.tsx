import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetMessagingOutboxDemoUseCase,
  MESSAGING_OUTBOX_DEMO_USE_CASE,
} from '../../../domain/demo/get-messaging-outbox-demo-usecase';
import type {
  MessagingOutboxBeat,
  MessagingOutboxDemoData,
} from '../../../domain/demo/messaging-outbox';

/**
 * MessagingOutboxDemo — the messaging topic's second live demo (demo
 * block id `msg:outbox`). It renders the payload of GET
 * /api/demo/msg/outbox: one scripted walk of the transactional outbox
 * across BOTH live stores — event rows and outbox rows committed in one
 * PostgreSQL transaction each, a third pair deliberately rolled back to
 * prove all-or-nothing, the relay draining committed rows onto the
 * stream (and a second pass draining nothing), a consumer processing
 * each message exactly once by deduplicating on the message id, and a
 * forced redelivery that the dedup seam refuses.
 *
 * Every load mints a fresh throwaway namespace server-side; an
 * unreachable PostgreSQL or Redis degrades to the typed error panel
 * (the server's honest 503), never to a stale answer.
 */

type OutboxStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: MessagingOutboxDemoData }
  | { state: 'error'; failure: Failure };

export function useMessagingOutboxDemo(
  useCase: GetMessagingOutboxDemoUseCase = container.resolve(MESSAGING_OUTBOX_DEMO_USE_CASE),
): { status: OutboxStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<OutboxStatus>({ state: 'idle' });

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

export function MessagingOutboxDemo() {
  const { status, retry } = useMessagingOutboxDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="msg-outbox-loading" className="text-body-sm text-on-surface-variant">
        walking the outbox pattern against the live stores…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the messaging outbox demo" failure={status.failure} onRetry={retry} />;
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid="msg-outbox-ready">
      <p className="font-mono text-caption text-on-surface-variant">
        outbox stream <span className="text-secondary">{data.stream}</span> — PostgreSQL commits the
        unit, Redis carries the fan-out
      </p>

      {data.beats.map((beat) => (
        <OutboxBeat key={beat.beat} beat={beat} />
      ))}

      <section
        data-testid="msg-outbox-proofs"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-primary">the three derived proofs</p>
        <p className="text-caption text-on-surface">{data.proofs.atomicity}</p>
        <p className="text-caption text-on-surface">{data.proofs.relayProof}</p>
        <p className="text-caption text-on-surface">{data.proofs.idempotency}</p>
        <p data-testid="msg-outbox-cleanup" className="font-mono text-caption text-on-surface">
          duplicates refused{' '}
          <span className={data.proofs.duplicates >= 1 ? 'text-primary' : 'text-error'}>
            {String(data.proofs.duplicates)}
          </span>{' '}
          · rows deleted {data.proofs.eventsDeleted}/{data.proofs.outboxDeleted} — both stores as
          the run found them
        </p>
      </section>

      <p data-testid="msg-outbox-reasoning" className="text-body-sm text-on-surface-variant">
        {data.reasoning}
      </p>
    </div>
  );
}

function OutboxBeat({ beat }: { beat: MessagingOutboxBeat }) {
  return (
    <section
      data-testid={`msg-outbox-beat-${slug(beat.beat)}`}
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
      {beat.ids.map((id) => (
        <p key={id} className="font-mono text-caption text-on-surface-variant">
          id {id}
        </p>
      ))}
      <p className="font-mono text-caption text-on-surface">{beat.outcome}</p>
    </section>
  );
}

/** slug turns a beat label into a stable testid fragment. */
function slug(label: string): string {
  return label
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

export default MessagingOutboxDemo;
