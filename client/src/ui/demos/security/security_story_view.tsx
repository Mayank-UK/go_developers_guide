import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import type { GetSecurityStoryDemoUseCase } from '../../../domain/demo/get-security-story-demo-usecase';
import type { SecurityStoryDemoData } from '../../../domain/demo/security-story';

/**
 * SecurityStoryView — the shared renderer for the security topic's two
 * story demos. The server deliberately serves both payloads in one
 * shape (mechanism, beats with verdicts and facts, proofs, reasoning),
 * so the client renders them with one component and each demo file is
 * a thin named wrapper resolving its own container token. Verdict
 * colors: ok → primary, rejected → error (the 429 boundary, a failed
 * audit), info → muted.
 */

type StoryStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: SecurityStoryDemoData }
  | { state: 'error'; failure: Failure };

/** The per-demo knobs: the container token, testid prefix, and copy. */
export interface SecurityStoryViewProps {
  token: symbol;
  testidPrefix: string;
  loadingSentence: string;
  subject: string;
}

function verdictClass(verdict: string): string {
  if (verdict === 'ok') return 'text-primary';
  if (verdict === 'rejected') return 'text-error';
  return 'text-on-surface-variant';
}

export function useSecurityStoryDemo(useCase: GetSecurityStoryDemoUseCase): {
  status: StoryStatus;
  retry: () => void;
} {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<StoryStatus>({ state: 'idle' });

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

export function SecurityStoryView({
  token,
  testidPrefix,
  loadingSentence,
  subject,
}: SecurityStoryViewProps) {
  // Resolved per RENDER (the family's hook-default pattern): a retry
  // re-resolves the token, so a spec — or the container — can swap the
  // registration between renders and the demo picks it up.
  const useCase = container.resolve<GetSecurityStoryDemoUseCase>(token);
  const { status, retry } = useSecurityStoryDemo(useCase);

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid={`${testidPrefix}-loading`} className="text-body-sm text-on-surface-variant">
        {loadingSentence}
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject={subject} failure={status.failure} onRetry={retry} />;
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid={`${testidPrefix}-ready`}>
      <p className="font-mono text-caption text-on-surface-variant">
        mechanism <span className="text-secondary">{data.mechanism}</span> — one request, the whole
        story run live against the guide&apos;s security engines
      </p>

      {data.beats.map((beat) => (
        <section
          key={beat.beat}
          data-testid={`${testidPrefix}-beat-${slug(beat.beat)}`}
          className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
        >
          <p className={`font-mono text-caption ${verdictClass(beat.verdict)}`}>
            <span className="text-secondary">{beat.beat}</span> · {beat.verdict}
          </p>
          <p className="text-caption text-on-surface">{beat.detail}</p>
          {beat.facts.map((fact) => (
            <p key={fact.label} className="font-mono text-caption text-on-surface-variant">
              {fact.label}: <span className="text-on-surface">{fact.value}</span>
            </p>
          ))}
        </section>
      ))}

      <section
        data-testid={`${testidPrefix}-proofs`}
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-primary">the derived proofs</p>
        {data.proofs.map((proof) => (
          <p key={proof.label} className="text-caption text-on-surface">
            <span className="font-mono text-secondary">{proof.label}</span> — {proof.value}
          </p>
        ))}
      </section>

      <p data-testid={`${testidPrefix}-reasoning`} className="text-body-sm text-on-surface-variant">
        {data.reasoning}
      </p>
    </div>
  );
}

/** slug turns a beat label into a stable testid fragment. */
function slug(label: string): string {
  return label
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}
