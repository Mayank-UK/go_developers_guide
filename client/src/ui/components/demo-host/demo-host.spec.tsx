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
    // unknown ids still degrade to the friendly card above. First landed:
    // the go-language topic's runtime-types demo.
    expect(Object.keys(demoRegistry)).toEqual(['go:runtime-types']);
  });
});
