import type {
  AuthStoryBeat,
  AuthStoryDemoData,
  AuthStoryFact,
} from '../../domain/demo/auth-story';
import { isRecord, requiredArray, requiredString } from '../content/validate';

/**
 * AuthStoryDTO — the wire/transport shape of the auth demos' shared
 * story payload. Mirrors the domain model 1:1; the `fromJson` builds
 * ONLY the listed fields (whitelist) and throws on any shape violation,
 * which parseDto upgrades to an InvalidResponseException. Story prose
 * and previews are server-minted per run — validated as non-empty
 * strings, never pinned to literals.
 */

export type AuthStoryDemoDto = AuthStoryDemoData;

function factFromJson(value: unknown): AuthStoryFact {
  if (!isRecord(value)) throw new Error('auth story fact must be an object');
  return {
    label: requiredString(value, 'label'),
    value: requiredString(value, 'value'),
  };
}

function beatFromJson(value: unknown): AuthStoryBeat {
  if (!isRecord(value)) throw new Error('auth story beat must be an object');
  const facts = requiredArray(value, 'facts').map(factFromJson);
  return {
    beat: requiredString(value, 'beat'),
    detail: requiredString(value, 'detail'),
    verdict: requiredString(value, 'verdict'),
    facts,
  };
}

/** Deserialize + validate one auth story payload (any of the seven). */
export function authStoryDtoFromJson(body: unknown): AuthStoryDemoDto {
  if (!isRecord(body)) throw new Error('auth story payload must be an object');
  return {
    mechanism: requiredString(body, 'mechanism'),
    beats: requiredArray(body, 'beats').map(beatFromJson),
    proofs: requiredArray(body, 'proofs').map(factFromJson),
    reasoning: requiredString(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toAuthStoryDemo(dto: AuthStoryDemoDto): AuthStoryDemoData {
  return dto;
}

/** The story endpoints' paths, keyed by the demo registry's ids. */
export const AUTH_STORY_PATHS = {
  sessions: '/demo/auth/sessions',
  apiKeys: '/demo/auth/api-keys',
  compare: '/demo/auth/compare',
  jwt: '/demo/auth/jwt',
  oauth: '/demo/auth/oauth',
  hashing: '/demo/auth/hashing',
  refresh: '/demo/auth/refresh',
} as const;
