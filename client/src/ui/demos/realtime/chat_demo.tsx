import { useEffect, useRef, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import type { GetRealtimeStoryDemoUseCase } from '../../../domain/demo/get-realtime-story-demo-usecase';
import { REALTIME_CHAT_DEMO_USE_CASE } from '../../../domain/demo/get-realtime-story-demo-usecase';
import type { RealtimeStoryDemoData, RealtimeLiveMessage } from '../../../domain/demo/realtime-story';
import { REALTIME_LIVE_PATHS } from '../../../domain/demo/realtime-story';

/**
 * ChatDemo — the realtime topic's `realtime:chat` demo in two halves,
 * like the server serves it: the STORY (one request, six beats whose
 * counts are a REAL hub choreography the payload ran — delivered,
 * dropped, evicted) and the LIVE ROOM beside it — a browser-native
 * WebSocket this panel joins on request: speak and watch your own echo
 * (the confirmation you were served), watch system notices as guests
 * come and go, and read the named close code when the room ends your
 * connection. WS reconnection is CLIENT-owned by design — the rejoin
 * button is the honest UI for that, no silent auto-retry.
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

function useChatStory(useCase: GetRealtimeStoryDemoUseCase): {
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

/** The room's connection lifecycle — WebSocket states, closed legible. */
type RoomStatus = 'idle' | 'connecting' | 'open' | 'closed' | 'unsupported';

/** chatUrl builds the room's ws:// (or wss://) URL off the page origin. */
function chatUrl(): string {
  const scheme = window.location.protocol === 'https:' ? 'wss' : 'ws';
  return `${scheme}://${window.location.host}${REALTIME_LIVE_PATHS.chatRoom}`;
}

/** LiveChatRoom joins this page to the guide's real WebSocket room. */
function LiveChatRoom() {
  const [status, setStatus] = useState<RoomStatus>('idle');
  const [closeNote, setCloseNote] = useState('');
  const [guest, setGuest] = useState('');
  const [transcript, setTranscript] = useState<RealtimeLiveMessage[]>([]);
  const [draft, setDraft] = useState('');
  const socketRef = useRef<WebSocket | null>(null);

  // Unmount always leaves the room: no socket outlives its panel.
  useEffect(() => () => socketRef.current?.close(), []);

  const leave = () => {
    socketRef.current?.close();
    socketRef.current = null;
    setStatus('idle');
  };

  const join = () => {
    if (typeof WebSocket === 'undefined') {
      setStatus('unsupported');
      return;
    }
    setStatus('connecting');
    setTranscript([]);
    setGuest('');
    setCloseNote('');
    const socket = new WebSocket(chatUrl());
    socketRef.current = socket;
    socket.onopen = () => setStatus('open');
    socket.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data as string) as RealtimeLiveMessage;
        // The room's own join notice names THIS connection's guest id —
        // the first system message the room sends a new member.
        if (guest === '' && msg.kind === 'system') {
          const named = /guest-\d+/.exec(msg.body);
          if (named) setGuest(named[0]);
        }
        setTranscript((current) => [...current.slice(-11), msg]);
      } catch {
        // Not the room's protocol: ignored defensively, never rendered.
      }
    };
    socket.onclose = (event) => {
      // Every ending is named: the close frame carries its code.
      setStatus('closed');
      setCloseNote(`close ${event.code}${event.reason ? ` — ${event.reason}` : ''}`);
    };
  };

  const speak = () => {
    const line = draft.trim();
    if (line === '' || socketRef.current?.readyState !== WebSocket.OPEN) return;
    socketRef.current.send(JSON.stringify({ type: 'chat', body: line }));
    setDraft('');
  };

  return (
    <section
      data-testid="realtime-chat-live"
      className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
    >
      <p className="font-mono text-caption text-primary">
        the live room · <span data-testid="realtime-chat-live-status">{status}</span>
        {guest !== '' && <> · you are <span className="text-secondary">{guest}</span></>}
      </p>
      <p className="font-mono text-caption text-on-surface-variant">
        endpoint <span className="text-on-surface">{REALTIME_LIVE_PATHS.chatRoom}</span> — join,
        say something, watch your own echo arrive
      </p>
      {closeNote !== '' && (
        <p data-testid="realtime-chat-live-close" className="font-mono text-caption text-error">
          {closeNote}
        </p>
      )}
      {transcript.length > 0 && (
        <div data-testid="realtime-chat-live-transcript" className="flex flex-col gap-xxs">
          {transcript.map((msg) => (
            <p key={msg.seq} className="font-mono text-caption text-on-surface">
              <span className={msg.kind === 'system' ? 'text-on-surface-variant' : 'text-secondary'}>
                {msg.from}
              </span>
              : {msg.body}
            </p>
          ))}
        </div>
      )}
      {status === 'unsupported' ? (
        <p className="text-caption text-error">this browser exposes no WebSocket</p>
      ) : status === 'idle' || status === 'closed' ? (
        <button type="button" onClick={join} className="self-start text-caption text-primary underline">
          {status === 'closed' ? 'rejoin the room' : 'join the room'}
        </button>
      ) : (
        <div className="flex items-center gap-xs">
          <input
            data-testid="realtime-chat-live-input"
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter') speak();
            }}
            disabled={status !== 'open'}
            placeholder="say something to the room"
            className="rounded-md border border-outline bg-surface px-sm py-xxs text-caption text-on-surface"
          />
          <button
            type="button"
            onClick={speak}
            disabled={status !== 'open' || draft.trim() === ''}
            data-testid="realtime-chat-live-send"
            className="text-caption text-primary underline disabled:text-on-surface-variant disabled:no-underline"
          >
            send
          </button>
          <button type="button" onClick={leave} className="text-caption text-primary underline">
            leave
          </button>
        </div>
      )}
    </section>
  );
}

export function ChatDemo() {
  // Resolved per RENDER (the family's hook-default pattern): a retry
  // re-resolves the token, so a spec — or the container — can swap the
  // registration between renders and the demo picks it up.
  const useCase = container.resolve<GetRealtimeStoryDemoUseCase>(REALTIME_CHAT_DEMO_USE_CASE);
  const { status, retry } = useChatStory(useCase);

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="realtime-chat-loading" className="text-body-sm text-on-surface-variant">
        running the hub choreography — three members, one of whom never reads…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the realtime chat demo" failure={status.failure} onRetry={retry} />;
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid="realtime-chat-ready">
      <LiveChatRoom />
      <p className="font-mono text-caption text-on-surface-variant">
        mechanism <span className="text-secondary">{data.mechanism}</span> — one request, six beats
        whose counts are the hub&apos;s own answers to a choreography run fresh
      </p>

      {data.beats.map((beat) => (
        <section
          key={beat.beat}
          data-testid={`realtime-chat-beat-${slug(beat.beat)}`}
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
        data-testid="realtime-chat-proofs"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-primary">the derived proofs</p>
        {data.proofs.map((proof) => (
          <p key={proof.label} className="text-caption text-on-surface">
            <span className="font-mono text-secondary">{proof.label}</span> — {proof.value}
          </p>
        ))}
      </section>

      <p data-testid="realtime-chat-reasoning" className="text-body-sm text-on-surface-variant">
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

export default ChatDemo;
