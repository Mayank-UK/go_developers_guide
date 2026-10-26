import { Link } from 'react-router';
import { RoutePaths } from '../../routing/routes';

/**
 * NotFoundView — the friendly catch-all for unknown routes (any URL that is
 * not the home screen). A topic id the served content does not know is NOT
 * this screen's job — that arrives with the topic view (Phase 2), handled by
 * its error state with the server's requestId.
 */
export function NotFoundView() {
  return (
    <section className="mx-auto flex w-full max-w-content flex-col items-center gap-sm px-md py-xl text-center">
      <p className="font-mono text-overline uppercase tracking-[0.08em] text-on-surface-variant">
        404
      </p>
      <h1 className="font-display text-display-sm text-on-surface">Page not found</h1>
      <p className="text-body text-on-surface-variant">
        The page you are looking for does not exist.
      </p>
      <Link
        to={RoutePaths.Home}
        className="rounded-md border border-outline px-md py-xs text-body text-primary hover:bg-surface-variant"
      >
        ← back to the guide
      </Link>
    </section>
  );
}
