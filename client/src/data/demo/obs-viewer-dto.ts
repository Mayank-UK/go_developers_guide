import type {
  ObsViewerAnatomy,
  ObsViewerDemoData,
  ObsViewerFieldRow,
  ObsViewerLevelRow,
  ObsViewerLevels,
  ObsViewerReplay,
  ObsViewerReplayedRequest,
  ObsViewerReplayLine,
} from '../../domain/demo/obs-viewer';
import {
  isRecord,
  optionalString,
  requiredArray,
  requiredNumber,
  requiredRecord,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * ObsViewerDTO — the wire/transport shape of the slog viewer payload.
 * Mirrors the domain model 1:1; the `fromJson` builds ONLY the listed
 * fields (whitelist) and throws on any shape violation, which parseDto
 * upgrades to an InvalidResponseException. Optional correlation fields
 * on replay lines (level/msg/requestId) are absent-when-unparseable by
 * contract — the domain marks them optional, the DTO reads them with
 * the optional-string helper.
 */

export type ObsViewerDemoDto = ObsViewerDemoData;

function fieldFromJson(value: unknown): ObsViewerFieldRow {
  if (!isRecord(value)) throw new Error('anatomy field must be an object');
  return {
    key: requiredString(value, 'key'),
    value: requiredString(value, 'value'),
    note: requiredString(value, 'note'),
  };
}

function anatomyFromJson(body: Record<string, unknown>): ObsViewerAnatomy {
  const anatomy = requiredRecord(body, 'anatomy');
  return {
    line: requiredString(anatomy, 'line'),
    fields: requiredArray(anatomy, 'fields').map(fieldFromJson),
    note: requiredString(anatomy, 'note'),
  };
}

function levelRowFromJson(value: unknown): ObsViewerLevelRow {
  if (!isRecord(value)) throw new Error('level row must be an object');
  const row = value as Record<string, unknown>;
  return {
    level: requiredString(row, 'level'),
    survivesAt: requiredArray(row, 'survivesAt').map((entry) => {
      if (typeof entry !== 'string') throw new Error('survivesAt entries must be strings');
      return entry;
    }),
    emittedBy: requiredString(row, 'emittedBy'),
    note: requiredString(row, 'note'),
  };
}

function levelsFromJson(body: Record<string, unknown>): ObsViewerLevels {
  const levels = requiredRecord(body, 'levels');
  return {
    rows: requiredArray(levels, 'rows').map(levelRowFromJson),
    gate: requiredString(levels, 'gate'),
    note: requiredString(levels, 'note'),
  };
}

function replayLineFromJson(value: unknown): ObsViewerReplayLine {
  if (!isRecord(value)) throw new Error('replay line must be an object');
  return {
    source: requiredString(value, 'source'),
    level: optionalString(value, 'level'),
    msg: optionalString(value, 'msg'),
    requestId: optionalString(value, 'requestId'),
    raw: requiredString(value, 'raw'),
  };
}

function replayFromJson(body: Record<string, unknown>): ObsViewerReplay {
  const replay = requiredRecord(body, 'replay');
  const requests = requiredArray(replay, 'requests').map((value) => {
    if (!isRecord(value)) throw new Error('replayed request must be an object');
    return {
      label: requiredString(value, 'label'),
      request: requiredString(value, 'request'),
      requestId: requiredString(value, 'requestId'),
      status: requiredNumber(value, 'status'),
    } satisfies ObsViewerReplayedRequest;
  });
  return {
    requests,
    lines: requiredArray(replay, 'lines').map(replayLineFromJson),
    note: requiredString(replay, 'note'),
  };
}

/** Deserialize + validate the GET /api/demo/obs/viewer payload. */
export function obsViewerDtoFromJson(body: unknown): ObsViewerDemoDto {
  if (!isRecord(body)) throw new Error('obs viewer payload must be an object');
  return {
    anatomy: anatomyFromJson(body),
    levels: levelsFromJson(body),
    replay: replayFromJson(body),
    // Present-but-empty is real data: reasoning is server prose.
    reasoning: requiredText(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toObsViewerDemo(dto: ObsViewerDemoDto): ObsViewerDemoData {
  return dto;
}
