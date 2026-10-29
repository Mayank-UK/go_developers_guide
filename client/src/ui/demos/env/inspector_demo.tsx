import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import type { EnvInspectorDemoData } from '../../../domain/demo/env-inspector';
import {
  GetEnvInspectorDemoUseCase,
  ENV_INSPECTOR_DEMO_USE_CASE,
} from '../../../domain/demo/get-env-inspector-demo-usecase';

/**
 * EnvInspectorDemo — the environments topic's env inspector demo (demo
 * block id `env:inspector`). It renders the payload of
 * GET /api/demo/env/inspector: the resolved-config table (every
 * variable: name, source — environment or default — validated value,
 * redacted where sensitive), the twelve-factor scorecard (the same
 * twelve verdicts the topic's prose walked, as data), and the live
 * flag panel (the ScenarioStore's CURRENT state — flip it with a
 * demo's embedded scenario control, or PATCH /api/demo/scenario,
 * and reload to watch the panel follow).
 */

type InspectorStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: EnvInspectorDemoData }
  | { state: 'error'; failure: Failure };

export function useEnvInspectorDemo(
  useCase: GetEnvInspectorDemoUseCase = container.resolve(ENV_INSPECTOR_DEMO_USE_CASE),
): { status: InspectorStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<InspectorStatus>({ state: 'idle' });

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

export function EnvInspectorDemo() {
  const { status, retry } = useEnvInspectorDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="env-inspector-loading" className="text-body-sm text-on-surface-variant">
        deriving this process’s resolved configuration…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the env inspector demo" failure={status.failure} onRetry={retry} />;
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid="env-inspector-ready">
      <section
        data-testid="env-inspector-variables"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="text-caption text-on-surface-variant">{data.mode}</p>
        <p className="font-mono text-caption text-on-surface">
          env: {data.env} · {String(data.variables.length)} variables
        </p>
        {data.variables.map((variable) => (
          <p key={variable.envVar} className="font-mono text-caption text-on-surface-variant">
            {variable.envVar}{' '}
            <span className="text-on-surface">{variable.value}</span>{' '}
            {variable.sensitive ? <span className="text-on-surface">[redacted]</span> : null} — from{' '}
            {variable.source}
          </p>
        ))}
        <p className="text-caption text-on-surface-variant">{data.redaction}</p>
      </section>

      <section
        data-testid="env-inspector-scorecard"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        {data.twelveFactor.map((row) => (
          <p key={row.factor} className="font-mono text-caption">
            <span className="text-on-surface">{row.factor}</span>{' '}
            <span className="text-on-surface-variant">— {row.verdict} — {row.how}</span>
          </p>
        ))}
      </section>

      <section
        data-testid="env-inspector-flags"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <p className="font-mono text-caption text-on-surface">
          flags · dataMode: {data.flags.dataMode} · injectFailure: {data.flags.injectFailure}
        </p>
        <p className="text-caption text-on-surface-variant">
          the ScenarioStore’s current state, snapshotted at request time — flip it with a
          demo’s embedded scenario control (or PATCH /api/demo/scenario) and reload
        </p>
      </section>

      <p data-testid="env-inspector-reasoning" className="text-body-sm text-on-surface-variant">
        {data.reasoning}
      </p>
    </div>
  );
}

export default EnvInspectorDemo;
