import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetAuthzMatrixDemoUseCase,
  AUTHZ_MATRIX_DEMO_USE_CASE,
} from '../../../domain/demo/get-authz-matrix-demo-usecase';
import type {
  AuthzMatrixBeat,
  AuthzMatrixDemoData,
} from '../../../domain/demo/authz-matrix';

/**
 * AuthzMatrixDemo — the authorization topic's live demo (demo block id
 * `authz:matrix`). It renders the payload of GET /api/demo/authz/matrix:
 * one request, the whole decision matrix evaluated server-side through
 * the REAL policy evaluator over the seeded directory and owned posts —
 * the role roster, the seven principals (anonymous through the role-less
 * control), and every subject × action × object cell with the evaluator's
 * own reason sentence. The teaching beats (collection-level reads, owner
 * vs editor vs stranger, scopes narrowing roles, deny by default,
 * namespace isolation) are derived from the cells, so a drifted verdict
 * renders as a red beat rather than a green lie. Verdict colors: ok →
 * primary, rejected → error, alarm → warning, info → muted.
 */

type MatrixStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: AuthzMatrixDemoData }
  | { state: 'error'; failure: Failure };

export function useAuthzMatrixDemo(
  useCase: GetAuthzMatrixDemoUseCase = container.resolve(AUTHZ_MATRIX_DEMO_USE_CASE),
): { status: MatrixStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<MatrixStatus>({ state: 'idle' });

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

export function AuthzMatrixDemo() {
  const { status, retry } = useAuthzMatrixDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="authz-matrix-loading" className="text-body-sm text-on-surface-variant">
        evaluating every cell of the roles × scopes × ownership matrix through the real
        evaluator…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the authorization matrix" failure={status.failure} onRetry={retry} />;
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid="authz-matrix-ready">
      <p className="font-mono text-caption text-on-surface-variant">
        mechanism <span className="text-secondary">{data.mechanism}</span> — one request, every
        verdict the guide&apos;s policy evaluator answered, reason sentences included
      </p>

      {data.beats.map((beat) => (
        <BeatRun key={beat.beat} beat={beat} />
      ))}

      <section
        data-testid="authz-matrix-proofs"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-primary">the derived proofs</p>
        {data.proofs.map((proof) => (
          <p key={proof.label} className="text-caption text-on-surface">
            <span className="font-mono text-secondary">{proof.label}</span> — {proof.value}
          </p>
        ))}
      </section>

      <p data-testid="authz-matrix-reasoning" className="text-body-sm text-on-surface-variant">
        {data.reasoning}
      </p>
    </div>
  );
}

function BeatRun({ beat }: { beat: AuthzMatrixBeat }) {
  const verdictClass =
    beat.verdict === 'ok'
      ? 'text-primary'
      : beat.verdict === 'rejected'
        ? 'text-error'
        : beat.verdict === 'alarm'
          ? 'text-warning'
          : 'text-on-surface-variant';
  return (
    <section
      data-testid={`authz-matrix-beat-${slug(beat.beat)}`}
      className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
    >
      <p className={`font-mono text-caption ${verdictClass}`}>
        <span className="text-secondary">{beat.beat}</span> · {beat.verdict}
      </p>
      <p className="text-caption text-on-surface">{beat.detail}</p>
      {beat.facts.map((fact) => (
        <p key={fact.label} className="font-mono text-caption text-on-surface-variant">
          {fact.label}: <span className="text-on-surface">{fact.value}</span>
        </p>
      ))}
    </section>
  );
}

/** slug turns a beat label into a stable testid fragment. */
function slug(label: string): string {
  return label
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

export default AuthzMatrixDemo;
