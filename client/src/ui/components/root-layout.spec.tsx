import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { ThemeProvider } from '../design-system/theme-provider';
import { appRoutes } from '../routing/router';
import { RoutePaths, topicPath } from '../routing/routes';
import { container } from '../../core/container';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch } from '../../test/fetch_stub';

/**
 * RootLayout header spec — the back arrow's route-dependent behavior, proven
 * through the REAL route tree (the same memory-router harness as the routing
 * spec, so the shell wiring is exercised, not the component in isolation):
 * absent on home (the navigation hub — nothing to walk back to), present on
 * every other route (topic pages and the unknown-route fallback), and a click
 * walks history back to where the user came from.
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

/** The header panel's mount-time GET; stubbed so every render is deterministic. */
const scenario = {
  url: '/api/demo/scenario',
  body: { dataMode: 'fake', injectFailure: 'none' },
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

describe('root layout header back button', () => {
  it('is absent on home (the navigation hub)', async () => {
    stubApiFetch([
      { url: '/api/health', body: { status: 'ok', env: 'dev' } },
      { url: '/api/content', body: contentIndex },
      scenario,
    ]);

    renderAt(RoutePaths.Home);

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Go Developers Guide' }, { timeout: 5000 }),
    ).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Go back' })).not.toBeInTheDocument();
  });

  it('is present on a topic route', async () => {
    stubApiFetch([{ url: '/api/content/go-language', body: topicDetail }, scenario]);

    renderAt(topicPath('go-language'));

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Go Language' }, { timeout: 5000 }),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Go back' })).toBeInTheDocument();
  });

  it('is present on the unknown-route fallback (any non-home path shows it)', async () => {
    stubApiFetch([scenario]);

    renderAt('/nowhere/at/all');

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Page not found' }),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Go back' })).toBeInTheDocument();
  });

  it('carries the global scenario control next to the theme toggle', async () => {
    stubApiFetch([
      { url: '/api/health', body: { status: 'ok', env: 'dev' } },
      { url: '/api/content', body: contentIndex },
      scenario,
    ]);

    renderAt(RoutePaths.Home);

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Go Developers Guide' }, { timeout: 5000 }),
    ).toBeInTheDocument();
    expect(await screen.findByTestId('scenario-panel')).toBeInTheDocument();
    expect(screen.getByLabelText('Data mode')).toBeInTheDocument();
    expect(screen.getByLabelText('Inject failure')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /switch to .+ theme/i })).toBeInTheDocument();
  });

  it('clicking it walks history back to the previous page', async () => {
    stubApiFetch([
      { url: '/api/health', body: { status: 'ok', env: 'dev' } },
      { url: '/api/content', body: contentIndex },
      { url: '/api/content/go-language', body: topicDetail },
      scenario,
    ]);

    renderAt(RoutePaths.Home);

    // Home → topic via the topic card, creating the history entry to walk back through.
    fireEvent.click(await screen.findByRole('link', { name: /go language/i }, { timeout: 5000 }));
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Go Language' }, { timeout: 5000 }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Go back' }));

    // navigate(-1) returns to home — the hub heading is back and the arrow hides again.
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Go Developers Guide' }, { timeout: 5000 }),
    ).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Go back' })).not.toBeInTheDocument();
  });
});
