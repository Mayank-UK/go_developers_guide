import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MessagingOutboxDemo } from './outbox_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetMessagingOutboxDemoUseCase,
  MESSAGING_OUTBOX_DEMO_USE_CASE,
} from '../../../domain/demo/get-messaging-outbox-demo-usecase';
import type { MessagingOutboxDemoData } from '../../../domain/demo/messaging-outbox';

/**
 * MessagingOutboxDemo spec — fixture-driven (happy + failure): the
 * component resolves its use case through the container token (the
 * documented register-a-singleton seam), so the spec swaps that
 * registration instead of stubbing fetch. Behavior, not implementation:
 * the beats render with their commands and ids, the three derived
 * proofs render with the refused-duplicate and cleanup counts, and
 * failures — including the server's honest 503 when a store is
 * unreachable — degrade to the shared error panel with recovery.
 */

const fixture: MessagingOutboxDemoData = {
  namespace: 'gdg:demo:msg:0123456789ab',
  stream: 'gdg:demo:msg:0123456789ab:outbox',
  beats: [
    {
      beat: 'record (rollback)',
      commands: ['BEGIN', 'INSERT INTO msgdemo_events …', 'INSERT INTO msgdemo_outbox …', 'ROLLBACK'],
      outcome: 'the third pair rolled back: SELECT count(*) answers 0.',
      ids: [],
    },
    {
      beat: 'relay',
      commands: [
        'SELECT … WHERE published_at IS NULL FOR UPDATE SKIP LOCKED',
        'XADD gdg:demo:msg:0123456789ab:outbox messageId … payload …',
        'UPDATE msgdemo_outbox SET published_at = now()',
        'COMMIT',
      ],
      outcome: 'the relay published 2 message(s); a second pass drained 0.',
      ids: ['m-aaaaaaaaaaaaaaaa', 'm-bbbbbbbbbbbbbbbb'],
    },
    {
      beat: 'redeliver',
      commands: ['XAUTOCLAIM gdg:demo:msg:0123456789ab:outbox workers consumer-b 0 0-0 COUNT 10'],
      outcome: 'the stalled entry re-delivered to consumer-b was refused by dedup 1 time(s).',
      ids: ['1780000000000-0'],
    },
  ],
  proofs: {
    atomicity: 'a third write pair rolled back deliberately and the count query answered 0.',
    relayProof: 'the relay\u2019s first pass published 2 message(s); a second pass drained 0.',
    idempotency: 'the forced redelivery of an already-processed message was refused 1 time(s).',
    duplicates: 1,
    eventsDeleted: 2,
    outboxDeleted: 2,
  },
  reasoning: 'exactly-once delivery is a myth at this boundary; exactly-once EFFECT is one SET NX away.',
};

function stubUseCase(execute: () => Promise<Result<MessagingOutboxDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    MESSAGING_OUTBOX_DEMO_USE_CASE,
    { execute } as unknown as GetMessagingOutboxDemoUseCase,
  );
}

describe('MessagingOutboxDemo', () => {
  it('renders loading first, then the whole pattern', async () => {
    stubUseCase(async () => success(fixture));

    render(<MessagingOutboxDemo />);

    expect(screen.getByTestId('msg-outbox-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('msg-outbox-ready')).toBeInTheDocument());

    // The beats, each with its commands, ids, and outcome.
    const rollback = screen.getByTestId('msg-outbox-beat-record-rollback');
    expect(rollback).toHaveTextContent('ROLLBACK');
    expect(rollback).toHaveTextContent('answers 0');

    expect(screen.getByTestId('msg-outbox-beat-relay')).toHaveTextContent('FOR UPDATE SKIP LOCKED');
    expect(screen.getByTestId('msg-outbox-beat-relay')).toHaveTextContent('m-aaaaaaaaaaaaaaaa');
    expect(screen.getByTestId('msg-outbox-beat-redeliver')).toHaveTextContent('refused by dedup');

    // The three derived proofs and the cleanup line.
    const proofs = screen.getByTestId('msg-outbox-proofs');
    expect(proofs).toHaveTextContent('count query answered 0');
    expect(proofs).toHaveTextContent('second pass drained 0');
    expect(screen.getByTestId('msg-outbox-cleanup')).toHaveTextContent('duplicates refused 1');
    expect(screen.getByTestId('msg-outbox-reasoning')).toHaveTextContent('exactly-once EFFECT');
  });

  it('degrades to the error panel on the server\u2019s honest 503, with recovery', async () => {
    const failure = new ServerFailure(
      'The messaging outbox demo runs a real transaction against the guide\u2019s PostgreSQL and Redis, which are not both reachable right now.',
      { code: 'service_unavailable', status: 503, requestId: 'rid-outbox-503' },
    );
    stubUseCase(async () => failed(failure));

    render(<MessagingOutboxDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('service_unavailable');
    expect(panel).toHaveTextContent('not both reachable right now');

    // Retry re-runs the load through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('msg-outbox-ready')).toBeInTheDocument());
  });
});
