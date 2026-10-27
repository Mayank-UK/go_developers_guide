import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { ChatDemo } from './chat_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetRealtimeStoryDemoUseCase,
  REALTIME_CHAT_DEMO_USE_CASE,
} from '../../../domain/demo/get-realtime-story-demo-usecase';
import type { RealtimeStoryDemoData } from '../../../domain/demo/realtime-story';
import { realtimeChatFixture } from './realtime_story_fixtures';

/**
 * ChatDemo spec — the story half is fixture-driven (happy + failure,
 * the family pattern). The LIVE half drives a fake WebSocket through
 * the room's whole lifecycle: join, the join notice naming THIS
 * connection's guest id, a spoken line sent in the room's JSON
 * protocol, the echo arriving in the transcript, and a named close
 * (code 1003) rendering legibly with the manual rejoin after it —
 * WS reconnection is client-owned by design, and the panel says so.
 */

const fixture: RealtimeStoryDemoData = realtimeChatFixture();

function stubUseCase(execute: () => Promise<Result<RealtimeStoryDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    REALTIME_CHAT_DEMO_USE_CASE,
    { execute } as unknown as GetRealtimeStoryDemoUseCase,
  );
}

/** FakeWebSocket implements exactly the WebSocket surface the panel uses. */
class FakeWebSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  static instances: FakeWebSocket[] = [];
  readyState = FakeWebSocket.CONNECTING;
  url: string;
  sent: string[] = [];
  onopen: ((ev: Event) => void) | null = null;
  onmessage: ((ev: { data: unknown }) => void) | null = null;
  onclose: ((ev: { code: number; reason: string }) => void) | null = null;
  onerror: ((ev: Event) => void) | null = null;

  constructor(url: string) {
    this.url = url;
    FakeWebSocket.instances.push(this);
  }

  send(payload: string): void {
    this.sent.push(payload);
  }

  close(): void {
    this.readyState = FakeWebSocket.CLOSED;
  }

  // ---- spec drivers (the room, seen from the server) ----
  serverOpen(): void {
    this.readyState = FakeWebSocket.OPEN;
    this.onopen?.(new Event('open'));
  }

  serverMessage(msg: unknown): void {
    this.onmessage?.({ data: JSON.stringify(msg) });
  }

  serverClose(code: number, reason: string): void {
    this.readyState = FakeWebSocket.CLOSED;
    this.onclose?.({ code, reason });
  }
}

afterEach(() => {
  vi.unstubAllGlobals();
  FakeWebSocket.instances = [];
});

describe('ChatDemo', () => {
  it('renders loading first, then the story with its six beats and proofs', async () => {
    stubUseCase(async () => success(fixture));

    render(<ChatDemo />);

    expect(screen.getByTestId('realtime-chat-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('realtime-chat-ready')).toBeInTheDocument());
    expect(screen.getByTestId('realtime-chat-beat-the-handshake')).toHaveTextContent('101');
    expect(screen.getByTestId('realtime-chat-beat-the-slow-consumer')).toHaveTextContent(
      'evicted: [stalled]',
    );
    expect(screen.getByTestId('realtime-chat-beat-ping-pong-and-the-close-codes')).toHaveTextContent('1003');
    expect(screen.getByTestId('realtime-chat-proofs')).toHaveTextContent('eviction math');
    expect(screen.getByTestId('realtime-chat-reasoning')).toBeInTheDocument();
    expect(screen.getByTestId('realtime-chat-live')).toHaveTextContent('idle');
  });

  it('degrades to the error panel on failure, with recovery', async () => {
    const failure = new ServerFailure('The realtime chat demo could not run its story.', {
      code: 'service_unavailable',
      status: 503,
      requestId: 'rid-rc-503',
    });
    stubUseCase(async () => failed(failure));

    render(<ChatDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('service_unavailable');

    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('realtime-chat-ready')).toBeInTheDocument());
  });

  it('lives: join, speak in the room\u2019s protocol, hear the echo, read the named close', async () => {
    stubUseCase(async () => success(fixture));
    vi.stubGlobal('WebSocket', FakeWebSocket as unknown as typeof WebSocket);

    render(<ChatDemo />);
    await waitFor(() => expect(screen.getByTestId('realtime-chat-ready')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /join the room/ }));
    const socket = FakeWebSocket.instances[0];
    if (socket === undefined) throw new Error('the panel never constructed its WebSocket');
    expect(socket.url).toContain('/api/demo/realtime/chat/ws');
    expect(screen.getByTestId('realtime-chat-live-status')).toHaveTextContent('connecting');

    act(() => socket.serverOpen());
    expect(screen.getByTestId('realtime-chat-live-status')).toHaveTextContent('open');

    // The room's own join notice names this connection's guest id.
    act(() =>
      socket.serverMessage({ seq: 1, kind: 'system', from: 'hub', body: 'guest-3 joined the room — 1 online' }),
    );
    expect(screen.getByTestId('realtime-chat-live')).toHaveTextContent('you are');
    expect(screen.getByTestId('realtime-chat-live')).toHaveTextContent('guest-3');

    // Speak: one JSON text frame in the room's protocol.
    fireEvent.change(screen.getByTestId('realtime-chat-live-input'), {
      target: { value: 'hello room' },
    });
    fireEvent.click(screen.getByTestId('realtime-chat-live-send'));
    expect(socket.sent).toEqual(['{"type":"chat","body":"hello room"}']);

    // The echo is the confirmation the room served the line.
    act(() =>
      socket.serverMessage({ seq: 2, kind: 'chat', from: 'guest-3', body: 'hello room' }),
    );
    expect(screen.getByTestId('realtime-chat-live-transcript')).toHaveTextContent('hello room');

    // A binary frame ends the connection with the named close code —
    // legible on screen, with the manual rejoin after it.
    act(() => socket.serverClose(1003, 'a binary frame arrived — this chat speaks JSON text frames only'));
    expect(screen.getByTestId('realtime-chat-live-close')).toHaveTextContent('close 1003');
    expect(screen.getByTestId('realtime-chat-live-status')).toHaveTextContent('closed');
    expect(screen.getByRole('button', { name: /rejoin the room/ })).toBeInTheDocument();
  });
});
