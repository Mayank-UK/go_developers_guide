import type {
  SecurityStoryBeat,
  SecurityStoryDemoData,
  SecurityStoryFact,
} from '../../domain/demo/security-story';
import { isRecord, requiredArray, requiredString } from '../content/validate';

/**
 * SecurityStoryDTO — the wire/transport shape of the security demos'
 * shared story payload. Mirrors the domain model 1:1; the `fromJson`
 * builds ONLY the listed fields (whitelist) and throws on any shape
 * violation, which parseDto upgrades to an InvalidResponseException.
 * Story prose and previews are server-minted per run — validated as
 * non-empty strings, never pinned to literals.
 */

export type SecurityStoryDemoDto = SecurityStoryDemoData;

function factFromJson(value: unknown): SecurityStoryFact {
  if (!isRecord(value)) throw new Error('security story fact must be an object');
  return {
    label: requiredString(value, 'label'),
    value: requiredString(value, 'value'),
  };
}

function beatFromJson(value: unknown): SecurityStoryBeat {
  if (!isRecord(value)) throw new Error('security story beat must be an object');
  const facts = requiredArray(value, 'facts').map(factFromJson);
  return {
    beat: requiredString(value, 'beat'),
    detail: requiredString(value, 'detail'),
    verdict: requiredString(value, 'verdict'),
    facts,
  };
}

/** Deserialize + validate one security story payload (either demo). */
export function securityStoryDtoFromJson(body: unknown): SecurityStoryDemoDto {
  if (!isRecord(body)) throw new Error('security story payload must be an object');
  return {
    mechanism: requiredString(body, 'mechanism'),
    beats: requiredArray(body, 'beats').map(beatFromJson),
    proofs: requiredArray(body, 'proofs').map(factFromJson),
    reasoning: requiredString(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toSecurityStoryDemo(dto: SecurityStoryDemoDto): SecurityStoryDemoData {
  return dto;
}

/** The story endpoints' paths, keyed by the demo registry's ids. */
export const SECURITY_STORY_PATHS = {
  lab: '/demo/security/lab',
  pii: '/demo/security/pii',
} as const;
