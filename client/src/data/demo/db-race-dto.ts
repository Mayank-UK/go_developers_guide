import type { RaceDemoData, RaceStep } from '../../domain/demo/db-race';
import { isRecord, requiredArray, requiredBoolean, requiredString } from '../content/validate';

/** RaceDTO — the wire/transport shape of the isolation demo payload. */

export type RaceDemoDto = RaceDemoData;

/** Deserialize + validate the GET /api/demo/db/race payload. */
export function raceDtoFromJson(body: unknown): RaceDemoDto {
  if (!isRecord(body)) throw new Error('race payload must be an object');
  return {
    isolation: requiredString(body, 'isolation'),
    scratchBefore: requiredString(body, 'scratchBefore'),
    scratchAfter: requiredString(body, 'scratchAfter'),
    steps: requiredArray(body, 'steps').map(stepFromJson),
    lockWaited: requiredBoolean(body, 'lockWaited'),
    serializableNote: requiredString(body, 'serializableNote'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toRaceDemo(dto: RaceDemoDto): RaceDemoData {
  return dto;
}

function stepFromJson(body: unknown): RaceStep {
  if (!isRecord(body)) throw new Error('race step must be an object');
  return {
    phase: requiredString(body, 'phase'),
    actor: requiredString(body, 'actor'),
    action: requiredString(body, 'action'),
    observed: requiredString(body, 'observed'),
    note: requiredString(body, 'note'),
  };
}
