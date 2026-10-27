import type { RealtimeStoryDemoData } from '../../../domain/demo/realtime-story';

/**
 * The shared fixture for both realtime demo specs — shaped exactly like
 * the server's payloads, with each topic's own distinctive beats so the
 * specs pin the demos' teaching content (the byte-exact tick frame, the
 * resume arithmetic, the eviction with close 1013) rather than generic
 * story chrome.
 */

/** A ticker-shaped story: the SSE beats, honest to the wire shape. */
export function realtimeTickerFixture(
  overrides: Partial<RealtimeStoryDemoData> = {},
): RealtimeStoryDemoData {
  return {
    mechanism: 'sse-ticker',
    beats: [
      {
        beat: 'the frame anatomy',
        detail:
          'an SSE event is field lines ended by ONE blank line — the blank line is the delivery signal, so an unfinished frame is not an event yet.',
        verdict: 'ok',
        facts: [
          {
            label: 'a real tick frame',
            value: 'id: 41\nevent: tick\ndata: {"seq":41,"kind":"tick","from":"feed","body":"tick","at":"2026-10-27T09:30:00Z"}\n\n',
          },
        ],
      },
      {
        beat: 'the response contract',
        detail: 'the content type says event stream, no-cache tells intermediaries never to serve a stale copy.',
        verdict: 'ok',
        facts: [
          { label: 'Content-Type', value: 'text/event-stream; charset=utf-8' },
          { label: 'reconnection advice', value: 'retry: 3000' },
        ],
      },
      {
        beat: 'the live stream',
        detail: 'the stream itself runs beside this story — one shared feed, one tick per second.',
        verdict: 'info',
        facts: [{ label: 'endpoint', value: 'GET /api/demo/realtime/ticker/stream' }],
      },
      {
        beat: 'Last-Event-ID resume',
        detail: 'the server replays every retained tick strictly newer than that id, then continues live.',
        verdict: 'ok',
        facts: [
          { label: 'Last-Event-ID: 38', value: '3 frames replayed — seqs 39, 40, 41 — then live' },
        ],
      },
      {
        beat: 'keepalive',
        detail: 'comment frames keep the path warm while delivering nothing to the page.',
        verdict: 'ok',
        facts: [{ label: 'the keepalive bytes', value: ': keepalive' }],
      },
      {
        beat: 'at-least-once honesty',
        detail: 'a frame may be redelivered after a reconnect; dedupe on seq makes it harmless.',
        verdict: 'info',
        facts: [
          { label: 'per id', value: 'at-most-once — drop anything ≤ the highest seq you have handled' },
        ],
      },
    ],
    proofs: [
      { label: 'the wire format', value: 'field lines (id, event, data) + one blank line' },
      { label: 'one-way only', value: 'chat and client pushes need WebSockets' },
    ],
    reasoning:
      'the server flushes, numbers, retains, and keeps the line warm — the browser owns the retry.',
    ...overrides,
  };
}

/** A chat-shaped story: the WS beats, honest to the wire shape. */
export function realtimeChatFixture(
  overrides: Partial<RealtimeStoryDemoData> = {},
): RealtimeStoryDemoData {
  return {
    mechanism: 'ws-chat',
    beats: [
      {
        beat: 'the handshake',
        detail: 'one ordinary HTTP request the server promotes with 101 Switching Protocols.',
        verdict: 'ok',
        facts: [{ label: 'origin policy', value: 'the request host is always authorized; cross-origin must match a pattern' }],
      },
      {
        beat: 'rooms and broadcast',
        detail: 'one Publish stamps a per-room sequence number and hands the message to EVERY member.',
        verdict: 'ok',
        facts: [{ label: 'the choreography', value: 'publish #1 (seq 1) → all 3 members served instantly' }],
      },
      {
        beat: 'the ceiling',
        detail: 'two goroutines and one bounded buffer per connection; non-blocking sends per broadcast.',
        verdict: 'ok',
        facts: [{ label: 'per subscriber', value: 'a 16-message buffer — the memory ceiling per member' }],
      },
      {
        beat: 'the slow consumer',
        detail: 'the stalled subscriber stopped reading; the second overflow drop reached its budget.',
        verdict: 'rejected',
        facts: [
          { label: 'publish #4', value: 'seq 4: the drop budget hit — evicted: [stalled]' },
          { label: 'the eviction close', value: 'code 1013 — the subscriber stopped reading' },
        ],
      },
      {
        beat: 'ping/pong and the close codes',
        detail: 'every way this stack ends a connection has its own named code.',
        verdict: 'info',
        facts: [{ label: '1003', value: 'a binary frame arrived — JSON text frames only' }],
      },
      {
        beat: 'this very room',
        detail: 'the live room runs beside this story; join as a guest and say something.',
        verdict: 'info',
        facts: [{ label: 'endpoint', value: 'GET /api/demo/realtime/chat/ws (Upgrade: websocket)' }],
      },
    ],
    proofs: [
      { label: 'ordering', value: 'per-room seq, strictly monotonic — render in seq order' },
      { label: 'eviction math', value: 'live policy: buffer 16 + budget 8 drops → close 1013' },
    ],
    reasoning: 'bounded fan-out, explicit slow-consumer policy, honest teardown.',
    ...overrides,
  };
}
