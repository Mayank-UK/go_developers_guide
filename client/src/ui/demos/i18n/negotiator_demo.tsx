import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import type { GetI18nNegotiatorDemoUseCase } from '../../../domain/demo/get-i18n-negotiator-demo-usecase';
import { I18N_NEGOTIATOR_DEMO_USE_CASE } from '../../../domain/demo/get-i18n-negotiator-demo-usecase';
import type { I18nStoryDemoData } from '../../../domain/demo/i18n-story';

/**
 * I18nNegotiatorDemo — the i18n topic's `i18n:negotiator` demo: one
 * request renders the server-run story of THIS browser's own
 * Accept-Language header (seven beats — the parsed candidates with
 * their q-values, the fallback chain's selection, the same message per
 * catalog, the plural rules at 0/1/2, the per-locale format matrices,
 * the timezone-is-not-locale beat with its honest unknown-zone error,
 * and the two honest edges). The story is single-demo, so the hook and
 * rendering live here per the family's single-file component rule.
 * Verdict colors: ok → primary, rejected → error, info → muted.
 */

type StoryStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: I18nStoryDemoData }
  | { state: 'error'; failure: Failure };

function verdictClass(verdict: string): string {
  if (verdict === 'ok') return 'text-primary';
  if (verdict === 'rejected') return 'text-error';
  return 'text-on-surface-variant';
}

function useI18nStory(useCase: GetI18nNegotiatorDemoUseCase): {
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

export function I18nNegotiatorDemo() {
  // Resolved per RENDER (the family's hook-default pattern): a retry
  // re-resolves the token, so a spec — or the container — can swap the
  // registration between renders and the demo picks it up.
  const useCase = container.resolve<GetI18nNegotiatorDemoUseCase>(I18N_NEGOTIATOR_DEMO_USE_CASE);
  const { status, retry } = useI18nStory(useCase);

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="i18n-negotiator-loading" className="text-body-sm text-on-surface-variant">
        negotiating with your browser&apos;s own Accept-Language header…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the i18n negotiator demo" failure={status.failure} onRetry={retry} />;
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid="i18n-negotiator-ready">
      <p className="font-mono text-caption text-on-surface-variant">
        mechanism <span className="text-secondary">{data.mechanism}</span> — every beat derives from
        the Accept-Language header this page&apos;s own request carried
      </p>

      {data.beats.map((beat) => (
        <section
          key={beat.beat}
          data-testid={`i18n-negotiator-beat-${slug(beat.beat)}`}
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
        data-testid="i18n-negotiator-proofs"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-primary">the derived proofs</p>
        {data.proofs.map((proof) => (
          <p key={proof.label} className="text-caption text-on-surface">
            <span className="font-mono text-secondary">{proof.label}</span> — {proof.value}
          </p>
        ))}
      </section>

      <p data-testid="i18n-negotiator-reasoning" className="text-body-sm text-on-surface-variant">
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

export default I18nNegotiatorDemo;
