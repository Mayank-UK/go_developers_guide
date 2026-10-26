import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetValidationArenaDemoUseCase,
  VALIDATION_ARENA_DEMO_USE_CASE,
} from '../../../domain/demo/get-validation-arena-demo-usecase';
import type {
  ArenaCandidate,
  ArenaDecodeOutcome,
  ArenaRulesOutcome,
  ValidationArenaDemoData,
} from '../../../domain/demo/validation-arena';

/**
 * ArenaDemo — the validation topic's live demo (demo block id
 * `validation:arena`). It renders the payload of GET
 * /api/demo/validation/arena: the judged FeedbackForm contract, the
 * three-layer stack, and the candidate roster — each row showing both
 * layers' verdicts and the composed HTTP response, with the REAL 422
 * envelopes the server's central error writer produced. The tag-ecosystem
 * comparison (go-playground/validator taught by comparison, imported
 * flag false) and the dissected envelope anatomy ride along.
 *
 * Dependency flow mirrors the views: the hook resolves its use case from
 * the container (the token's default parameter is the spec seam — tests
 * substitute fakes through the same token), failures arrive as typed
 * Failures inside Results, and `retry` re-runs the load.
 */

type ArenaDemoStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: ValidationArenaDemoData }
  | { state: 'error'; failure: Failure };

export function useArenaDemo(
  useCase: GetValidationArenaDemoUseCase = container.resolve(VALIDATION_ARENA_DEMO_USE_CASE),
): { status: ArenaDemoStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<ArenaDemoStatus>({ state: 'idle' });

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

export function ArenaDemo() {
  const { status, retry } = useArenaDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="validation-arena-loading" className="text-body-sm text-on-surface-variant">
        judging the candidates…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the validation arena" failure={status.failure} onRetry={retry} />;
  }

  const { subject, layers, candidates, comparison, envelope, reasoning } = status.data;

  return (
    <div className="flex flex-col gap-md">
      <section data-testid="va-subject" className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          The judged form: <span className="font-mono">{subject.name}</span>
        </h4>
        <div className="overflow-x-auto">
          <table className="w-full text-left font-mono text-caption">
            <thead className="text-on-surface-variant">
              <tr>
                <th scope="col" className="pr-sm py-xxs">field</th>
                <th scope="col" className="pr-sm py-xxs">type</th>
                <th scope="col" className="pr-sm py-xxs">rule</th>
                <th scope="col" className="py-xxs">required</th>
              </tr>
            </thead>
            <tbody>
              {subject.fields.map((field) => (
                <tr key={field.name} className="border-t border-outline">
                  <td className="pr-sm py-xxs text-on-surface">{field.name}</td>
                  <td className="pr-sm py-xxs text-on-surface-variant">{field.type}</td>
                  <td className="pr-sm py-xxs text-on-surface-variant">{field.rule}</td>
                  <td className={`py-xxs ${field.required ? 'text-secondary' : 'text-on-surface-variant'}`}>
                    {field.required ? 'yes' : 'optional'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section data-testid="va-layers" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">The three layers</h4>
        <ol className="flex flex-col gap-xxs">
          {layers.map((layer) => (
            <li key={layer.id} className="text-body-sm text-on-surface-variant">
              <span className="font-mono text-on-surface">{layer.title}</span> — {layer.summary}
            </li>
          ))}
        </ol>
      </section>

      <section data-testid="va-candidates" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          The candidates, judged by both layers
        </h4>
        <div className="flex flex-col gap-sm">
          {candidates.map((candidate) => (
            <CandidateCard key={candidate.id} candidate={candidate} />
          ))}
        </div>
      </section>

      <section data-testid="va-comparison" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">
          The tag ecosystem, by comparison
        </h4>
        <p className="text-body-sm text-on-surface-variant">
          <span className="font-mono text-on-surface">{comparison.library}</span> — imported:{' '}
          <span className="font-mono text-secondary">{String(comparison.imported)}</span>. {comparison.note}
        </p>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">field</th>
                <th scope="col" className="px-sm py-xxs">tag</th>
                <th scope="col" className="px-sm py-xxs">library rule</th>
                <th scope="col" className="px-sm py-xxs">hand-rolled rule</th>
              </tr>
            </thead>
            <tbody>
              {comparison.rows.map((row) => (
                <tr key={row.field} className="border-t border-outline">
                  <td className="px-sm py-xxs text-on-surface">{row.field}</td>
                  <td className="px-sm py-xxs text-secondary">{row.tag}</td>
                  <td className="px-sm py-xxs text-on-surface-variant">{row.libraryRule}</td>
                  <td className="px-sm py-xxs text-on-surface-variant">{row.handRolled}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section data-testid="va-envelope" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">
          The 422 envelope, dissected
        </h4>
        <pre className="overflow-x-auto rounded-md border border-outline bg-surface-tint px-sm py-xs font-mono text-caption text-on-surface">
          {JSON.stringify(envelope.body, null, 2)}
        </pre>
        <ul className="flex flex-col gap-xxs">
          {envelope.anatomy.map((row) => (
            <li key={row.path} className="text-body-sm text-on-surface-variant">
              <span className="font-mono text-on-surface">{row.path}</span> ={' '}
              <span className="font-mono text-secondary">{row.value}</span> — {row.why}
            </li>
          ))}
        </ul>
      </section>

      <p data-testid="va-reasoning" className="text-body-sm text-on-surface-variant">
        {reasoning}
      </p>
    </div>
  );
}

function CandidateCard({ candidate }: { candidate: ArenaCandidate }) {
  const refused = candidate.response.status !== 200;
  return (
    <article
      data-testid={`va-candidate-${candidate.id}`}
      className={`flex flex-col gap-xxs rounded-md border px-md py-sm ${refused ? 'border-outline' : 'border-outline bg-surface-tint'}`}
    >
      <header className="flex flex-wrap items-baseline gap-x-sm gap-y-xxs">
        <h5 className="font-display text-body-md text-on-surface">{candidate.label}</h5>
        <span
          className={`rounded border px-xs py-xxs font-mono text-caption ${refused ? 'border-error text-error' : 'border-outline text-secondary'}`}
        >
          {candidate.response.status}
        </span>
      </header>
      <p className="font-mono text-caption text-on-surface-variant">{candidate.body}</p>
      <p className="text-body-sm text-on-surface-variant">
        decode <DecodeBadge decode={candidate.decode} /> · rules <RulesBadge rules={candidate.validate} />
      </p>
      {candidate.validate.issues && candidate.validate.issues.length > 0 ? (
        <ul className="flex flex-col gap-xxs">
          {candidate.validate.issues.map((issue) => (
            <li key={issue.field} className="text-body-sm text-on-surface-variant">
              <span className="font-mono text-on-surface">{issue.field}</span> — {issue.rule}{' '}
              <span className="font-mono text-secondary">(got {issue.got})</span>
            </li>
          ))}
        </ul>
      ) : null}
      {candidate.decode.error ? (
        <p className="font-mono text-caption text-error">
          {candidate.decode.errorType}: {candidate.decode.error}
        </p>
      ) : null}
      {candidate.note ? <p className="text-caption text-on-surface-variant">{candidate.note}</p> : null}
    </article>
  );
}

function DecodeBadge({ decode }: { decode: ArenaDecodeOutcome }) {
  return (
    <span className={`font-mono ${decode.status === 'rejected' ? 'text-error' : 'text-secondary'}`}>
      {decode.status}
    </span>
  );
}

function RulesBadge({ rules }: { rules: ArenaRulesOutcome }) {
  const tone = rules.status === 'rejected' ? 'text-error' : rules.status === 'skipped' ? 'text-on-surface-variant' : 'text-secondary';
  return <span className={`font-mono ${tone}`}>{rules.status}</span>;
}

export default ArenaDemo;
