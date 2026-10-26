import type { Failure } from '../../core/error-handling/failures';

interface ErrorPanelProps {
  /** What the user was trying to load, e.g. "the health probe" or "this topic". */
  subject: string;
  failure: Failure;
  /** Optional recovery: renders a "Try again" button that re-runs the failed load. */
  onRetry?: () => void;
}

/**
 * ErrorPanel — the full-screen failure state for a Critical failure: shows the
 * failure's user message plus the machine diagnostics (code, HTTP status, and
 * the server's requestId when present) so any failure can be traced in the
 * server logs. Used wherever a screen's data fetch resolved to a Failed
 * Result; non-blocking severities would degrade inline instead (chips/notes).
 * When the caller passes `onRetry`, the panel offers a one-click reload of
 * whatever failed — the recovery path is first-class, not a page refresh.
 */
export function ErrorPanel({ subject, failure, onRetry }: ErrorPanelProps) {
  return (
    <section
      role="alert"
      data-testid="error-panel"
      className="flex w-full flex-col gap-xs rounded-lg border border-outline bg-surface-tint p-md shadow-button-card"
    >
      <h3 className="font-display text-title-sm text-error">
        Could not load {subject}
      </h3>
      <p className="text-body-sm text-on-surface-variant">{failure.message}</p>
      <p className="font-mono text-caption text-on-surface-variant">
        {failure.code} · status {String(failure.status)}
        {failure.requestId !== undefined && ` · request ${failure.requestId}`}
      </p>
      {onRetry !== undefined && (
        <button
          type="button"
          onClick={onRetry}
          className="mt-xxs self-start rounded-pill border border-outline px-sm py-xxs font-mono text-caption uppercase tracking-wide text-on-surface-variant transition-colors hover:border-primary hover:text-primary"
        >
          Try again
        </button>
      )}
    </section>
  );
}
