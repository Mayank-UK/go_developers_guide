import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, within } from '@testing-library/react';
import { createMemoryRouter, RouterProvider, type RouteObject } from 'react-router';
import { ThemeProvider } from '../design-system/theme-provider';
import { RootLayout } from './root-layout';
import { appRoutes } from '../routing/router';
import { RoutePaths, topicPath } from '../routing/routes';
import { useHeaderTitle } from '../hooks/use-header-title';
import { container } from '../../core/container';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch } from '../../test/fetch_stub';

/**
 * RootLayout header spec — two behaviors through the REAL route tree (the
 * same memory-router harness as the routing spec, so the shell wiring is
 * exercised, not the component in isolation):
 *
 * - the back arrow's route-dependent behavior: absent on home (the
 *   navigation hub — nothing to walk back to), present on every other route
 *   (topic pages and the unknown-route fallback), and a click walks history
 *   back to where the user came from;
 * - the header title's lifecycle on a topic page: the route handle's
 *   generic 'Topic' while the served content loads, the served title once
 *   it lands, and the release back to the handle on navigating home.
 *
 * The override mechanism itself (a screen calling useHeaderTitle) is pinned
 * on a synthetic shell: plain outlet children behind the real RootLayout, so
 * the set/release cycle is observed without a fetch fixture.
 *
 * The header h1 and each screen's own h1 can share an accessible name (home
 * hero and banner both say the guide name; the banner mirrors the served
 * topic title), so heading assertions are scoped: getByRole('banner') for
 * the header, getByRole('main') for the page.
 *
 * The content views are lazy chunks behind Suspense; under full-suite jsdom
 * load a fork's first pull of a view's module graph can exceed findBy's
 * default 1s, so awaits gated on a lazy view's first paint carry an explicit
 * generous timeout — identical assertions, patient under load.
 */

beforeAll(() => {
  container.initialize(createAppConfig());
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function renderAt(path: string) {
  const router = createMemoryRouter(appRoutes(), { initialEntries: [path] });
  return render(
    <ThemeProvider>
      <RouterProvider router={router} />
    </ThemeProvider>,
  );
}

const contentIndex = {
  version: 1,
  groups: [{ id: 'foundations', title: 'Foundations', order: 1 }],
  topics: [
    {
      id: 'go-language',
      title: 'Go Language',
      group: 'foundations',
      order: 1,
      sectionCount: 1,
    },
  ],
};

const topicDetail = {
  version: 1,
  id: 'go-language',
  title: 'Go Language',
  group: 'foundations',
  order: 1,
  sections: [
    { id: 's1', title: 'The section', blocks: [{ kind: 'text', id: 'b1', text: 'The body.' }] },
  ],
};

/**
 * The header region — where the route-handle / override title renders.
 * getAllByRole + first match: jsdom's role computation also counts the home
 * hero's `<header>` (inside <main>) as a banner, and the sticky shell header
 * always precedes it in DOM order.
 */
function banner() {
  const [shellHeader] = screen.getAllByRole('banner');
  return within(shellHeader as HTMLElement);
}

describe('root layout header back button', () => {
  it('is absent on home (the navigation hub)', async () => {
    stubApiFetch([
      { url: '/api/health', body: { status: 'ok', env: 'dev' } },
      { url: '/api/content', body: contentIndex },
    ]);

    renderAt(RoutePaths.Home);

    // The header shows the home route's handle title immediately; the lazy
    // home view then paints its own hero h1 inside main.
    expect(
      banner().getByRole('heading', { level: 1, name: 'Go Developers Guide' }),
    ).toBeInTheDocument();
    expect(
      await within(screen.getByRole('main')).findByRole(
        'heading',
        { level: 1, name: 'Go Developers Guide' },
        { timeout: 5000 },
      ),
    ).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Go back' })).not.toBeInTheDocument();
  });

  it('is present on a topic route, where the header follows the served title', async () => {
    stubApiFetch([{ url: '/api/content/go-language', body: topicDetail, delayMs: 30 }]);

    renderAt(topicPath('go-language'));

    // While the topic loads, the header keeps the route handle's generic
    // title — never a blank header, never a stale one.
    expect(
      banner().getByRole('heading', { level: 1, name: 'Topic' }),
    ).toBeInTheDocument();

    // Once served, the topic screen's useHeaderTitle override takes over.
    expect(
      await banner().findByRole('heading', { level: 1, name: 'Go Language' }, { timeout: 5000 }),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Go back' })).toBeInTheDocument();
  });

  it('is present on the unknown-route fallback (any non-home path shows it)', async () => {
    stubApiFetch([]);

    renderAt('/nowhere/at/all');

    expect(
      await within(screen.getByRole('main')).findByRole('heading', {
        level: 1,
        name: 'Page not found',
      }),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Go back' })).toBeInTheDocument();
  });

  it('clicking it walks history back to the previous page', async () => {
    stubApiFetch([
      { url: '/api/health', body: { status: 'ok', env: 'dev' } },
      { url: '/api/content', body: contentIndex },
      { url: '/api/content/go-language', body: topicDetail },
    ]);

    renderAt(RoutePaths.Home);

    // Home → topic via the topic card, creating the history entry to walk back through.
    fireEvent.click(await screen.findByRole('link', { name: /go language/i }, { timeout: 5000 }));
    expect(
      await banner().findByRole('heading', { level: 1, name: 'Go Language' }, { timeout: 5000 }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Go back' }));

    // navigate(-1) returns to home — the hub heading is back, the arrow hides
    // again, and the topic screen's title override was released with its
    // unmount (the header must not keep showing the previous topic).
    expect(
      await within(screen.getByRole('main')).findByRole(
        'heading',
        { level: 1, name: 'Go Developers Guide' },
        { timeout: 5000 },
      ),
    ).toBeInTheDocument();
    expect(
      banner().getByRole('heading', { level: 1, name: 'Go Developers Guide' }),
    ).toBeInTheDocument();
    expect(
      banner().queryByRole('heading', { level: 1, name: 'Go Language' }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Go back' })).not.toBeInTheDocument();
  });
});

/* ---- header title override: synthetic children behind the real shell ---- */

/** A screen that fine-tunes the header title with data only it knows. */
function TitledChild() {
  useHeaderTitle('Served Topic Title');
  return <p>titled outlet</p>;
}

function shellRoutes(withTitledChild: boolean): RouteObject[] {
  return [
    {
      element: <RootLayout />,
      children: [
        { path: '/', element: <p>home outlet</p>, handle: { title: 'Shell Home' } },
        {
          path: '/titled',
          element: withTitledChild ? <TitledChild /> : <p>titled outlet</p>,
          handle: { title: 'Route Title' },
        },
        // No handle — pins the defensive guide-name floor.
        { path: '/bare', element: <p>bare outlet</p> },
      ],
    },
  ];
}

function renderShell(initialEntry: string, withTitledChild = false) {
  const router = createMemoryRouter(shellRoutes(withTitledChild), {
    initialEntries: [initialEntry],
  });
  render(
    <ThemeProvider>
      <RouterProvider router={router} />
    </ThemeProvider>,
  );
  return router;
}

describe('root layout header title', () => {
  it('renders the matched route handle title', () => {
    renderShell('/titled');

    expect(
      banner().getByRole('heading', { level: 1, name: 'Route Title' }),
    ).toBeInTheDocument();
  });

  it('floors to the guide name on a route with no handle (defensive)', () => {
    renderShell('/bare');

    expect(
      banner().getByRole('heading', { level: 1, name: 'Go Developers Guide' }),
    ).toBeInTheDocument();
  });

  it("wins over the handle while the calling screen is mounted, and releases on unmount", async () => {
    const router = renderShell('/titled', true);

    // The screen's override replaces the route handle's title.
    expect(
      banner().getByRole('heading', { level: 1, name: 'Served Topic Title' }),
    ).toBeInTheDocument();
    expect(
      banner().queryByRole('heading', { level: 1, name: 'Route Title' }),
    ).not.toBeInTheDocument();

    // Navigate inside act so the router transition + the override-release
    // state update flush before the assertions read the DOM.
    await act(async () => {
      await router.navigate('/');
    });
    expect(screen.getByText('home outlet')).toBeInTheDocument();
    expect(
      banner().getByRole('heading', { level: 1, name: 'Shell Home' }),
    ).toBeInTheDocument();
    expect(
      banner().queryByRole('heading', { level: 1, name: 'Served Topic Title' }),
    ).not.toBeInTheDocument();
  });
});
