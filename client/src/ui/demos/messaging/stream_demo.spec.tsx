import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MessagingStreamDemo } from './stream_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetMessagingStreamDemoUseCase,
  MESSAGING_STREAM_DEMO_USE_CASE,
} from '../../../domain/demo/get-messaging-stream-demo-usecase';
import type { MessagingStreamDemoData } from '../../../domain/demo/messaging-stream';

/**
 * MessagingStreamDemo spec — fixture-driven (happy + failure): the
 * component resolves its use case through the container token (the
 * documented register-a-singleton seam), so the spec swaps that
 * registration instead of stubbing fetch. Behavior, not implementation:
 * the beats render with their commands and outcomes, the three derived
 * proofs render with the pending-ledger count, and failures — including
 * the server's honest 503 for an unreachable Redis — degrade to the
 * shared error panel with recovery.
 */

const fixture: MessagingStreamDemoData = {
  namespace: 'gdg:demo:msg:0123456789ab',
  stream: 'gdg:demo:msg:0123456789ab:orders',
  group: 'workers',
  commands: ['XADD', 'XGROUP CREATE', 'XREADGROUP', 'XACK', 'XPENDING', 'XAUTOCLAIM'],
  beats: [
    {
      beat: 'append',
      commands: [
        'XADD gdg:demo:msg:0123456789ab:orders … step created',
        'XADD gdg:demo:msg:0123456789ab:orders … step paid',
        'XADD gdg:demo:msg:0123456789ab:orders … step shipped',
      ],
      outcome: 'three entries appended; ids assigned by the store: 1780000000000-0, …',
      entries: ['1780000000000-0', '1780000000001-0', '1780000000002-0'],
    },
    {
      beat: 'consume (b)',
      commands: ['XREADGROUP GROUP workers consumer-b COUNT 10 >'],
      outcome: 'consumer-b received 0 new entries — the queue had nothing left for it.',
      entries: [],
    },
    {
      beat: 'autoclaim',
      commands: ['XAUTOCLAIM gdg:demo:msg:0123456789ab:orders workers consumer-b 0 0-0 COUNT 10'],
      outcome: 'consumer-b claimed 1 stalled entry with the values intact.',
      entries: ['1780000000001-0'],
    },
  ],
  proofs: {
    ordering: '3 entries appended in order (created → paid → shipped); consumer-a answered the same ids in the same order.',
    queueProof: 'consumer-b answered 0 entries: one group over a stream is a work QUEUE.',
    recovery: 'XPENDING named the stalled entry; XAUTOCLAIM moved exactly it; the ledger drained to 0.',
    pendingLeft: 0,
  },
  reasoning: 'crash recovery is a command, not a mystery.',
};

function stubUseCase(execute: () => Promise<Result<MessagingStreamDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    MESSAGING_STREAM_DEMO_USE_CASE,
    { execute } as unknown as GetMessagingStreamDemoUseCase,
  );
}

describe('MessagingStreamDemo', () => {
  it('renders loading first, then the whole story', async () => {
    stubUseCase(async () => success(fixture));

    render(<MessagingStreamDemo />);

    expect(screen.getByTestId('msg-stream-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('msg-stream-ready')).toBeInTheDocument());

    // The beats, each with its commands, entries, and outcome.
    const append = screen.getByTestId('msg-stream-beat-append');
    expect(append).toHaveTextContent('step shipped');
    expect(append).toHaveTextContent('1780000000002-0');

    expect(screen.getByTestId('msg-stream-beat-consume-b')).toHaveTextContent(
      'received 0 new entries',
    );
    expect(screen.getByTestId('msg-stream-beat-autoclaim')).toHaveTextContent('claimed 1 stalled');

    // The three derived proofs and the namespace banner.
    expect(screen.getByTestId('msg-stream-proofs')).toHaveTextContent('same ids in the same order');
    expect(screen.getByTestId('msg-stream-proofs')).toHaveTextContent('work QUEUE');
    expect(screen.getByTestId('msg-stream-pending-left')).toHaveTextContent('0');
    expect(screen.getByTestId('msg-stream-reasoning')).toHaveTextContent('a command, not a mystery');
  });

  it('degrades to the error panel on the server\u2019s honest 503, with recovery', async () => {
    const failure = new ServerFailure(
      'The messaging stream demo runs real commands against the guide\u2019s Redis, which is not reachable right now.',
      { code: 'service_unavailable', status: 503, requestId: 'rid-msg-503' },
    );
    stubUseCase(async () => failed(failure));

    render(<MessagingStreamDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('service_unavailable');
    expect(panel).toHaveTextContent('not reachable right now');

    // Retry re-runs the load through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('msg-stream-ready')).toBeInTheDocument());
  });
});
