/**
 * Domain models for the go-http-fundamentals topic's app-info demo — the
 * client-side shape of `GET /api/demo/gohttp/app-info`. Field names match
 * the server's camelCase wire contract 1:1 (the DTO is the seam where wire
 * and domain may diverge later).
 */

export interface AppInfoModule {
  /** The module path from go.mod, e.g. `guide`. */
  path: string;
  /** `(devel)` until a VCS-stamped release build says otherwise. */
  version: string;
  /** The toolchain that built the binary, e.g. `go1.27.3`. */
  goVersion: string;
  /** Module dependencies the binary carries — 0 for this stdlib-only guide. */
  dependencyCount: number;
}

export interface AppInfoRuntime {
  /** Logical CPUs the process can see (`runtime.NumCPU()`). */
  numCPU: number;
  /** The GOMAXPROCS the scheduler runs with (`runtime.GOMAXPROCS(0)`). */
  goMaxProcs: number;
}

export interface AppInfoEnvironment {
  /** The TCP port the server listens on (GDG_PORT). */
  port: number;
  /** The environment name, e.g. `dev` or `prod` (GDG_ENV). */
  env: string;
  /** The log level the process booted with, e.g. `INFO` (GDG_LOG_LEVEL). */
  logLevel: string;
}

export interface BuildSettingRow {
  /** The toolchain flag, e.g. `-buildmode`, `CGO_ENABLED`, `GOARCH`. */
  key: string;
  /** The stamped value; an unset flag's empty string is data, not absence. */
  value: string;
}

export interface AppInfoDemoData {
  module: AppInfoModule;
  runtime: AppInfoRuntime;
  environment: AppInfoEnvironment;
  buildSettings: BuildSettingRow[];
}
