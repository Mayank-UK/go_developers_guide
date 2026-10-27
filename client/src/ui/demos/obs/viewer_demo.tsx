import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetObsViewerDemoUseCase,
  OBS_VIEWER_DEMO_USE_CASE,
} from '../../../domain/demo/get-obs-viewer-demo-usecase';
import type { ObsViewerDemoData } from '../../../domain/demo/obs-viewer';

/**
 * ObsViewerDemo — the observability topic's slog viewer demo (demo
 * block id `obs:viewer`). It renders the payload of
 * GET /api/demo/obs/viewer: one REAL http_request log line dissected
 * field by field (what wrote each key, what a consumer joins on it),
 * the level matrix (which GDG_LOG_LEVEL configurations let each
 * severity through — computed server-side by really running the gate),
 * and the replay: a 200 and a 404 walked through the production
 * middleware chain in-process, their captured JSON lines shown with
 * the one request id threading them.
 */

type ViewerStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: ObsViewerDemoData }
  | { state: 'error'; failure: Failure };

export function useObsViewerDemo(
  useCase: GetObsViewerDemoUseCase = container.resolve(OBS_VIEWER_DEMO_USE_CASE),
): { status: ViewerStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<ViewerStatus>({ state: 'idle' });

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

export function ObsViewerDemo() {
  const { status, retry } = useObsViewerDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="obs-viewer-loading" className="text-body-sm text-on-surface-variant">
        capturing a real request's log lines…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the slog viewer demo" failure={status.failure} onRetry={retry} />;
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid="obs-viewer-ready">
      <section
        data-testid="obs-viewer-anatomy"
        className="flex flex-col gap-sm rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-on-surface">{data.anatomy.line}</p>
        {data.anatomy.fields.map((field) => (
          <p key={field.key} data-testid={`obs-viewer-field-${field.key}`} className="text-caption">
            <span className="font-mono text-secondary">{field.key}</span>{' '}
            <span className="font-mono text-on-surface">{field.value}</span>{' '}
            <span className="text-on-surface-variant">— {field.note}</span>
          </p>
        ))}
        <p className="text-caption text-on-surface">{data.anatomy.note}</p>
      </section>

      <section
        data-testid="obs-viewer-levels"
        className="flex flex-col gap-sm rounded-md border border-outline px-md py-sm"
      >
        {data.levels.rows.map((row) => (
          <p key={row.level} data-testid={`obs-viewer-level-${row.level.toLowerCase()}`} className="text-caption">
            <span className="font-mono text-secondary">{row.level}</span>{' '}
            <span className="text-on-surface">survives at: {row.survivesAt.join(', ')}</span>{' '}
            <span className="text-on-surface-variant">— {row.emittedBy}</span>
          </p>
        ))}
        <p className="text-caption text-on-surface-variant">
          {data.levels.gate} — {data.levels.note}
        </p>
      </section>

      <section
        data-testid="obs-viewer-replay"
        className="flex flex-col gap-sm rounded-md border border-outline px-md py-sm"
      >
        {data.replay.requests.map((request) => (
          <p key={request.label} className="font-mono text-caption text-on-surface">
            <span className="text-secondary">{request.label}</span> {request.request} →{' '}
            {String(request.status)}{' '}
            <span className="text-on-surface-variant">id {request.requestId}</span>
          </p>
        ))}
        {data.replay.lines.map((line, index) => (
          <p
            key={`${line.source}-${String(index)}`}
            data-testid={`obs-viewer-line-${String(index + 1)}`}
            className="font-mono text-caption text-on-surface-variant"
          >
            {line.level ? <span className="text-primary">{line.level} </span> : null}
            {line.msg ? <span className="text-on-surface">{line.msg} </span> : null}
            {line.requestId ? <span className="text-secondary">[{line.requestId}] </span> : null}
            {line.raw}
          </p>
        ))}
        <p className="text-caption text-on-surface">{data.replay.note}</p>
      </section>

      <p data-testid="obs-viewer-reasoning" className="text-body-sm text-on-surface-variant">
        {data.reasoning}
      </p>
    </div>
  );
}

export default ObsViewerDemo;
