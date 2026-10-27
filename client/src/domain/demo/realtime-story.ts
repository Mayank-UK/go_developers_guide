/**
 * Domain models for the realtime topic's TWO demo stories (demo block
 * ids `realtime:ticker` and `realtime:chat`) — one shared shape, because
 * the server side deliberately serves both as the same wire structure:
 * a mechanism name, beats in execution order (each with a teaching
 * sentence, a verdict color, and label/value facts whose values carry
 * DERIVED bytes and counts — rendered frames, resume math, close-code
 * tables), derived proofs, and a closing reasoning paragraph. Field
 * names match the server's camelCase wire contract 1:1 (the DTO is the
 * seam where wire and domain may diverge later).
 */

/** One label/value pair — frame bytes, header values, close-code meanings. */
export interface RealtimeStoryFact {
  label: string;
  value: string;
}

/** One beat of a story: what was done, the sentence, the verdict color. */
export interface RealtimeStoryBeat {
  beat: string;
  detail: string;
  /** 'ok' | 'rejected' | 'info' — the client colors, never interprets. */
  verdict: string;
  facts: RealtimeStoryFact[];
}

/** The whole story payload both realtime demo endpoints serve. */
export interface RealtimeStoryDemoData {
  mechanism: string;
  beats: RealtimeStoryBeat[];
  proofs: RealtimeStoryFact[];
  reasoning: string;
}

/** One broadcast message off either live transport (SSE tick or WS chat). */
export interface RealtimeLiveMessage {
  seq: number;
  kind: string;
  from: string;
  body: string;
}

/**
 * The LIVE transport endpoints the demo panels connect to directly —
 * browser-native EventSource/WebSocket inside the components (no extra
 * client dependencies), relative to the app origin: the dev proxy
 * forwards /api, WebSocket upgrades included.
 */
export const REALTIME_LIVE_PATHS = {
  tickerStream: '/api/demo/realtime/ticker/stream',
  chatRoom: '/api/demo/realtime/chat/ws',
} as const;
