import { DataMode, InjectFailure, type ScenarioPatch, type ScenarioState } from '../../domain/scenario/scenario';
import { isRecord } from '../content/validate';

/**
 * ScenarioDTO — the wire shape of GET/PATCH /api/demo/scenario, responsible
 * for (de)serialization and mapping to the domain model (express-guide
 * `PostDTO` style). The bare `{dataMode, injectFailure}` object is the whole
 * contract (no data wrapper); the parser throws on any shape violation —
 * an unknown or missing enum value is hostile input, not a default to guess —
 * which parseDto upgrades to an InvalidResponseException.
 */

/** The validated wire object both endpoints answer with. */
export interface ScenarioDto {
  dataMode: DataMode;
  injectFailure: InjectFailure;
}

/** Deserialize + validate a scenario snapshot (input is hostile). */
export function scenarioDtoFromJson(body: unknown): ScenarioDto {
  if (!isRecord(body)) throw new Error('scenario snapshot must be an object');
  return {
    dataMode: parseDataMode(body['dataMode']),
    injectFailure: parseInjectFailure(body['injectFailure']),
  };
}

/**
 * Serialize a patch body. The server reads a PARTIAL document — absent keys
 * mean unchanged — so only the keys the caller actually set are emitted; a
 * patch of `{injectFailure}` must never send a `dataMode` of its own.
 */
export function serializeScenarioPatch(patch: ScenarioPatch): string {
  const body: { dataMode?: DataMode; injectFailure?: InjectFailure } = {};
  if (patch.dataMode !== undefined) body.dataMode = patch.dataMode;
  if (patch.injectFailure !== undefined) body.injectFailure = patch.injectFailure;
  return JSON.stringify(body);
}

/** Convert a transport DTO into the domain model. */
export function toScenarioState(dto: ScenarioDto): ScenarioState {
  return { dataMode: dto.dataMode, injectFailure: dto.injectFailure };
}

function parseDataMode(value: unknown): DataMode {
  if (value !== DataMode.Fake && value !== DataMode.Live) {
    throw new Error(`unknown data mode: ${String(value)}`);
  }
  return value;
}

function parseInjectFailure(value: unknown): InjectFailure {
  if (
    value !== InjectFailure.None &&
    value !== InjectFailure.Error &&
    value !== InjectFailure.Latency
  ) {
    throw new Error(`unknown failure injection: ${String(value)}`);
  }
  return value;
}
