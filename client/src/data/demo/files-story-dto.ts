import type {
  FilesStoryBeat,
  FilesStoryDemoData,
  FilesStoryFact,
} from '../../domain/demo/files-story';
import { isRecord, requiredArray, requiredString } from '../content/validate';

/**
 * FilesStoryDTO — the wire/transport shape of the files topic's
 * workbench story payload. Mirrors the domain model 1:1; the
 * `fromJson` builds ONLY the listed fields (whitelist) and throws on
 * any shape violation, which parseDto upgrades to an
 * InvalidResponseException. Verdicts validate against the server's
 * closed vocabulary (ok/rejected/info), and story prose is
 * server-minted per run — validated as non-empty strings, never pinned
 * to literals.
 */

export type FilesStoryDemoDto = FilesStoryDemoData;

const VERDICTS = new Set(['ok', 'rejected', 'info']);

function factFromJson(value: unknown): FilesStoryFact {
  if (!isRecord(value)) throw new Error('files story fact must be an object');
  return {
    label: requiredString(value, 'label'),
    value: requiredString(value, 'value'),
  };
}

function beatFromJson(value: unknown): FilesStoryBeat {
  if (!isRecord(value)) throw new Error('files story beat must be an object');
  const verdict = requiredString(value, 'verdict');
  if (!VERDICTS.has(verdict)) {
    throw new Error(`unknown files story verdict "${verdict}" (expected ok|rejected|info)`);
  }
  return {
    beat: requiredString(value, 'beat'),
    detail: requiredString(value, 'detail'),
    verdict,
    facts: requiredArray(value, 'facts').map(factFromJson),
  };
}

/** Deserialize + validate one workbench story payload. */
export function filesStoryDtoFromJson(body: unknown): FilesStoryDemoDto {
  if (!isRecord(body)) throw new Error('files story payload must be an object');
  return {
    mechanism: requiredString(body, 'mechanism'),
    beats: requiredArray(body, 'beats').map(beatFromJson),
    proofs: requiredArray(body, 'proofs').map(factFromJson),
    reasoning: requiredString(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toFilesStoryDemo(dto: FilesStoryDemoDto): FilesStoryDemoData {
  return dto;
}

/** The workbench story endpoint's path (the demo registry's id is `files:workbench`). */
export const FILES_STORY_PATH = '/demo/files/workbench' as const;
