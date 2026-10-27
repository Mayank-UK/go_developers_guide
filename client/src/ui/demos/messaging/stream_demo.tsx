import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetMessagingStreamDemoUseCase,
  MESSAGING_STREAM_DEMO_USE_CASE,
} from '../../../domain/demo/get-messaging-stream-demo-usecase';
import type {
  MessagingStreamBeat,
  MessagingStreamDemoData,
} from '../../../domain/demo/messaging-stream';

/**
 * MessagingStreamDemo — the messaging topic's first live demo (demo
 * block id `msg:stream`). It renders the payload of GET
 * /api/demo/msg/stream: one scripted walk of the consumer-group
 * lifecycle against the RUNNING stack Redis — three events appended in
 * order, the group created at 0-0, consumer-a reading everything in
 * insertion order, consumer-b reading nothing (the queue property),
 * consumer-a crashing with one entry un-acknowledged, XPENDING naming
 * the stalled entry, XAUTOCLAIM moving it to consumer-b with its values
 * intact, and the pending ledger drained to zero.
 *
 * Every load mints a fresh throwaway namespace server-side, so the
 * Retry button walks a genuinely new stream; an unreachable Redis
 * degrades to the typed error panel (the server's honest 503), never to
 * a stale answer.
 */

type StreamStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: MessagingStreamDemoData }
  | { state: 'error'; failure: Failure };

export function useMessagingStreamDemo(
  useCase: GetMessagingStreamDemoUseCase = container.resolve(MESSAGING_STREAM_DEMO_USE_CASE),
): { status: StreamStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<StreamStatus>({ state: 'idle' });

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

export function MessagingStreamDemo() {
  const { status, retry } = useMessagingStreamDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="msg-stream-loading" className="text-body-sm text-on-surface-variant">
        walking the consumer-group lifecycle against the live store…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the messaging stream demo" failure={status.failure} onRetry={retry} />;
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid="msg-stream-ready">
      <p className="font-mono text-caption text-on-surface-variant">
        stream <span className="text-secondary">{data.stream}</span> · group{' '}
        <span className="text-secondary">{data.group}</span> — throwaway per run, removed on the way
        out
      </p>

      {data.beats.map((beat) => (
        <StreamBeat key={beat.beat} beat={beat} />
      ))}

      <section
        data-testid="msg-stream-proofs"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-primary">the three derived proofs</p>
        <p className="text-caption text-on-surface">{data.proofs.ordering}</p>
        <p className="text-caption text-on-surface">{data.proofs.queueProof}</p>
        <p className="text-caption text-on-surface">{data.proofs.recovery}</p>
        <p data-testid="msg-stream-pending-left" className="font-mono text-caption text-on-surface">
          pending left after recovery:{' '}
          <span className={data.proofs.pendingLeft === 0 ? 'text-primary' : 'text-error'}>
            {String(data.proofs.pendingLeft)}
          </span>
        </p>
      </section>

      <p data-testid="msg-stream-reasoning" className="text-body-sm text-on-surface-variant">
        {data.reasoning}
      </p>
    </div>
  );
}

function StreamBeat({ beat }: { beat: MessagingStreamBeat }) {
  return (
    <section
      data-testid={`msg-stream-beat-${slug(beat.beat)}`}
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
      {beat.entries.map((entry) => (
        <p key={entry} className="font-mono text-caption text-on-surface-variant">
          entry {entry}
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

export default MessagingStreamDemo;
