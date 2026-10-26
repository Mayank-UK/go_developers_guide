import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { HomeView } from './home_view';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../../test/fetch_stub';

/**
 * HomeView spec — behavior, not implementation: loading → index render,
 * empty-registry degradation, error degradation with retry recovery, and the
 * live health chip. The view resolves use cases through the (real) container;
 * fetch is stubbed.
 */

beforeAll(() => {
  container.initialize(createAppConfig());
});

afterEach(() => {
  vi.unstubAllGlobals();
});

const healthOk = { status: 'ok', env: 'dev' };

/** Groups deliberately out of wire order to prove the client sorts by `order`. */
const contentIndex = {
  version: 1,
  groups: [
    { id: 'apis', title: 'APIs', order: 2 },
    { id: 'foundations', title: 'Foundations', order: 1 },
  ],
  topics: [
    { id: 'api-design', title: 'API Design', group: 'apis', order: 1, sectionCount: 0 },
    {
      id: 'go-language',
      title: 'Go Language',
      subtitle: 'Why Go on the server',
      group: 'foundations',
      order: 1,
      sectionCount: 7,
    },
    {
      id: 'go-http-fundamentals',
      title: 'Go HTTP Fundamentals',
      subtitle: 'From go mod init to a running server',
      group: 'foundations',
      order: 2,
      sectionCount: 8,
    },
  ],
};

const emptyIndex = { version: 1, groups: [], topics: [] };

function renderHome() {
  return render(
    <MemoryRouter>
      <HomeView />
    </MemoryRouter>,
  );
}

describe('HomeView', () => {
  it('renders loading first, then the topic map: groups in order, topic links with subtitle + section count, live health chip', async () => {
    stubApiFetch([
      { url: '/api/health', body: healthOk },
      { url: '/api/content', body: contentIndex },
    ]);

    renderHome();

    // Loading state comes first.
    expect(screen.getByTestId('topic-map-loading')).toBeInTheDocument();
    expect(screen.getByTestId('health-chip')).toHaveTextContent('api: checking…');

    // The health chip flips to the served status.
    await waitFor(() =>
      expect(screen.getByTestId('health-chip')).toHaveTextContent('api: ok · dev'),
    );

    // Topic cards arrive: populated ones are links carrying subtitle + count.
    const populated = await screen.findByTestId('topic-card-go-http-fundamentals');
    expect(populated).toHaveAttribute('href', '/topics/go-http-fundamentals');
    expect(populated).toHaveTextContent('Go HTTP Fundamentals');
    expect(populated).toHaveTextContent('From go mod init to a running server');
    expect(populated).toHaveTextContent('8 sections');

    // sectionCount === 0 topics degrade to disabled "soon" cards (no link).
    const soon = screen.getByTestId('topic-card-api-design');
    expect(soon.querySelector('a')).toBeNull();
    expect(soon).toHaveAttribute('aria-disabled', 'true');
    expect(soon).toHaveTextContent('soon');

    // Group display order follows `order`, not wire order.
    const foundations = screen.getByText('Foundations');
    const apis = screen.getByText('APIs');
    expect(
      foundations.compareDocumentPosition(apis) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  });

  it('renders the friendly landing-soon state for an EMPTY registry (zero topics)', async () => {
    stubApiFetch([
      { url: '/api/health', body: healthOk },
      { url: '/api/content', body: emptyIndex },
    ]);

    renderHome();

    const landing = await screen.findByTestId('curriculum-landing-soon');
    expect(landing).toHaveTextContent('The curriculum is landing soon');
    expect(landing).toHaveTextContent('being authored');
    // No crash, no topic cards, no error panel — an empty registry renders
    // the landing-soon note and nothing else.
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.queryByTestId('topic-map-loading')).not.toBeInTheDocument();
    expect(screen.queryByTestId('topic-card-go-language')).not.toBeInTheDocument();
  });

  it('shows the error panel with envelope diagnostics, then recovers when retry re-runs the load', async () => {
    stubApiFetch([
      { url: '/api/health', body: healthOk },
      {
        url: '/api/content',
        status: 500,
        body: {
          error: { code: 'internal', message: 'boom', requestId: 'req-1' },
        },
      },
    ]);

    renderHome();

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Could not load the topic map');
    expect(alert).toHaveTextContent('boom');
    expect(alert).toHaveTextContent('internal · status 500 · request req-1');

    // Retry: the second load succeeds and the topic map takes over.
    stubApiFetch([{ url: '/api/content', body: contentIndex }]);
    fireEvent.click(screen.getByRole('button', { name: /try again/i }));

    expect(
      await screen.findByTestId('topic-card-go-http-fundamentals'),
    ).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('degrades the health chip inline when the server is unreachable (status 0)', async () => {
    stubNetworkFailure();

    renderHome();

    await waitFor(() =>
      expect(screen.getByTestId('health-chip')).toHaveTextContent('api: down'),
    );
    // Both the health probe and the topic map fail; the content error is the
    // screen-level alert, the health failure stays in the chip.
    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Could not load the topic map');
    expect(alert).toHaveTextContent('Could not reach the server.');
    expect(alert).toHaveTextContent('network_error · status 0');
  });
});
