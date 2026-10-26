import type { SearchDemoData, SearchMatch } from '../../domain/demo/db-search';
import { isRecord, requiredArray, requiredString } from '../content/validate';

/**
 * SearchDTO — the wire/transport shape of the search demo payload. Plan
 * lines and matches validate as non-empty strings; the index definition
 * is the catalog's own rendering.
 */

export type SearchDemoDto = SearchDemoData;

/** Deserialize + validate the GET /api/demo/db/search payload. */
export function searchDtoFromJson(body: unknown): SearchDemoDto {
  if (!isRecord(body)) throw new Error('search payload must be an object');
  return {
    query: requiredString(body, 'query'),
    tsQuery: requiredString(body, 'tsQuery'),
    sql: requiredString(body, 'sql'),
    matches: requiredArray(body, 'matches').map(matchFromJson),
    plan: stringArrayOf(body, 'plan'),
    indexDefinition: requiredString(body, 'indexDefinition'),
    teach: requiredString(body, 'teach'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toSearchDemo(dto: SearchDemoDto): SearchDemoData {
  return dto;
}

function matchFromJson(body: unknown): SearchMatch {
  if (!isRecord(body)) throw new Error('match must be an object');
  return { title: requiredString(body, 'title'), rank: requiredString(body, 'rank') };
}

function stringArrayOf(source: Record<string, unknown>, key: string): string[] {
  return requiredArray(source, key).map((line) => {
    if (typeof line !== 'string' || line === '') {
      throw new Error(`expected a non-empty string in "${key}"`);
    }
    return line;
  });
}
