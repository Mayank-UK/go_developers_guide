import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes, createMemoryRouter, RouterProvider } from 'react-router';
import { TopicView } from './topic_view';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../../test/fetch_stub';

/**
 * TopicView spec — the generic renderer's behavior: every block kind in wire
 * order, paragraph splitting, numbered sections, degradation for unknown ids /
 * network loss, and race-safety when the route param changes mid-flight.
 */

beforeAll(() => {
  container.initialize(createAppConfig());
});

afterEach(() => {
  vi.unstubAllGlobals();
});

function renderTopic(topicId: string) {
  return render(
    <MemoryRouter initialEntries={[`/topics/${topicId}`]}>
      <Routes>
        <Route path="/topics/:topicId" element={<TopicView />} />
      </Routes>
    </MemoryRouter>,
  );
}

/** One topic with two sections; the first carries every block kind (the
 * ordered list), the second the unordered list variant. */
const richTopic = {
  version: 1,
  id: 'go-runtime',
  title: 'Go Runtime',
  subtitle: 'The scheduler behind the code',
  group: 'foundations',
  order: 2,
  sections: [
    {
      id: 'go-runtime:rich-section',
      title: 'The rich section',
      lead: 'The lead paragraph.',
      blocks: [
        {
          kind: 'text',
          id: 'b-text',
          text: 'First paragraph.\n\nSecond paragraph.',
        },
        {
          kind: 'code',
          id: 'b-code',
          language: 'go',
          caption: 'the caption',
          snippet: 'type Hello struct {\n    Name string\n}',
          source: 'server/internal/domain/demos/hello.go',
        },
        {
          kind: 'demo',
          id: 'nope:not-registered',
          title: 'The goroutine lab',
          description: 'Watch goroutines schedule in real time.',
        },
        {
          kind: 'list',
          id: 'b-list-ordered',
          ordered: true,
          items: [
            'Init — go mod init the module.',
            'Build — declare types and functions.',
            'Test — run go test ./... at the boundary.',
          ],
        },
      ],
    },
    {
      id: 'go-runtime:terminal',
      title: 'Terminal companion',
      blocks: [
        { kind: 'list', id: 'b-list-unordered', ordered: false, items: ['curl — tour the API.', 'jq — shape the output.', 'less — page long responses.'] },
        { kind: 'text', id: 'b-text-2', text: 'curl away.' },
      ],
    },
  ],
};

describe('TopicView', () => {
  it('renders loading, then the topic: numbered sections, split paragraphs, verbatim code, and the demo panel', async () => {
    stubApiFetch([{ url: '/api/content/go-runtime', body: richTopic }]);

    renderTopic('go-runtime');

    expect(screen.getByTestId('topic-loading')).toBeInTheDocument();

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Go Runtime' }),
    ).toBeInTheDocument();
    expect(screen.getByText('The scheduler behind the code')).toBeInTheDocument();

    // Numbered sections with titles + leads (01, 02). The number joins the
    // heading's accessible name, so title matching uses a regex.
    expect(screen.getByText('01')).toBeInTheDocument();
    expect(
      screen.getByRole('heading', { level: 2, name: /The rich section/ }),
    ).toBeInTheDocument();
    expect(screen.getByText('The lead paragraph.')).toBeInTheDocument();
    expect(screen.getByText('02')).toBeInTheDocument();

    // Text block: `\n\n` splits into two separate paragraph elements.
    const first = screen.getByText('First paragraph.');
    const second = screen.getByText('Second paragraph.');
    expect(first.tagName).toBe('P');
    expect(second.tagName).toBe('P');
    expect(first).not.toBe(second);

    // Code block starts collapsed (label visible, source hidden), then expands
    // to render the code verbatim with caption, source path, and copy button.
    // The toggle is the section's only expandable control (the copy button has
    // no aria-expanded), which disambiguates it from any other button.
    const codeToggle = screen.getByRole('button', { name: /go/, expanded: false });
    expect(codeToggle).toHaveAttribute('aria-expanded', 'false');
    fireEvent.click(codeToggle);
    expect(codeToggle).toHaveAttribute('aria-expanded', 'true');
    expect(
      screen.getByText(
        (_, element) =>
          element?.tagName === 'CODE' &&
          element.textContent === 'type Hello struct {\n    Name string\n}',
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('the caption')).toBeInTheDocument();
    expect(screen.getByText('source: server/internal/domain/demos/hello.go')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'copy' })).toBeInTheDocument();

    // Demo block resolves through DemoHost — an unregistered demo id degrades
    // to the friendly not-wired-yet card inside its titled panel.
    expect(
      await screen.findByRole('heading', { level: 3, name: 'The goroutine lab' }),
    ).toBeInTheDocument();
    expect(screen.getByTestId('demo-not-wired')).toHaveTextContent(
      'demoId: nope:not-registered',
    );

    // List blocks: `ordered` picks the semantic element — the first section's
    // ordered list renders a numbered <ol>, the terminal section's unordered
    // list a bulleted <ul>; items are real <li> text (listitem roles).
    const lists = screen.getAllByRole('list');
    expect(lists).toHaveLength(2);
    expect(lists[0]?.tagName).toBe('OL');
    expect(lists[1]?.tagName).toBe('UL');
    const listItems = screen.getAllByRole('listitem');
    expect(listItems).toHaveLength(6);
    expect(listItems[0]).toHaveTextContent('Init — go mod init the module.');
    expect(listItems[5]).toHaveTextContent('less — page long responses.');
    expect(listItems[0]?.tagName).toBe('LI');
  });

  it('surfaces the 404 envelope with the server requestId for an unknown topic id', async () => {
    stubApiFetch([
      {
        url: '/api/content/missing',
        status: 404,
        body: {
          error: {
            code: 'not_found',
            message: "Topic 'missing' not found",
            requestId: 'req-9',
          },
        },
      },
    ]);

    renderTopic('missing');

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Could not load this topic');
    expect(alert).toHaveTextContent("Topic 'missing' not found");
    expect(alert).toHaveTextContent('not_found · status 404 · request req-9');
  });

  it('shows the error panel when the network is down (status 0)', async () => {
    stubNetworkFailure();

    renderTopic('anything');

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Could not load this topic');
    expect(alert).toHaveTextContent('Could not reach the server.');
    expect(alert).toHaveTextContent('network_error · status 0');
  });

  it('is race-safe on route param changes: a slow stale fetch never overwrites the new topic', async () => {
    const topicA = { ...richTopic, id: 'topic-a', title: 'Slow Topic A' };
    const topicB = { ...richTopic, id: 'topic-b', title: 'Fast Topic B' };
    stubApiFetch([
      { url: '/api/content/topic-a', body: topicA, delayMs: 80 },
      { url: '/api/content/topic-b', body: topicB },
    ]);

    const router = createMemoryRouter(
      [{ path: '/topics/:topicId', element: <TopicView /> }],
      { initialEntries: ['/topics/topic-a'] },
    );
    render(<RouterProvider router={router} />);
    expect(screen.getByTestId('topic-loading')).toBeInTheDocument();

    // Navigate to topic B before A's slow response lands.
    await act(async () => {
      await router.navigate('/topics/topic-b');
    });

    // B wins; the loading state reset for the new param.
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Fast Topic B' }),
    ).toBeInTheDocument();

    // A's late response is dropped — it never overwrites B's screen.
    await new Promise((resolve) => setTimeout(resolve, 150));
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Fast Topic B' })).toBeInTheDocument(),
    );
    expect(screen.queryByText('Slow Topic A')).not.toBeInTheDocument();
  });
});
