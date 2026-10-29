import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { ThemeProvider } from '../design-system/theme-provider';
import { appRoutes } from './router';
import { RoutePaths, topicPath } from './routes';
import { container } from '../../core/container';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch } from '../../test/fetch_stub';

/**
 * Routing spec — drives the REAL route tree (appRoutes + RootLayout + the lazy
 * home view behind the single Suspense boundary) through an equivalent memory
 * router: the unknown-route degradation and the lazy home view both prove the
 * shell wiring, not just the view components.
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

describe('routing', () => {
  it('renders the friendly not-found view for an unknown route', async () => {
    renderAt('/nowhere/at/all');

    expect(
      await within(screen.getByRole('main')).findByRole('heading', {
        level: 1,
        name: 'Page not found',
      }),
    ).toBeInTheDocument();
    expect(screen.getByText('The page you are looking for does not exist.')).toBeInTheDocument();
    // The shared chrome (the header's route-title h1 + theme toggle) stays
    // present — the banner mirrors the route handle's title.
    expect(
      within(screen.getByRole('banner')).getByRole('heading', {
        level: 1,
        name: 'Page not found',
      }),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /switch to/i })).toBeInTheDocument();
  });

  it('lazy-loads the home view behind the single Suspense boundary', async () => {
    stubApiFetch([
      { url: '/api/health', body: { status: 'ok', env: 'dev' } },
      { url: '/api/content', body: { version: 1, groups: [], topics: [] } },
    ]);

    renderAt(RoutePaths.Home);

    // The chunk resolves; then the home hero takes over from the fallback
    // (scoped to main — the banner header also renders an h1 with the route
    // handle's title).
    expect(
      await within(screen.getByRole('main')).findByRole(
        'heading',
        { level: 1, name: 'Go Developers Guide' },
        { timeout: 5000 },
      ),
    ).toBeInTheDocument();
    expect(await screen.findByTestId('curriculum-landing-soon')).toBeInTheDocument();
  });

  it('lazy-loads the topic view for /topics/:topicId and renders the served topic', async () => {
    stubApiFetch([
      {
        url: '/api/content/go-http-fundamentals',
        body: {
          version: 1,
          id: 'go-http-fundamentals',
          title: 'Go HTTP Fundamentals',
          group: 'framework',
          order: 5,
          sections: [
            { id: 's1', title: 'The section', blocks: [{ kind: 'text', id: 'b1', text: 'The body.' }] },
          ],
        },
      },
    ]);

    renderAt(topicPath('go-http-fundamentals'));

    // The article itself waits out the lazy chunk; its h1 paints with it.
    const article = await screen.findByRole('article', {}, { timeout: 5000 });
    expect(
      within(article).getByRole('heading', { level: 1, name: 'Go HTTP Fundamentals' }),
    ).toBeInTheDocument();
    expect(screen.getByText('The body.')).toBeInTheDocument();
  });
});
