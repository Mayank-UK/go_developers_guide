import type {
  DepsAnchorPanel,
  DepsBuildPanel,
  DepsCachePanel,
  DepsCacheVolumeRow,
  DepsGoModPanel,
  DepsManifestDemoData,
  DepsRequireRow,
  DepsVulncheckPanel,
} from '../../domain/demo/deps-manifest';
import {
  isRecord,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredRecord,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * DepsManifestDTO — the wire/transport shape of the manifest payload.
 * Mirrors the domain model 1:1; the `fromJson` builds ONLY the listed
 * fields (whitelist) and throws on any shape violation, which parseDto
 * upgrades to an InvalidResponseException. The vulncheck mode validates
 * against the server's closed vocabulary (running|live|fixtures) and
 * the optional summary/fixture fields accept undefined but never an
 * empty string — every version, count, and verdict stays
 * server-derived, never pinned to literals here.
 */

export type DepsManifestDemoDto = DepsManifestDemoData;

const VULNCHECK_MODES = new Set(['running', 'live', 'fixtures']);

function optionalString(source: Record<string, unknown>, key: string): string | undefined {
  if (source[key] === undefined) return undefined;
  const value = source[key];
  if (typeof value !== 'string' || value === '') {
    throw new Error(`expected a non-empty string at "${key}"`);
  }
  return value;
}

function requireRowFromJson(value: unknown): DepsRequireRow {
  if (!isRecord(value)) throw new Error('deps require row must be an object');
  return {
    path: requiredString(value, 'path'),
    version: requiredString(value, 'version'),
    linked: requiredBoolean(value, 'linked'),
  };
}

function stringList(source: Record<string, unknown>, key: string): string[] {
  return requiredArray(source, key).map((value) => {
    if (typeof value !== 'string' || value === '') {
      throw new Error(`expected a non-empty string at "${key}"`);
    }
    return value;
  });
}

function goModFromJson(body: Record<string, unknown>): DepsGoModPanel {
  const goMod = requiredRecord(body, 'goMod');
  return {
    available: requiredBoolean(goMod, 'available'),
    file: requiredString(goMod, 'file'),
    // Present-but-empty is real data: an unavailable source tree parses
    // to empty module/go directives (requiredText, not requiredString).
    module: requiredText(goMod, 'module'),
    goDirective: requiredText(goMod, 'goDirective'),
    directRequires: requiredArray(goMod, 'directRequires').map(requireRowFromJson),
    indirectCount: requiredNumber(goMod, 'indirectCount'),
    toolDirectives: stringList(goMod, 'toolDirectives'),
    note: requiredString(goMod, 'note'),
  };
}

function buildFromJson(body: Record<string, unknown>): DepsBuildPanel {
  const build = requiredRecord(body, 'build');
  return {
    available: requiredBoolean(build, 'available'),
    // Same empty-is-data rule: a process without stamped build info
    // reports empty strings honestly.
    modulePath: requiredText(build, 'modulePath'),
    mainVersion: requiredText(build, 'mainVersion'),
    goVersion: requiredText(build, 'goVersion'),
    linkedCount: requiredNumber(build, 'linkedCount'),
    note: requiredString(build, 'note'),
  };
}

function anchorsFromJson(body: Record<string, unknown>): DepsAnchorPanel {
  const anchors = requiredRecord(body, 'anchors');
  return {
    available: requiredBoolean(anchors, 'available'),
    file: requiredString(anchors, 'file'),
    buildTag: requiredString(anchors, 'buildTag'),
    anchors: requiredArray(anchors, 'anchors').map(requireRowFromJson),
    note: requiredString(anchors, 'note'),
  };
}

function cacheFromJson(body: Record<string, unknown>): DepsCachePanel {
  const cache = requiredRecord(body, 'cache');
  const volumes = requiredArray(cache, 'volumes').map((value) => {
    if (!isRecord(value)) throw new Error('deps cache volume row must be an object');
    const row: DepsCacheVolumeRow = {
      name: requiredString(value, 'name'),
      mount: requiredString(value, 'mount'),
      whatItHolds: requiredString(value, 'whatItHolds'),
      whyWarm: requiredString(value, 'whyWarm'),
    };
    return row;
  });
  return {
    volumes,
    layerOrderNote: requiredString(cache, 'layerOrderNote'),
    externalNote: requiredString(cache, 'externalNote'),
  };
}

function vulncheckFromJson(body: Record<string, unknown>): DepsVulncheckPanel {
  const vulncheck = requiredRecord(body, 'vulncheck');
  const mode = requiredString(vulncheck, 'mode');
  if (!VULNCHECK_MODES.has(mode)) {
    throw new Error(`unknown deps vulncheck mode "${mode}" (expected running|live|fixtures)`);
  }
  return {
    mode: mode as DepsVulncheckPanel['mode'],
    command: requiredString(vulncheck, 'command'),
    exitCode: requiredNumber(vulncheck, 'exitCode'),
    elapsedMs: requiredNumber(vulncheck, 'elapsedMs'),
    summary: optionalString(vulncheck, 'summary'),
    fixture: optionalString(vulncheck, 'fixture'),
    note: requiredString(vulncheck, 'note'),
  };
}

/** Deserialize + validate the GET /api/demo/deps/manifest payload. */
export function depsManifestDtoFromJson(body: unknown): DepsManifestDemoDto {
  if (!isRecord(body)) throw new Error('deps manifest payload must be an object');
  return {
    goMod: goModFromJson(body),
    build: buildFromJson(body),
    anchors: anchorsFromJson(body),
    cache: cacheFromJson(body),
    vulncheck: vulncheckFromJson(body),
    reasoning: requiredString(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toDepsManifestDemo(dto: DepsManifestDemoDto): DepsManifestDemoData {
  return dto;
}
