import type {
  ArenaAnatomyRow,
  ArenaCandidate,
  ArenaComparison,
  ArenaDecodeOutcome,
  ArenaDecodeStatus,
  ArenaEnvelopeShow,
  ArenaFieldSpec,
  ArenaIssue,
  ArenaLayerCard,
  ArenaLayerId,
  ArenaResponse,
  ArenaRulesOutcome,
  ArenaRulesStatus,
  ArenaSubject,
  ArenaTagRow,
  ArenaValidatedForm,
  ValidationArenaDemoData,
} from '../../domain/demo/validation-arena';
import {
  isRecord,
  optionalRecord,
  optionalString,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredRecord,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * ValidationArenaDTO — the wire/transport shape of the validation arena
 * payload. Mirrors the domain model 1:1 (the interface is the seam where
 * wire and domain may diverge); the `fromJson` builds ONLY the listed
 * fields (whitelist) and throws on any shape violation, which parseDto
 * upgrades to an InvalidResponseException.
 *
 * `body` fields are deliberately `requiredText` (present-but-empty
 * passes): the absent-optional-body candidate's validated form carries
 * `body: ""` — the zero-value default is DATA there, not absence. The
 * decode/validate/layer vocabularies validate against the server's
 * closed sets so a drifting builder fails here first. Optional wire keys
 * (`errorType`, `error`, `issues`, `note`, `envelope`, `validated`) are
 * OMITTED by the server when absent — they must land as undefined, never
 * null.
 */

export type ValidationArenaDemoDto = ValidationArenaDemoData;

const DECODE_STATUSES: readonly string[] = ['accepted', 'rejected'];
const RULES_STATUSES: readonly string[] = ['validated', 'rejected', 'skipped'];
const LAYER_IDS: readonly string[] = ['decode', 'rules', 'library'];

/** Deserialize + validate the GET /api/demo/validation/arena payload. */
export function validationArenaDtoFromJson(body: unknown): ValidationArenaDemoDto {
  if (!isRecord(body)) throw new Error('validation-arena payload must be an object');
  return {
    subject: arenaSubjectFromJson(requiredRecord(body, 'subject')),
    layers: requiredArray(body, 'layers').map(arenaLayerFromJson),
    candidates: requiredArray(body, 'candidates').map(arenaCandidateFromJson),
    comparison: arenaComparisonFromJson(requiredRecord(body, 'comparison')),
    envelope: arenaEnvelopeFromJson(requiredRecord(body, 'envelope')),
    reasoning: requiredText(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toValidationArenaDemo(dto: ValidationArenaDemoDto): ValidationArenaDemoData {
  return dto;
}

/* ---- nested payload validators (throw on shape violation) ---- */

function arenaSubjectFromJson(body: unknown): ArenaSubject {
  if (!isRecord(body)) throw new Error('arena subject must be an object');
  return {
    name: requiredString(body, 'name'),
    source: requiredString(body, 'source'),
    fields: requiredArray(body, 'fields').map(arenaFieldFromJson),
  };
}

function arenaFieldFromJson(body: unknown): ArenaFieldSpec {
  if (!isRecord(body)) throw new Error('arena field must be an object');
  return {
    name: requiredString(body, 'name'),
    type: requiredString(body, 'type'),
    rule: requiredString(body, 'rule'),
    required: requiredBoolean(body, 'required'),
  };
}

function arenaLayerFromJson(body: unknown): ArenaLayerCard {
  if (!isRecord(body)) throw new Error('arena layer must be an object');
  const id = requiredString(body, 'id');
  if (!LAYER_IDS.includes(id)) {
    throw new Error(`unknown arena layer id "${id}"`);
  }
  return {
    id: id as ArenaLayerId,
    title: requiredString(body, 'title'),
    tool: requiredString(body, 'tool'),
    summary: requiredString(body, 'summary'),
  };
}

function arenaCandidateFromJson(body: unknown): ArenaCandidate {
  if (!isRecord(body)) throw new Error('arena candidate must be an object');
  return {
    id: requiredString(body, 'id'),
    label: requiredString(body, 'label'),
    expects: requiredString(body, 'expects'),
    body: requiredText(body, 'body'),
    decode: arenaDecodeFromJson(requiredRecord(body, 'decode')),
    validate: arenaRulesFromJson(requiredRecord(body, 'validate')),
    response: arenaResponseFromJson(requiredRecord(body, 'response')),
    note: optionalString(body, 'note'),
  };
}

function arenaDecodeFromJson(body: unknown): ArenaDecodeOutcome {
  if (!isRecord(body)) throw new Error('arena decode outcome must be an object');
  const status = requiredString(body, 'status');
  if (!DECODE_STATUSES.includes(status)) {
    throw new Error(`unknown arena decode status "${status}"`);
  }
  return {
    status: status as ArenaDecodeStatus,
    errorType: optionalString(body, 'errorType'),
    error: optionalString(body, 'error'),
  };
}

function arenaRulesFromJson(body: unknown): ArenaRulesOutcome {
  if (!isRecord(body)) throw new Error('arena rules outcome must be an object');
  const status = requiredString(body, 'status');
  if (!RULES_STATUSES.includes(status)) {
    throw new Error(`unknown arena rules status "${status}"`);
  }
  const issues = body['issues'] === undefined ? undefined : requiredArray(body, 'issues').map(arenaIssueFromJson);
  return {
    status: status as ArenaRulesStatus,
    issues,
  };
}

function arenaIssueFromJson(body: unknown): ArenaIssue {
  if (!isRecord(body)) throw new Error('arena issue must be an object');
  return {
    field: requiredString(body, 'field'),
    rule: requiredString(body, 'rule'),
    got: requiredString(body, 'got'),
  };
}

function arenaResponseFromJson(body: unknown): ArenaResponse {
  if (!isRecord(body)) throw new Error('arena response must be an object');
  const validated = body['validated'] === undefined ? undefined : arenaValidatedFromJson(body['validated']);
  return {
    status: requiredNumber(body, 'status'),
    envelope: optionalRecord(body, 'envelope'),
    validated,
  };
}

function arenaValidatedFromJson(body: unknown): ArenaValidatedForm {
  if (!isRecord(body)) throw new Error('arena validated form must be an object');
  return {
    title: requiredString(body, 'title'),
    body: requiredText(body, 'body'),
    score: requiredNumber(body, 'score'),
    email: requiredString(body, 'email'),
  };
}

function arenaComparisonFromJson(body: unknown): ArenaComparison {
  if (!isRecord(body)) throw new Error('arena comparison must be an object');
  return {
    library: requiredString(body, 'library'),
    imported: requiredBoolean(body, 'imported'),
    rows: requiredArray(body, 'rows').map(arenaTagRowFromJson),
    note: requiredString(body, 'note'),
  };
}

function arenaTagRowFromJson(body: unknown): ArenaTagRow {
  if (!isRecord(body)) throw new Error('arena tag row must be an object');
  return {
    field: requiredString(body, 'field'),
    tag: requiredString(body, 'tag'),
    libraryRule: requiredString(body, 'libraryRule'),
    handRolled: requiredString(body, 'handRolled'),
  };
}

function arenaEnvelopeFromJson(body: unknown): ArenaEnvelopeShow {
  if (!isRecord(body)) throw new Error('arena envelope exhibit must be an object');
  return {
    candidateId: requiredString(body, 'candidateId'),
    status: requiredNumber(body, 'status'),
    body: requiredRecord(body, 'body'),
    anatomy: requiredArray(body, 'anatomy').map(arenaAnatomyRowFromJson),
  };
}

function arenaAnatomyRowFromJson(body: unknown): ArenaAnatomyRow {
  if (!isRecord(body)) throw new Error('arena anatomy row must be an object');
  return {
    path: requiredString(body, 'path'),
    value: requiredString(body, 'value'),
    why: requiredString(body, 'why'),
  };
}
