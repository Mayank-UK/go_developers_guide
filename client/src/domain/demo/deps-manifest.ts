/**
 * Domain models for the dependency-management topic's manifest demo
 * (demo block id `deps:manifest`) — the client-side shape of
 * `GET /api/demo/deps/manifest`. Field names match the server's
 * camelCase wire contract 1:1 (the DTO is the seam where wire and
 * domain may diverge later).
 *
 * One payload puts the module's two truths side by side, both derived
 * by the server at runtime and never hardcoded: the parsed live go.mod
 * (module, go directive, direct requires with their MVS-pinned
 * versions, the indirect count, the tool directives) and the serving
 * binary's own build info (the modules it actually links — a strict
 * subset of the manifest that excludes test-only requires). Around
 * that core: the tools/deps.go blank-import anchors, the Docker cache
 * volumes, and the govulncheck panel — live when the tool ran for
 * real, a clearly-labeled example when it could not.
 */

/** One require as the panel shows it: path, pinned version, linked. */
export interface DepsRequireRow {
  path: string;
  version: string;
  linked: boolean;
}

/** The manifest panel: go.mod parsed live beside the process. */
export interface DepsGoModPanel {
  available: boolean;
  file: string;
  module: string;
  goDirective: string;
  directRequires: DepsRequireRow[];
  indirectCount: number;
  toolDirectives: string[];
  note: string;
}

/** The serving binary's birth certificate, per debug.ReadBuildInfo. */
export interface DepsBuildPanel {
  available: boolean;
  modulePath: string;
  mainVersion: string;
  goVersion: string;
  linkedCount: number;
  note: string;
}

/** The tools/deps.go blank-import anchor story. */
export interface DepsAnchorPanel {
  available: boolean;
  file: string;
  buildTag: string;
  anchors: DepsRequireRow[];
  note: string;
}

/** One named Docker cache volume the dev compose stack mounts. */
export interface DepsCacheVolumeRow {
  name: string;
  mount: string;
  whatItHolds: string;
  whyWarm: string;
}

/** The Docker cache story: volumes, layer order, external pins. */
export interface DepsCachePanel {
  volumes: DepsCacheVolumeRow[];
  layerOrderNote: string;
  externalNote: string;
}

/** The govulncheck verdict: live, running, or a labeled example. */
export interface DepsVulncheckPanel {
  mode: 'running' | 'live' | 'fixtures';
  command: string;
  exitCode: number;
  elapsedMs: number;
  summary?: string;
  fixture?: string;
  note: string;
}

/** The whole deps manifest payload. */
export interface DepsManifestDemoData {
  goMod: DepsGoModPanel;
  build: DepsBuildPanel;
  anchors: DepsAnchorPanel;
  cache: DepsCachePanel;
  vulncheck: DepsVulncheckPanel;
  reasoning: string;
}
