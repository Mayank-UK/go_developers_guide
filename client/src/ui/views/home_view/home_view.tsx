import { Link } from 'react-router';
import { SectionWrapper } from '../../components/section-wrapper';
import { ErrorPanel } from '../../components/error-panel';
import { topicPath } from '../../routing/routes';
import type { TopicSummary } from '../../../domain/content/content';
import { groupTopics, useContentIndex, useHealth, type HealthChipState } from './home_view.hooks';

/**
 * HomeView — the guide's front door: hero with the live health chip and the
 * curriculum index (topic map) rendered from the content API. Like every
 * screen in this app it owns no teaching content of its own; the topic map is
 * exactly what the server serves — the authored curriculum. An EMPTY index is
 * a valid state (the registry fills in as topics land) and renders a friendly
 * landing note, never a crash.
 */
export function HomeView() {
  const health = useHealth();
  const map = useContentIndex();

  return (
    <div className="mx-auto flex w-full max-w-content flex-col gap-lg px-md py-xl">
      <header className="flex flex-col gap-sm">
        <h1 className="font-display text-display-md text-on-surface">
          Go Developers Guide
        </h1>
        <p className="text-title-sm text-on-surface-variant">
          Backend canon, taught by a working Go app
        </p>
        <HealthChip health={health} />
      </header>

      {(map.status.state === 'idle' || map.status.state === 'loading') && <TopicMapSkeleton />}

      {map.status.state === 'empty' && <EmptyCurriculum />}

      {map.status.state === 'error' && (
        <ErrorPanel subject="the topic map" failure={map.status.failure} onRetry={map.retry} />
      )}

      {map.status.state === 'ready' && (
        <SectionWrapper
          id="topic-map"
          title="Topic map"
          lead="Every topic the guide teaches, grouped like the curriculum. Each card links to its topic page, rendered live from the content API."
        >
          {groupTopics(map.status.index).map(({ group, topics }) =>
            topics.length === 0 ? null : (
              <section key={group.id} aria-label={group.title} className="flex flex-col gap-md">
                <h3 className="font-display text-title text-on-surface">{group.title}</h3>
                <div className="grid grid-cols-1 gap-md sm:grid-cols-2 lg:grid-cols-3">
                  {topics.map((topic) => (
                    <TopicCard key={topic.id} topic={topic} />
                  ))}
                </div>
              </section>
            ),
          )}
        </SectionWrapper>
      )}
    </div>
  );
}

/* ---- hero health chip ---- */

type ChipTone = 'ok' | 'warn' | 'error';

const CHIP_TONE_CLASSES: Record<ChipTone, string> = {
  ok: 'border-secondary/40 text-secondary',
  warn: 'border-warning/40 text-warning',
  error: 'border-error/40 text-error',
};

function HealthChip({ health }: { health: HealthChipState }) {
  const tone: ChipTone =
    health.state === 'ok' ? 'ok' : health.state === 'checking' ? 'warn' : 'error';

  return (
    <div className="flex flex-wrap items-center gap-xs" aria-label="Live API status">
      <span
        data-testid="health-chip"
        className={`rounded-pill border px-sm py-xxs font-mono text-caption uppercase tracking-wide ${CHIP_TONE_CLASSES[tone]}`}
      >
        {health.state === 'checking' && 'api: checking…'}
        {health.state === 'ok' && `api: ${health.status} · ${health.env}`}
        {health.state === 'fail' && 'api: down'}
      </span>
    </div>
  );
}

/* ---- topic map cards ---- */

function TopicCard({ topic }: { topic: TopicSummary }) {
  const badge = (
    <span className="shrink-0 rounded-pill border border-outline px-xs text-overline uppercase tracking-wide text-on-surface-variant">
      {topic.sectionCount === 1 ? '1 section' : `${String(topic.sectionCount)} sections`}
    </span>
  );

  // Zero sections = not authored yet. A served-but-empty summary renders as
  // a plain (non-focusable, keyboard-inert) div — no link, no route — with
  // an explicit coming-soon affordance.
  if (topic.sectionCount === 0) {
    return (
      <div
        data-testid={`topic-card-${topic.id}`}
        aria-disabled="true"
        title="Coming soon"
        className="flex flex-col gap-xs rounded-md border border-outline bg-surface p-md shadow-button-card"
      >
        <div className="flex items-baseline justify-between gap-xs">
          <span className="font-display text-title-sm text-on-surface-variant/60">
            {topic.title}
          </span>
          <span className="shrink-0 text-overline uppercase tracking-wide text-on-surface-variant/50">
            coming soon
          </span>
        </div>
        {topic.subtitle !== undefined && (
          <p className="text-body-sm text-on-surface-variant/60">{topic.subtitle}</p>
        )}
        {badge}
      </div>
    );
  }

  return (
    <Link
      data-testid={`topic-card-${topic.id}`}
      to={topicPath(topic.id)}
      className="group flex flex-col gap-xs rounded-md border border-outline bg-surface p-md shadow-button-card transition-all duration-200 hover:-translate-y-1 hover:border-primary hover:bg-surface-variant hover:shadow-button-card-hover"
    >
      <div className="flex items-baseline justify-between gap-xs">
        <span className="font-display text-title-sm text-on-surface transition-colors group-hover:text-primary">
          {topic.title}
          <span
            aria-hidden
            className="ml-xxs inline-block text-on-surface-variant transition-transform group-hover:translate-x-1 group-hover:text-primary"
          >
            →
          </span>
        </span>
        {badge}
      </div>
      {topic.subtitle !== undefined && (
        <p className="text-body-sm text-on-surface-variant">{topic.subtitle}</p>
      )}
    </Link>
  );
}

/* ---- degradation states ---- */

function TopicMapSkeleton() {
  return (
    <div data-testid="topic-map-loading" aria-hidden className="flex flex-col gap-md">
      <p className="text-body text-on-surface-variant">loading the topic map…</p>
      <div className="h-lg w-full animate-pulse rounded-md bg-surface-variant" />
      <div className="grid grid-cols-1 gap-md sm:grid-cols-2 lg:grid-cols-3">
        {[0, 1, 2, 3, 4, 5].map((index) => (
          <div key={index} className="h-xl animate-pulse rounded-md bg-surface-variant" />
        ))}
      </div>
    </div>
  );
}

function EmptyCurriculum() {
  return (
    <section
      data-testid="curriculum-landing-soon"
      className="flex w-full flex-col items-center gap-sm rounded-lg border border-dashed border-outline bg-surface-tint px-lg py-xl text-center"
    >
      <h2 className="font-display text-title text-on-surface">
        The curriculum is landing soon
      </h2>
      <p className="max-w-prose text-body text-on-surface-variant">
        The content API is live and serving, but the registry is empty while
        topics are still being authored. Topics land phase by phase — each one
        arrives as typed content with live, inline demos. Ask the API directly
        what is registered today: GET /api/content.
      </p>
    </section>
  );
}

export default HomeView;
