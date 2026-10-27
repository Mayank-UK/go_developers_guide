import { describe, expect, it } from 'vitest';
import { lazy } from 'react';
import { render, screen } from '@testing-library/react';
import { DemoHost } from './demo-host';
import { demoRegistry } from '../../demos/demo-registry';

/**
 * DemoHost spec — the known-demo path is proven with a fixture registry (the
 * same `DemoRegistry` contract the live one grows through), and the unknown
 * path with the live one; the count pin below tracks the landed demos.
 */

const fixtureRegistry = {
  'go:app-info': lazy(() =>
    Promise.resolve({
      default: function FixtureDemo() {
        return <p data-testid="fixture-demo-body">the fixture demo body</p>;
      },
    }),
  ),
};

describe('DemoHost', () => {
  it('renders the panel title and description around the registered demo body', async () => {
    render(
      <DemoHost
        demoId="go:app-info"
        title="App info"
        description="Inspect the running server build."
        registry={fixtureRegistry}
      />,
    );

    expect(
      screen.getByRole('heading', { level: 3, name: 'App info' }),
    ).toBeInTheDocument();
    expect(screen.getByText('Inspect the running server build.')).toBeInTheDocument();
    expect(await screen.findByTestId('fixture-demo-body')).toHaveTextContent(
      'the fixture demo body',
    );
  });

  it('renders the friendly not-wired-yet card for a demo id missing from the registry', () => {
    render(<DemoHost demoId="nope:not-built-yet" title="A demo that does not exist" />);

    const card = screen.getByTestId('demo-not-wired');
    expect(card).toBeInTheDocument();
    expect(card).toHaveTextContent('demo not wired yet');
    expect(card).toHaveTextContent('demoId: nope:not-built-yet');
    // It is a note, not an error.
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('the live registry holds exactly the demos that have landed', () => {
    // Grows topic by topic (Phase 3+: one lazy entry per demo block id);
    // unknown ids still degrade to the friendly card above. Landed so far:
    // Phase 3 fundamentals (runtime-types, concurrency, headers, app-info,
    // matrix, context, race), the Phase 4 request-handling set (routing
    // explorer, middleware pipeline, reqres inspector + methods, validation
    // arena), the full Phase 5 apis set (posts workbench, webhook
    // simulator, docs link, graphql playground, grpc console), the Phase 6
    // data demos (schema explorer, uuid lab, the sql sextet, the orm probe,
    // mongo console, redis playground), the Phase 7 state demos (the
    // caching lab, the messaging stream + outbox pair, and the
    // authentication topic's seven story demos), and the Phase 8-9 sets
    // (files workbench, i18n negotiator, realtime pair, jobs console,
    // errors explorer, deps manifest, arch tracer, the slog viewer,
    // and the trace round-trip).
    expect(Object.keys(demoRegistry)).toEqual([
      'go:runtime-types',
      'runtime:concurrency',
      'http:headers',
      'gohttp:app-info',
      'iface:matrix',
      'conc:context',
      'conc:race',
      'routing:explorer',
      'middleware:pipeline',
      'reqres:inspector',
      'reqres:methods',
      'validation:arena',
      'api:posts-workbench',
      'api:webhook-simulator',
      'api:docs-link',
      'api:graphql-playground',
      'api:grpc-console',
      'db:schema-explorer',
      'db:uuid',
      'db:sql-safety',
      'db:migrations',
      'db:sql-lab',
      'db:search',
      'db:race',
      'db:audit',
      'db:orm-probe',
      'db:mongo-console',
      'redis:playground',
      'caching:lab',
      'msg:stream',
      'msg:outbox',
      'auth:sessions',
      'auth:api-keys',
      'auth:compare',
      'auth:jwt',
      'auth:oauth',
      'auth:hashing',
      'auth:refresh',
      'authz:matrix',
      'security:lab',
      'security:pii',
      'files:workbench',
      'i18n:negotiator',
      'realtime:ticker',
      'realtime:chat',
      'jobs:console',
      'errors:explorer',
      'deps:manifest',
      'arch:tracer',
      'obs:viewer',
      'obs:trace',
    ]);
  });
});
