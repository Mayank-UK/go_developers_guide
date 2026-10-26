import type {
  AppInfoDemoData,
  AppInfoEnvironment,
  AppInfoModule,
  AppInfoRuntime,
  BuildSettingRow,
} from '../../domain/demo/app-info';
import {
  isRecord,
  requiredArray,
  requiredNumber,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * AppInfoDTO — the wire/transport shape of the go-http-fundamentals
 * topic's app-info payload. Mirrors the domain model 1:1 (the interface is
 * the seam where wire and domain may diverge); the `fromJson` builds ONLY
 * the listed fields (whitelist) and throws on any shape violation, which
 * parseDto upgrades to an InvalidResponseException.
 *
 * Two fields are deliberately `requiredText` (present-but-empty passes)
 * rather than `requiredString`: a binary built without build info reports
 * an empty `version`/`goVersion`, and an unset build flag's `value` is an
 * empty string — empty is DATA there, not absence. `buildSettings` may be
 * an EMPTY array (the server marshals [] never null), but it must be an
 * array.
 */

export type AppInfoDemoDto = AppInfoDemoData;

/** Deserialize + validate the GET /api/demo/gohttp/app-info payload. */
export function appInfoDtoFromJson(body: unknown): AppInfoDemoDto {
  if (!isRecord(body)) throw new Error('app-info payload must be an object');
  return {
    module: moduleFromJson(body.module),
    runtime: runtimeFromJson(body.runtime),
    environment: environmentFromJson(body.environment),
    buildSettings: requiredArray(body, 'buildSettings').map(buildSettingFromJson),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toAppInfoDemo(dto: AppInfoDemoDto): AppInfoDemoData {
  return dto;
}

/* ---- nested payload validators (throw on shape violation) ---- */

function moduleFromJson(body: unknown): AppInfoModule {
  if (!isRecord(body)) throw new Error('module facts must be an object');
  return {
    path: requiredString(body, 'path'),
    version: requiredText(body, 'version'),
    goVersion: requiredText(body, 'goVersion'),
    dependencyCount: requiredNumber(body, 'dependencyCount'),
  };
}

function runtimeFromJson(body: unknown): AppInfoRuntime {
  if (!isRecord(body)) throw new Error('runtime facts must be an object');
  return {
    numCPU: requiredNumber(body, 'numCPU'),
    goMaxProcs: requiredNumber(body, 'goMaxProcs'),
  };
}

function environmentFromJson(body: unknown): AppInfoEnvironment {
  if (!isRecord(body)) throw new Error('environment facts must be an object');
  return {
    port: requiredNumber(body, 'port'),
    env: requiredString(body, 'env'),
    logLevel: requiredString(body, 'logLevel'),
  };
}

function buildSettingFromJson(body: unknown): BuildSettingRow {
  if (!isRecord(body)) throw new Error('build setting row must be an object');
  return {
    key: requiredString(body, 'key'),
    value: requiredText(body, 'value'),
  };
}
