import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { ScalingStatelessDemo } from './stateless_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetScalingStatelessDemoUseCase,
  SCALING_STATELESS_DEMO_USE_CASE,
} from '../../../domain/demo/get-scaling-stateless-demo-usecase';
import type { ScalingStatelessDemoData } from '../../../domain/demo/scaling-stateless';

/**
 * ScalingStatelessDemo spec — fixture-driven (happy + failure): the
 * component resolves its use case through the container token.
 * Behavior, not implementation: the config panel renders the fleet
 * shape and both tokens; the distribution renders its measured rows
 * and derived note; the session round renders every step with its
 * phase, replica, and measured observation; the counter beat and the
 * verdicts render with their measured figures; the replay button
 * drives a fresh execute (the round replays identically — that is the
 * determinism claim); and failures degrade to the shared error panel
 * with recovery.
 */

const fixture: ScalingStatelessDemoData = {
  mode: 'one fleet, two placements: three in-process replicas behind a real round-robin balancer',
  config: {
    replicas: 3,
    distributionRequests: 12,
    clientId: 'ada',
    counterIncrements: 6,
    localToken: 's-local-ada',
    sharedToken: 's-shared-ada',
    sharedStoreNote: 'an in-process analogue of the redis container this stack carries',
  },
  distribution: {
    totalServed: 12,
    rows: [
      { replicaId: 'replica-1', served: 4 },
      { replicaId: 'replica-2', served: 4 },
      { replicaId: 'replica-3', served: 4 },
    ],
    note: '12 request(s) across 3 replica(s), per-replica 4/4/4 — spread exactly even',
  },
  steps: [
    {
      step: 1,
      phase: 'local-memory',
      label: 'login: the session lands in replica-local memory',
      route: 'round-robin',
      replicaId: 'replica-1',
      outcome: 'stored',
      detail: "the write goes to the replica the balancer picked — its own map, nobody else's",
      observed: 'replica-1 holds 1 session(s) after storing s-local-ada — the only copy in the fleet',
    },
    {
      step: 2,
      phase: 'local-memory',
      label: 'the next request hops elsewhere (replica-2)',
      route: 'round-robin',
      replicaId: 'replica-2',
      outcome: 'miss',
      detail: 'process-local state does not follow the request',
      observed: 'replica-2 answers from its own map: 0 session(s) held, token s-local-ada — not found',
    },
    {
      step: 5,
      phase: 'swap',
      label: 'a rolling update replaces the pinned replica',
      route: 'rolling update',
      replicaId: 'replica-1',
      outcome: 'replaced',
      detail: "one replica terminated, a fresh one under the same slot — the k8s manifest's maxUnavailable 0 dance",
      observed: 'replica-1 terminated: 6 request(s) served, 1 session(s) held — both gone',
    },
    {
      step: 12,
      phase: 'shared-store',
      label: 'the lookup after the swap',
      route: 'round-robin',
      replicaId: 'replica-2',
      outcome: 'hit',
      detail: 'externalized state survives the rollout that killed its writers',
      observed: 'replica-2 (a fresh process) reads the shared store: found (ada)',
    },
  ],
  counter: {
    rows: [
      { replicaId: 'replica-1', increments: 2 },
      { replicaId: 'replica-2', increments: 2 },
      { replicaId: 'replica-3', increments: 2 },
    ],
    total: 6,
    note: '6 increments through three different replicas read back as 6 from one store',
  },
  verdicts: [
    {
      id: 'distribution',
      label: 'the balancer spreads the work',
      value: '12 requests landed 4/4/4 across 3 replicas — measured per replica after the round',
    },
    {
      id: 'the-trade',
      label: 'the trade this topic keeps making',
      value: 'in-process state is fast, free, and invisible to every other replica',
    },
  ],
  honesty: 'Every count on this panel is real — the round is deterministic',
};

function stubUseCase(execute: () => Promise<Result<ScalingStatelessDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    SCALING_STATELESS_DEMO_USE_CASE,
    { execute } as unknown as GetScalingStatelessDemoUseCase,
  );
}

describe('ScalingStatelessDemo', () => {
  it('renders loading first, then the config, distribution, steps, counter, and verdicts', async () => {
    stubUseCase(async () => success(structuredClone(fixture)));

    render(<ScalingStatelessDemo />);

    expect(screen.getByTestId('scaling-stateless-loading')).toBeInTheDocument();

    await screen.findByTestId('scaling-stateless-ready');

    const config = screen.getByTestId('scaling-stateless-config');
    expect(config).toHaveTextContent('fleet · 3 replicas · 12 distribution requests');
    expect(config).toHaveTextContent('s-local-ada (replica-local) vs s-shared-ada (shared store)');

    const distribution = screen.getByTestId('scaling-stateless-distribution');
    expect(distribution).toHaveTextContent('distribution · 12 requests');
    expect(distribution).toHaveTextContent('replica-1 4 · replica-2 4 · replica-3 4');
    expect(distribution).toHaveTextContent('4/4/4');

    const steps = screen.getByTestId('scaling-stateless-steps');
    expect(steps).toHaveTextContent('1. [local-memory] replica-1');
    expect(steps).toHaveTextContent('2. [local-memory] replica-2');
    expect(steps).toHaveTextContent('not found');
    expect(steps).toHaveTextContent('5. [swap] replica-1');
    expect(steps).toHaveTextContent('6 request(s) served, 1 session(s) held');
    expect(steps).toHaveTextContent('12. [shared-store] replica-2');
    expect(steps).toHaveTextContent('found (ada)');

    const counter = screen.getByTestId('scaling-stateless-counter');
    expect(counter).toHaveTextContent('counter · replica-1 +2 · replica-2 +2 · replica-3 +2');
    expect(counter).toHaveTextContent('shared reads 6');

    expect(screen.getByTestId('scl-verdict-distribution')).toHaveTextContent('4/4/4');
    expect(screen.getByTestId('scaling-stateless-honesty')).toHaveTextContent('deterministic');
  });

  it('replays the round through the same use case when the button fires', async () => {
    const execute = vi.fn(async () => success(structuredClone(fixture)));
    stubUseCase(execute);

    render(<ScalingStatelessDemo />);
    await screen.findByTestId('scaling-stateless-ready');
    expect(execute).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByTestId('scaling-stateless-replay'));

    await waitFor(() => expect(execute).toHaveBeenCalledTimes(2));
    expect(await screen.findByTestId('scaling-stateless-ready')).toBeInTheDocument();
  });

  it('degrades to the shared error panel with recovery', async () => {
    stubUseCase(async () =>
      failed(new ServerFailure('the statelessness round endpoint is unreachable')),
    );

    render(<ScalingStatelessDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('the statelessness round');
    expect(panel).toHaveTextContent('unreachable');

    const recover = vi.fn(async () => success(structuredClone(fixture)));
    container.register(
      SCALING_STATELESS_DEMO_USE_CASE,
      { execute: recover } as unknown as GetScalingStatelessDemoUseCase,
    );

    await fireEvent.click(screen.getByRole('button', { name: /try again/i }));
    expect(await screen.findByTestId('scaling-stateless-ready')).toBeInTheDocument();
  });
});
