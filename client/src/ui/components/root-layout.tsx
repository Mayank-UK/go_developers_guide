import { Suspense, useEffect } from 'react';
import { Link, Outlet, useLocation, useNavigate } from 'react-router';
import { ThemeToggle } from './theme-toggle';
import { RoutePaths } from '../routing/routes';
import { initAnalytics } from '../analytics/analytics';

/**
 * RootLayout — the single shared app shell: a quiet header (guide name as a
 * home link + theme toggle) and a scrolling content area via <Outlet/>. The
 * home screen is the navigation hub (the served topic map arrives with the
 * content engine), so the header carries no drawer and no extra chrome — the
 * served content carries the page. The one extra control is the back arrow,
 * shown on every route except home (the hub), so any nested page has a one-
 * click way back through history to where the user came from.
 *
 * This is also the app's ONE Suspense boundary for route views: the home view
 * is a lazy() chunk (see ui/routing/router.tsx) and resolves behind this
 * single fallback, so navigation shows exactly one consistent loading state.
 */
export function RootLayout() {
  const location = useLocation();
  const navigate = useNavigate();
  const canGoBack = location.pathname !== RoutePaths.Home;

  // Self-hosted analytics beacon (Umami — see ui/analytics/analytics.ts):
  // shell-level so it loads once per document for EVERY route. A no-op
  // unless the build baked both VITE_UMAMI_* vars in; the loader's
  // idempotence guard absorbs StrictMode's dev double-mount.
  useEffect(() => {
    initAnalytics();
  }, []);

  return (
    <div className="flex min-h-screen flex-col bg-surface-tint text-on-surface">
      <header className="sticky top-0 z-20 flex h-header items-center gap-md border-b border-outline bg-surface px-lg">
        {canGoBack && (
          <button
            type="button"
            onClick={() => {
              void navigate(-1);
            }}
            className="cursor-pointer rounded-md px-sm py-xxs text-body text-on-surface-variant hover:bg-surface-variant hover:text-on-surface"
            aria-label="Go back"
          >
            ←
          </button>
        )}
        <Link
          to={RoutePaths.Home}
          className="truncate font-display text-title-sm text-on-surface hover:text-primary"
        >
          Go Developers Guide
        </Link>
        <div className="flex-1" />
        <ThemeToggle />
      </header>

      <main className="flex flex-1 flex-col">
        <Suspense
          fallback={
            <div
              data-testid="route-loading"
              className="mx-auto flex w-full max-w-content flex-col gap-md px-md py-xl"
              aria-hidden
            >
              <p className="text-body text-on-surface-variant">loading…</p>
              <div className="h-lg w-full animate-pulse rounded-md bg-surface-variant" />
              <div className="h-lg w-2/3 animate-pulse rounded-md bg-surface-variant" />
            </div>
          }
        >
          <Outlet />
        </Suspense>
      </main>
    </div>
  );
}
