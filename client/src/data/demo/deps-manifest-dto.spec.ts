import { describe, expect, it } from 'vitest';
import { depsManifestDtoFromJson } from './deps-manifest-dto';
import type { DepsManifestDemoData } from '../../domain/demo/deps-manifest';

/**
 * DepsManifest DTO spec — fixture-driven: the full payload
 * round-trips into the domain shape, and the closed vulncheck
 * vocabulary plus every required field is enforced: a violation
 * throws, which parseDto upgrades to an InvalidResponseException
 * upstream.
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
    note: 'read from the process serving this response',
  },
  anchors: {
    available: true,
    file: 'server/tools/deps.go',
    buildTag: 'tools',
    anchors: [
      { path: 'github.com/jackc/pgx/v5', version: 'v5.11.0', linked: true },
      { path: 'golang.org/x/crypto/argon2', version: 'v0.58.0', linked: false },
    ],
    note: 'blank imports behind the tools build tag',
  },
  cache: {
    volumes: [
      {
        name: 'gdg-go',
        mount: '/go/pkg/mod',
        whatItHolds: 'the module cache',
        whyWarm: 'no re-downloads for required modules',
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

describe('depsManifestDtoFromJson', () => {
  it('round-trips the full payload into the domain shape', () => {
    const parsed = depsManifestDtoFromJson(fixture);

    expect(parsed.goMod.module).toBe('guide');
    expect(parsed.goMod.goDirective).toBe('1.27.3');
    expect(parsed.goMod.directRequires).toHaveLength(2);
    expect(parsed.goMod.directRequires[1]?.linked).toBe(false);
    expect(parsed.goMod.indirectCount).toBe(160);
    expect(parsed.build.linkedCount).toBe(38);
    expect(parsed.anchors.anchors[1]?.path).toBe('golang.org/x/crypto/argon2');
    expect(parsed.cache.volumes[0]?.mount).toBe('/go/pkg/mod');
    expect(parsed.vulncheck.summary).toBe('No vulnerabilities found.');
    expect(parsed.reasoning).toBe('one module, two honest views');
  });

  it('accepts a fixtures-mode payload without summary or fixture text', () => {
    const degraded = {
      ...fixture,
      goMod: { ...fixture.goMod, available: false, directRequires: [], toolDirectives: [] },
      vulncheck: {
        mode: 'fixtures',
        command: 'govulncheck ./...',
        exitCode: 0,
        elapsedMs: 0,
        fixture: 'Vulnerability #1: GO-2021-0113\n…',
        note: 'the tool could not run here',
      },
    };

    const parsed = depsManifestDtoFromJson(degraded);
    expect(parsed.goMod.available).toBe(false);
    expect(parsed.goMod.directRequires).toEqual([]);
    expect(parsed.vulncheck.mode).toBe('fixtures');
    expect(parsed.vulncheck.summary).toBeUndefined();
    expect(parsed.vulncheck.fixture).toContain('GO-2021-0113');
  });

  it('accepts the degraded manifest: empty module/goDirective is data, not absence', () => {
    const degraded = {
      ...fixture,
      goMod: {
        ...fixture.goMod,
        available: false,
        module: '',
        goDirective: '',
        directRequires: [],
        toolDirectives: [],
      },
    };

    const parsed = depsManifestDtoFromJson(degraded);
    expect(parsed.goMod.available).toBe(false);
    expect(parsed.goMod.module).toBe('');
    expect(parsed.goMod.goDirective).toBe('');
    expect(parsed.goMod.directRequires).toEqual([]);
  });

  it('rejects an unknown vulncheck mode', () => {
    const poisoned = {
      ...fixture,
      vulncheck: { ...fixture.vulncheck, mode: 'maybe' },
    };

    expect(() => depsManifestDtoFromJson(poisoned)).toThrow(/running\|live\|fixtures/);
  });

  it('rejects a require row whose version is missing', () => {
    const poisoned = {
      ...fixture,
      goMod: {
        ...fixture.goMod,
        directRequires: [{ ...fixture.goMod.directRequires[0]!, version: '' }],
      },
    };

    expect(() => depsManifestDtoFromJson(poisoned)).toThrow('version');
  });

  it('rejects a payload whose build panel is missing', () => {
    const poisoned = { ...fixture, build: undefined };

    expect(() => depsManifestDtoFromJson(poisoned)).toThrow();
  });

  it('rejects a non-object payload', () => {
    expect(() => depsManifestDtoFromJson(null)).toThrow();
    expect(() => depsManifestDtoFromJson('nope')).toThrow();
  });
});
