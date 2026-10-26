import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetGRPCConsoleDemoUseCase,
  API_GRPC_CONSOLE_DEMO_USE_CASE,
} from '../../../domain/demo/get-grpc-console-demo-usecase';
import type { GRPCCallReport, GRPCConsoleDemoData } from '../../../domain/demo/grpc-console';

/**
 * GRPCConsoleDemo — the grpc topic's live demo (demo block id
 * `api:grpc-console`). It renders the payload of
 * GET /api/demo/api/grpc-console: real RPCs the server ran in-process
 * over a bufconn listener. One card per exhibit call shows the method
 * kind, the full method path, the observed STATUS CODE (the contract's
 * error vocabulary, name + numeric), the round-trip count, and the result
 * rows — hits and misses side by side, because on gRPC the status code IS
 * the error model: a NotFound miss and a DeadlineExceeded deadline are
 * data, not surprises. The transport card records how the console reached
 * the service (bufconn, passthrough dial, explicit plaintext) — the facts
 * that make it honest rather than narrated.
 *
 * Dependency flow mirrors the views: the hook resolves its use case from
 * the container (the token's default parameter is the spec seam — tests
 * substitute fakes through the same token), failures arrive as typed
 * Failures inside Results, and `retry` re-runs the whole in-process
 * round trip.
 */

type GRPCConsoleStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: GRPCConsoleDemoData }
  | { state: 'error'; failure: Failure };

export function useGRPCConsoleDemo(
  useCase: GetGRPCConsoleDemoUseCase = container.resolve(API_GRPC_CONSOLE_DEMO_USE_CASE),
): { status: GRPCConsoleStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<GRPCConsoleStatus>({ state: 'idle' });

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

export function GRPCConsoleDemo() {
  const { status, retry } = useGRPCConsoleDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="grpc-console-loading" className="text-body-sm text-on-surface-variant">
        dialing bufconn, running the four kinds…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the gRPC console" failure={status.failure} onRetry={retry} />;
  }

  const { transport, calls } = status.data;

  return (
    <div className="flex flex-col gap-md">
      <section
        data-testid="grpc-transport"
        className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <h4 className="font-display text-title-sm text-on-surface">The transport, honestly recorded</h4>
        <p className="font-mono text-caption text-on-surface">{transport.mode}</p>
        <p className="font-mono text-caption text-on-surface-variant">
          dial {transport.dialTarget} · {transport.security}
        </p>
        <p className="font-mono text-caption text-on-surface-variant">
          {transport.service} · contract {transport.protoFile} · codegen:{' '}
          <span className="text-secondary">{transport.codegenCmd}</span> → {transport.generatedGo}
        </p>
      </section>

      {calls.map((call, index) => (
        <CallCard key={`${call.method}-${index}`} call={call} />
      ))}
    </div>
  );
}

function CallCard({ call }: { call: GRPCCallReport }) {
  const healthy = call.statusCode === 'OK';
  return (
    <section
      data-testid={`grpc-call-${call.kind}`}
      className="flex flex-col gap-xs rounded-md border border-outline px-md py-sm"
    >
      <div className="flex flex-wrap items-baseline justify-between gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">
          <span className="font-mono text-secondary">{call.kind}</span> · {call.pattern}
        </h4>
        <span
          data-testid="grpc-status-chip"
          className={`rounded border px-xs py-xxs font-mono text-caption ${
            healthy ? 'border-outline text-on-surface-variant' : 'border-error text-error'
          }`}
        >
          {call.statusCode} ({call.statusNumber})
        </span>
      </div>
      <p className="font-mono text-caption text-on-surface-variant">
        {call.method} · {call.roundTrips} round trip{call.roundTrips === 1 ? '' : 's'}
      </p>
      <p className="text-body-sm text-on-surface">{call.summary}</p>
      <ul className="flex flex-col gap-xxs">
        {call.results.map((result, index) => (
          <li key={index} className="text-body-sm">
            <span className={`font-mono text-caption ${result.ok ? 'text-secondary' : 'text-error'}`}>
              {result.ok ? '✓' : '✗'} {result.label}
            </span>{' '}
            <span className="text-on-surface-variant">— {result.detail}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}

export default GRPCConsoleDemo;
