import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { DepsManifestDemo } from './manifest_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetDepsManifestDemoUseCase,
  DEPS_MANIFEST_DEMO_USE_CASE,
} from '../../../domain/demo/get-deps-manifest-demo-usecase';
import type { DepsManifestDemoData } from '../../../domain/demo/deps-manifest';

/**
 * DepsManifestDemo spec — fixture-driven (happy + failure): the
 * component resolves its use case through the container token (the
 * documented register-a-singleton seam), so the spec swaps that
 * registration instead of stubbing fetch. Behavior, not
 * implementation: the live manifest renders with its direct requires
 * and linked flags, the build panel names which binary it describes,
 * the anchors, cache volumes, and the live vulncheck verdict render,
 * and failures degrade to the shared error panel with recovery.
 */

const fixture: DepsManifestDemoData = {
  goMod: {
    available: true,
    file: 'server/go.mod',
    module: 'guide',
    goDirective: '1.27.3',
    directRequires: [
      { path: 'github.com/jackc/pgx/v5', version: 'v5.11.0', linked: true },
      { path: 'github.com/testcontainers/testcontainers-go', version: 'v0.44.0', linked: false },
    ],
    indirectCount: 160,
    toolDirectives: ['github.com/99designs/gqlgen'],
    note: 'parsed live at request time',
  },
  build: {
    available: true,
    modulePath: 'guide',
    mainVersion: '(devel)',
    goVersion: 'go1.27.3',
    linkedCount: 38,
    note: 'read at runtime from runtime/debug.ReadBuildInfo of the process serving this response',
  },
  anchors: {
    available: true,
    file: 'server/tools/deps.go',
    buildTag: 'tools',
    anchors: [{ path: 'github.com/coder/websocket', version: 'v1.8.15', linked: true }],
    note: 'blank imports behind the tools build tag keep tidy honest',
  },
  cache: {
    volumes: [
      {
        name: 'gdg-go',
        mount: '/go/pkg/mod',
        whatItHolds: 'the module cache',
        whyWarm: 'no re-downloads',
      },
      {
        name: 'gdg-gocache',
        mount: '/root/.cache/go-build',
        whatItHolds: 'the build cache',
        whyWarm: 'unchanged packages are cache hits',
      },
    ],
    layerOrderNote: 'manifests first, source last',
    externalNote: 'external: true keeps one warm cache',
  },
  vulncheck: {
    mode: 'live',
    command: 'govulncheck ./...',
    exitCode: 0,
    elapsedMs: 3281,
    summary: 'No vulnerabilities found.',
    note: 'the real tool ran just now',
  },
  reasoning: 'one module, two honest views',
};

function stubUseCase(execute: () => Promise<Result<DepsManifestDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    DEPS_MANIFEST_DEMO_USE_CASE,
    { execute } as unknown as GetDepsManifestDemoUseCase,
  );
}

describe('DepsManifestDemo', () => {
  it('renders loading first, then the manifest, build, anchors, caches, and verdict', async () => {
    stubUseCase(async () => success(fixture));

    render(<DepsManifestDemo />);

    expect(screen.getByTestId('deps-manifest-loading')).toBeInTheDocument();

    await waitForReady();
    const goMod = screen.getByTestId('deps-manifest-gomod');
    expect(goMod).toHaveTextContent('module guide');
    expect(goMod).toHaveTextContent('2 direct requires');
    expect(goMod).toHaveTextContent('github.com/jackc/pgx/v5 v5.11.0');
    expect(goMod).toHaveTextContent('linked in this binary');
    expect(goMod).toHaveTextContent('not linked in this binary');
    expect(goMod).toHaveTextContent('160 indirect');

    const build = screen.getByTestId('deps-manifest-build');
    expect(build).toHaveTextContent('(devel)');
    expect(build).toHaveTextContent('links 38 modules');
    expect(build).toHaveTextContent('process serving this response');

    const anchors = screen.getByTestId('deps-manifest-anchors');
    expect(anchors).toHaveTextContent('server/tools/deps.go');
    expect(anchors).toHaveTextContent('github.com/coder/websocket v1.8.15');

    const cache = screen.getByTestId('deps-manifest-cache');
    expect(cache).toHaveTextContent('gdg-go → /go/pkg/mod');
    expect(cache).toHaveTextContent('manifests first');

    const vulncheck = screen.getByTestId('deps-manifest-vulncheck');
    expect(vulncheck).toHaveTextContent('govulncheck ./... — mode live');
    expect(vulncheck).toHaveTextContent('exit 0 in 3281 ms');
    expect(vulncheck).toHaveTextContent('No vulnerabilities found.');

    expect(screen.getByTestId('deps-manifest-reasoning')).toHaveTextContent('two honest views');
  });

  it('renders the honest degradation for an unavailable manifest and tool', async () => {
    const degraded: DepsManifestDemoData = {
      ...fixture,
      goMod: {
        ...fixture.goMod,
        available: false,
        directRequires: [],
        toolDirectives: [],
        note: 'the source tree is not available beside this process',
      },
      anchors: { ...fixture.anchors, available: false, anchors: [], note: 'not available' },
      vulncheck: {
        mode: 'fixtures',
        command: 'govulncheck ./...',
        exitCode: 0,
        elapsedMs: 0,
        fixture: 'Vulnerability #1: GO-2021-0113\n…',
        note: 'an EXAMPLE, clearly not a scan of this repo',
      },
    };
    stubUseCase(async () => success(degraded));

    render(<DepsManifestDemo />);

    await waitForReady();
    expect(screen.getByTestId('deps-manifest-gomod')).toHaveTextContent(
      'not available beside this process',
    );
    expect(screen.getByTestId('deps-manifest-vulncheck')).toHaveTextContent('mode fixtures');
    expect(screen.getByTestId('deps-manifest-vulncheck')).toHaveTextContent('GO-2021-0113');
    expect(screen.getByTestId('deps-manifest-vulncheck')).toHaveTextContent(
      'not a scan of this repo',
    );
  });

  it('degrades to the error panel on a transport failure, with recovery', async () => {
    const failure = new ServerFailure('The deps manifest could not be reached.', {
      code: 'network_error',
      status: 0,
      requestId: 'rid-deps-down',
    });
    stubUseCase(async () => failed(failure));

    render(<DepsManifestDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('network_error');

    // Retry re-runs the load through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitForReady();
  });
});

function waitForReady() {
  return waitFor(() => expect(screen.getByTestId('deps-manifest-ready')).toBeInTheDocument());
}
