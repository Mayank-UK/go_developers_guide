import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { TickerDemo } from './ticker_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetRealtimeStoryDemoUseCase,
  REALTIME_TICKER_DEMO_USE_CASE,
} from '../../../domain/demo/get-realtime-story-demo-usecase';
import type { RealtimeStoryDemoData } from '../../../domain/demo/realtime-story';
import { realtimeTickerFixture } from './realtime_story_fixtures';

/**
 * TickerDemo spec — the story half is fixture-driven (happy + failure,
 * the family pattern: the component resolves its use case through the
 * container token, so the spec swaps that registration instead of
 * stubbing fetch). The LIVE half drives a fake EventSource through the
 * panel's whole lifecycle: connect, named 'tick' events (the default
 * onmessage never fires — the stream names its events), the browser's
 * own reconnect state, and hang-up.
 */

const fixture: RealtimeStoryDemoData = realtimeTickerFixture();

function stubUseCase(execute: () => Promise<Result<RealtimeStoryDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    REALTIME_TICKER_DEMO_USE_CASE,
    { execute } as unknown as GetRealtimeStoryDemoUseCase,
  );
}

/** FakeEventSource implements exactly the EventSource surface the panel uses. */
class FakeEventSource {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSED = 2;
  static instances: FakeEventSource[] = [];
  readyState = FakeEventSource.CONNECTING;
  url: string;
  onopen: ((ev: Event) => void) | null = null;
  onerror: ((ev: Event) => void) | null = null;
  closed = false;
  private readonly listeners = new Map<string, (ev: { lastEventId: string; data: string }) => void>();

  constructor(url: string) {
    this.url = url;
    FakeEventSource.instances.push(this);
  }

  addEventListener(type: string, listener: (ev: { lastEventId: string; data: string }) => void): void {
    this.listeners.set(type, listener);
  }

  close(): void {
    this.closed = true;
    this.readyState = FakeEventSource.CLOSED;
  }

  // ---- spec drivers ----
  serverOpen(): void {
    this.readyState = FakeEventSource.OPEN;
    this.onopen?.(new Event('open'));
  }

  serverTick(seq: number): void {
    this.listeners.get('tick')?.({
      lastEventId: String(seq),
      data: JSON.stringify({ seq, kind: 'tick', from: 'feed', body: 'tick', at: '2026-10-27T10:00:00Z' }),
    });
  }

  serverDrop(): void {
    this.onerror?.(new Event('error'));
  }
}

afterEach(() => {
  vi.unstubAllGlobals();
  FakeEventSource.instances = [];
});

describe('TickerDemo', () => {
  it('renders loading first, then the story with its six beats and proofs', async () => {
    stubUseCase(async () => success(fixture));

    render(<TickerDemo />);

    expect(screen.getByTestId('realtime-ticker-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('realtime-ticker-ready')).toBeInTheDocument());
    expect(screen.getByTestId('realtime-ticker-beat-the-frame-anatomy')).toHaveTextContent(
      'id: 41',
    );
    expect(screen.getByTestId('realtime-ticker-beat-last-event-id-resume')).toHaveTextContent(
      '39, 40, 41',
    );
    expect(screen.getByTestId('realtime-ticker-beat-at-least-once-honesty')).toHaveTextContent('dedupe');
    expect(screen.getByTestId('realtime-ticker-proofs')).toHaveTextContent('the wire format');
    expect(screen.getByTestId('realtime-ticker-reasoning')).toBeInTheDocument();
    expect(screen.getByTestId('realtime-ticker-live')).toHaveTextContent('idle');
  });

  it('degrades to the error panel on failure, with recovery', async () => {
    const failure = new ServerFailure('The realtime ticker demo could not run its story.', {
      code: 'service_unavailable',
      status: 503,
      requestId: 'rid-rt-503',
    });
    stubUseCase(async () => failed(failure));

    render(<TickerDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('service_unavailable');

    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('realtime-ticker-ready')).toBeInTheDocument());
  });

  it('lives: named tick events render, the reconnect is the browser\u2019s, hang-up closes', async () => {
    stubUseCase(async () => success(fixture));
    vi.stubGlobal('EventSource', FakeEventSource as unknown as typeof EventSource);

    render(<TickerDemo />);
    await waitFor(() => expect(screen.getByTestId('realtime-ticker-ready')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /connect the live stream/ }));
    const source = FakeEventSource.instances[0];
    if (source === undefined) throw new Error('the panel never constructed its EventSource');
    expect(source.url).toBe('/api/demo/realtime/ticker/stream');
    expect(screen.getByTestId('realtime-ticker-live-status')).toHaveTextContent('connecting');

    act(() => source.serverOpen());
    expect(screen.getByTestId('realtime-ticker-live-status')).toHaveTextContent('open');

    act(() => source.serverTick(41));
    act(() => source.serverTick(42));
    const frames = screen.getByTestId('realtime-ticker-live-frames');
    expect(frames).toHaveTextContent('id: 41');
    expect(frames).toHaveTextContent('id: 42');
    expect(screen.getByTestId('realtime-ticker-live-last-id')).toHaveTextContent('42');

    // A dropped connection while the browser retries: reconnecting, and
    // the LAST event id stays on screen — it is the resume key.
    act(() => source.serverDrop());
    expect(screen.getByTestId('realtime-ticker-live-status')).toHaveTextContent('reconnecting');

    fireEvent.click(screen.getByRole('button', { name: /hang up/ }));
    expect(source.closed).toBe(true);
    expect(screen.getByTestId('realtime-ticker-live-status')).toHaveTextContent('idle');
  });
});
