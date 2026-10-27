import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { DepsManifestApi } from './deps-manifest-api';
import { DepsManifestRepositoryImpl } from './deps-manifest-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * DepsManifestRepositoryImpl spec — fixture-driven (happy + failure):
 * the manifest payload (go.mod panel, build info, anchors, caches,
 * vulncheck verdict) round-trips; malformed payloads and transport
 * failures resolve as Failed Results, never throws.
 */

const demoPayload = {
  goMod: {
    available: true,
    file: 'server/go.mod',
    module: 'guide',
    goDirective: '1.27.3',
    directRequires: [
      { path: 'github.com/jackc/pgx/v5', version: 'v5.11.0', linked: true },
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
    note: 'read from the process serving this response',
  },
  anchors: {
    available: true,
    file: 'server/tools/deps.go',
    buildTag: 'tools',
    anchors: [{ path: 'github.com/jackc/pgx/v5', version: 'v5.11.0', linked: true }],
    note: 'blank imports behind the tools build tag',
  },
  cache: {
    volumes: [
      { name: 'gdg-go', mount: '/go/pkg/mod', whatItHolds: 'module cache', whyWarm: 'warm' },
      { name: 'gdg-gocache', mount: '/root/.cache/go-build', whatItHolds: 'build cache', whyWarm: 'warm' },
    ],
    layerOrderNote: 'manifests first',
    externalNote: 'external: true',
  },
  vulncheck: {
    mode: 'running',
    command: 'govulncheck ./...',
    exitCode: 0,
    elapsedMs: 0,
    note: 'the scan is running in the background right now',
  },
  reasoning: 'one module, two honest views',
};

function makeRepository(): DepsManifestRepositoryImpl {
  return new DepsManifestRepositoryImpl(new DepsManifestApi(createAppConfig()));
}

describe('DepsManifestRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/deps/manifest', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.goMod.module).toBe('guide');
      expect(result.value.goMod.directRequires[0]?.version).toBe('v5.11.0');
      expect(result.value.build.linkedCount).toBe(38);
      expect(result.value.vulncheck.mode).toBe('running');
      expect(result.value.reasoning).toContain('two honest views');
    }
  });

  it('rejects a payload whose goMod panel is missing', async () => {
    stubApiFetch([{ url: '/api/demo/deps/manifest', body: { ...demoPayload, goMod: undefined } }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
    }
  });

  it('guards a network failure into a Failed Result (network_error, status 0)', async () => {
    stubNetworkFailure();

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
      expect(result.failure.status).toBe(0);
    }
  });
});
