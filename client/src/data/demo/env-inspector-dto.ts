import type {
  EnvInspectorDemoData,
  EnvInspectorFlags,
  EnvInspectorScoreRow,
  EnvInspectorVariable,
} from '../../domain/demo/env-inspector';
import {
  isRecord,
  requiredArray,
  requiredBoolean,
  requiredRecord,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * EnvInspectorDTO — the wire/transport shape of the env inspector
 * payload. Mirrors the domain model 1:1; the `fromJson` builds ONLY
 * the listed fields (whitelist) and throws on any shape violation,
 * which parseDto upgrades to an InvalidResponseException.
 */

export type EnvInspectorDemoDto = EnvInspectorDemoData;

function variableFromJson(value: unknown): EnvInspectorVariable {
  if (!isRecord(value)) throw new Error('variable row must be an object');
  return {
    field: requiredString(value, 'field'),
    envVar: requiredString(value, 'envVar'),
    value: requiredString(value, 'value'),
    source: requiredString(value, 'source'),
    sensitive: requiredBoolean(value, 'sensitive'),
    validatedBy: requiredString(value, 'validatedBy'),
  };
}

function scoreRowFromJson(value: unknown): EnvInspectorScoreRow {
  if (!isRecord(value)) throw new Error('scorecard row must be an object');
  return {
    factor: requiredString(value, 'factor'),
    verdict: requiredString(value, 'verdict'),
    how: requiredString(value, 'how'),
  };
}

function flagsFromJson(body: Record<string, unknown>): EnvInspectorFlags {
  const flags = requiredRecord(body, 'flags');
  return {
    dataMode: requiredString(flags, 'dataMode'),
    injectFailure: requiredString(flags, 'injectFailure'),
  };
}

/** Deserialize + validate the GET /api/demo/env/inspector payload. */
export function envInspectorDtoFromJson(body: unknown): EnvInspectorDemoDto {
  if (!isRecord(body)) throw new Error('env inspector payload must be an object');
  return {
    mode: requiredText(body, 'mode'),
    env: requiredString(body, 'env'),
    variables: requiredArray(body, 'variables').map(variableFromJson),
    twelveFactor: requiredArray(body, 'twelveFactor').map(scoreRowFromJson),
    flags: flagsFromJson(body),
    redaction: requiredText(body, 'redaction'),
    reasoning: requiredText(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toEnvInspectorDemo(dto: EnvInspectorDemoDto): EnvInspectorDemoData {
  return dto;
}
