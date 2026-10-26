import type {
  HygieneRow,
  RaceDetectorDemoData,
  RaceExhibit,
  SynchronizedRun,
} from '../../domain/demo/race-detector';
import {
  isRecord,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredString,
} from '../content/validate';

/**
 * RaceDetectorDTO — the wire/transport shape of the race-detector demo
 * payload. Mirrors the domain model 1:1; the `fromJson` builds ONLY the
 * listed fields (whitelist) and throws on any shape violation, which
 * parseDto upgrades to an InvalidResponseException. The multi-line
 * `code`/`verdict` strings are plain required strings — their content is
 * the server's pinned exhibit, not something the client interprets.
 */

export type RaceDetectorDemoDto = RaceDetectorDemoData;

/** Deserialize + validate the GET /api/demo/conc/race payload. */
export function raceDetectorDtoFromJson(body: unknown): RaceDetectorDemoDto {
  if (!isRecord(body)) throw new Error('race-detector payload must be an object');
  return {
    race: raceExhibitFromJson(body.race),
    mutex: synchronizedRunFromJson(body.mutex),
    channel: synchronizedRunFromJson(body.channel),
    hygiene: requiredArray(body, 'hygiene').map(hygieneRowFromJson),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toRaceDetectorDemo(dto: RaceDetectorDemoDto): RaceDetectorDemoData {
  return dto;
}

/* ---- nested payload validators (throw on shape violation) ---- */

function raceExhibitFromJson(body: unknown): RaceExhibit {
  if (!isRecord(body)) throw new Error('race exhibit must be an object');
  return {
    code: requiredString(body, 'code'),
    command: requiredString(body, 'command'),
    verdict: requiredString(body, 'verdict'),
    explanation: requiredString(body, 'explanation'),
    fix: requiredString(body, 'fix'),
  };
}

function synchronizedRunFromJson(body: unknown): SynchronizedRun {
  if (!isRecord(body)) throw new Error('synchronized run must be an object');
  return {
    strategy: requiredString(body, 'strategy'),
    goroutines: requiredNumber(body, 'goroutines'),
    incrementsEach: requiredNumber(body, 'incrementsEach'),
    expected: requiredNumber(body, 'expected'),
    observed: requiredNumber(body, 'observed'),
    exact: requiredBoolean(body, 'exact'),
  };
}

function hygieneRowFromJson(body: unknown): HygieneRow {
  if (!isRecord(body)) throw new Error('hygiene row must be an object');
  return {
    case: requiredString(body, 'case'),
    readerPresent: requiredBoolean(body, 'readerPresent'),
    outcome: requiredString(body, 'outcome'),
  };
}
