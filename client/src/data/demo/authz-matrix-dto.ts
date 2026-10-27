import type {
  AuthzMatrixBeat,
  AuthzMatrixDemoData,
  AuthzMatrixFact,
} from '../../domain/demo/authz-matrix';
import { isRecord, requiredArray, requiredString } from '../content/validate';

/**
 * AuthzMatrixDTO — the wire/transport shape of the authorization matrix
 * payload. Mirrors the domain model 1:1; the `fromJson` builds ONLY the
 * listed fields (whitelist) and throws on any shape violation, which
 * parseDto upgrades to an InvalidResponseException. Every beat sentence
 * and cell citation is minted server-side per run — validated as
 * non-empty strings, never pinned to literals.
 */

export type AuthzMatrixDemoDto = AuthzMatrixDemoData;

function factFromJson(value: unknown): AuthzMatrixFact {
  if (!isRecord(value)) throw new Error('authz matrix fact must be an object');
  return {
    label: requiredString(value, 'label'),
    value: requiredString(value, 'value'),
  };
}

function beatFromJson(value: unknown): AuthzMatrixBeat {
  if (!isRecord(value)) throw new Error('authz matrix beat must be an object');
  const facts = requiredArray(value, 'facts').map(factFromJson);
  return {
    beat: requiredString(value, 'beat'),
    detail: requiredString(value, 'detail'),
    verdict: requiredString(value, 'verdict'),
    facts,
  };
}

/** Deserialize + validate the matrix payload. */
export function authzMatrixDtoFromJson(body: unknown): AuthzMatrixDemoDto {
  if (!isRecord(body)) throw new Error('authz matrix payload must be an object');
  return {
    mechanism: requiredString(body, 'mechanism'),
    beats: requiredArray(body, 'beats').map(beatFromJson),
    proofs: requiredArray(body, 'proofs').map(factFromJson),
    reasoning: requiredString(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toAuthzMatrixDemo(dto: AuthzMatrixDemoDto): AuthzMatrixDemoData {
  return dto;
}

/** The matrix endpoint's path, keyed to the demo registry's id. */
export const AUTHZ_MATRIX_PATH = '/demo/authz/matrix';
