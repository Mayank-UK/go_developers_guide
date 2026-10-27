import { useEffect, useRef, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import type { GetRealtimeStoryDemoUseCase } from '../../../domain/demo/get-realtime-story-demo-usecase';
import { REALTIME_TICKER_DEMO_USE_CASE } from '../../../domain/demo/get-realtime-story-demo-usecase';
import type { RealtimeStoryDemoData, RealtimeLiveMessage } from '../../../domain/demo/realtime-story';
import { REALTIME_LIVE_PATHS } from '../../../domain/demo/realtime-story';

/**
 * TickerDemo — the realtime topic's `realtime:ticker` demo in two
 * halves, like the server serves it: the STORY (one request, six beats
 * whose facts carry bytes DERIVED by the same formatter the live stream
 * calls) and the LIVE STREAM beside it — a browser-native EventSource
 * this panel connects on request, showing each frame's id advance and
 * the panel's own reconnect behavior (the browser owns the retry;
 * onerror during a reconnect is normal, and the stream resumes by
 * Last-Event-ID on its own). Verdict colors: ok → primary, rejected →
 * error, info → muted.
 */

type StoryStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: RealtimeStoryDemoData }
  | { state: 'error'; failure: Failure };

function verdictClass(verdict: string): string {
  if (verdict === 'ok') return 'text-primary';
  if (verdict === 'rejected') return 'text-error';
  return 'text-on-surface-variant';
}

function useRealtimeStory(useCase: GetRealtimeStoryDemoUseCase): {
  status: StoryStatus;
  retry: () => void;
} {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<StoryStatus>({ state: 'idle' });

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

/** The live panel's connection lifecycle — EventSource's own states. */
type LiveStatus = 'idle' | 'connecting' | 'open' | 'reconnecting' | 'closed' | 'unsupported';

/** LiveTickerPanel connects this page to the guide's real event stream. */
function LiveTickerPanel() {
  const [status, setStatus] = useState<LiveStatus>('idle');
  const [ticks, setTicks] = useState<RealtimeLiveMessage[]>([]);
  const [lastEventId, setLastEventId] = useState('');
  const sourceRef = useRef<EventSource | null>(null);

  // Unmount always hangs up: no stream outlives its panel.
  useEffect(() => () => sourceRef.current?.close(), []);

  const hangUp = () => {
    sourceRef.current?.close();
    sourceRef.current = null;
    setStatus('idle');
  };

  const connect = () => {
    if (typeof EventSource === 'undefined') {
      setStatus('unsupported');
      return;
    }
    setStatus('connecting');
    const source = new EventSource(REALTIME_LIVE_PATHS.tickerStream);
    sourceRef.current = source;
    source.onopen = () => setStatus('open');
    // The stream names every event ("event: tick"), so the DEFAULT
    // onmessage handler never fires — named events subscribe by name.
    source.addEventListener('tick', (event) => {
      const messageEvent = event as MessageEvent<string>;
      setLastEventId(messageEvent.lastEventId);
      try {
        const msg = JSON.parse(messageEvent.data) as RealtimeLiveMessage;
        setTicks((current) => [...current.slice(-5), msg]);
      } catch {
        // Not the room's protocol: ignored defensively, never rendered.
      }
      setStatus('open');
    });
    source.onerror = () => {
      // readyState CLOSED means the browser gave up; anything else it is
      // ALREADY reconnecting — with Last-Event-ID — by itself.
      setStatus(source.readyState === EventSource.CLOSED ? 'closed' : 'reconnecting');
    };
  };

  const live = (
    <section
      data-testid="realtime-ticker-live"
      className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
    >
      <p className="font-mono text-caption text-primary">
        the live stream · <span data-testid="realtime-ticker-live-status">{status}</span>
      </p>
      <p className="font-mono text-caption text-on-surface-variant">
        endpoint <span className="text-on-surface">{REALTIME_LIVE_PATHS.tickerStream}</span> — one
        tick per second; the browser reconnects and resumes on its own
      </p>
      {lastEventId !== '' && (
        <p className="font-mono text-caption text-on-surface-variant" data-testid="realtime-ticker-live-last-id">
          last event id <span className="text-secondary">{lastEventId}</span> — the resume key the
          browser resends as Last-Event-ID
        </p>
      )}
      {ticks.length > 0 && (
        <div data-testid="realtime-ticker-live-frames" className="flex flex-col gap-xxs">
          {ticks.map((tick) => (
            <p key={tick.seq} className="font-mono text-caption text-on-surface">
              id: {tick.seq} · event: {tick.kind} · data: {tick.body}
            </p>
          ))}
        </div>
      )}
      {status === 'unsupported' ? (
        <p className="text-caption text-error">this browser exposes no EventSource</p>
      ) : status === 'idle' || status === 'closed' ? (
        <button type="button" onClick={connect} className="self-start text-caption text-primary underline">
          {status === 'closed' ? 'reconnect the stream' : 'connect the live stream'}
        </button>
      ) : (
        <button type="button" onClick={hangUp} className="self-start text-caption text-primary underline">
          hang up (Ctrl-C)
        </button>
      )}
    </section>
  );

  return live;
}

export function TickerDemo() {
  // Resolved per RENDER (the family's hook-default pattern): a retry
  // re-resolves the token, so a spec — or the container — can swap the
  // registration between renders and the demo picks it up.
  const useCase = container.resolve<GetRealtimeStoryDemoUseCase>(REALTIME_TICKER_DEMO_USE_CASE);
  const { status, retry } = useRealtimeStory(useCase);

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="realtime-ticker-loading" className="text-body-sm text-on-surface-variant">
        deriving the frame anatomy and the resume math off the real feed…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the realtime ticker demo" failure={status.failure} onRetry={retry} />;
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid="realtime-ticker-ready">
      <LiveTickerPanel />
      <p className="font-mono text-caption text-on-surface-variant">
        mechanism <span className="text-secondary">{data.mechanism}</span> — one request, six beats
        derived from the same formatter the live stream above is writing right now
      </p>

      {data.beats.map((beat) => (
        <section
          key={beat.beat}
          data-testid={`realtime-ticker-beat-${slug(beat.beat)}`}
          className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
        >
          <p className={`font-mono text-caption ${verdictClass(beat.verdict)}`}>
            <span className="text-secondary">{beat.beat}</span> · {beat.verdict}
          </p>
          <p className="text-caption text-on-surface">{beat.detail}</p>
          {beat.facts.map((fact) => (
            <p key={fact.label} className="font-mono text-caption text-on-surface-variant">
              {fact.label}: <span className="text-on-surface">{fact.value}</span>
            </p>
          ))}
        </section>
      ))}

      <section
        data-testid="realtime-ticker-proofs"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-primary">the derived proofs</p>
        {data.proofs.map((proof) => (
          <p key={proof.label} className="text-caption text-on-surface">
            <span className="font-mono text-secondary">{proof.label}</span> — {proof.value}
          </p>
        ))}
      </section>

      <p data-testid="realtime-ticker-reasoning" className="text-body-sm text-on-surface-variant">
        {data.reasoning}
      </p>
    </div>
  );
}

/** slug turns a beat label into a stable testid fragment. */
function slug(label: string): string {
  return label
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

export default TickerDemo;
