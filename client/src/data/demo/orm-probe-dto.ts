import type {
  ORMProbeDataset,
  ORMProbeDemoData,
  ORMProbeStep,
  ORMProbeTrial,
  ORMProbeUnitOfWork,
} from '../../domain/demo/orm-probe';
import {
  isRecord,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * ORMProbeDTO — the wire/transport shape of the measured data-access
 * payload. Mirrors the domain model 1:1; the `fromJson` builds ONLY the
 * listed fields (whitelist) and throws on any shape violation, which
 * parseDto upgrades to an InvalidResponseException.
 *
 * Trial strategies validate against the server's closed vocabulary (the
 * four fixed trial identities) so a drifting server fails here first.
 * `label`, `library`, `countingVia`, and step `detail` are deliberately
 * `requiredText` (present-but-empty passes): they are explanatory prose
 * the server curates, and an empty string is data, not absence.
 */

export type ORMProbeDemoDto = ORMProbeDemoData;

const TRIAL_STRATEGIES: readonly string[] = ['pgx-baseline', 'sqlc-join', 'gorm-lazy', 'gorm-eager'];

/** Deserialize + validate the GET /api/demo/db/orm-probe payload. */
export function ormProbeDtoFromJson(body: unknown): ORMProbeDemoDto {
  if (!isRecord(body)) throw new Error('orm-probe payload must be an object');
  return {
    dataset: datasetFromJson(body.dataset),
    trials: requiredArray(body, 'trials').map(trialFromJson),
    resultsAgree: requiredBoolean(body, 'resultsAgree'),
    unitOfWork: unitOfWorkFromJson(body.unitOfWork),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toORMProbeDemo(dto: ORMProbeDemoDto): ORMProbeDemoData {
  return dto;
}

/* ---- nested payload validators (throw on shape violation) ---- */

function datasetFromJson(body: unknown): ORMProbeDataset {
  if (!isRecord(body)) throw new Error('dataset must be an object');
  return {
    posts: requiredNumber(body, 'posts'),
    tags: requiredNumber(body, 'tags'),
    edges: requiredNumber(body, 'edges'),
  };
}

function trialFromJson(body: unknown): ORMProbeTrial {
  if (!isRecord(body)) throw new Error('trial must be an object');
  const strategy = requiredString(body, 'strategy');
  if (!TRIAL_STRATEGIES.includes(strategy)) {
    throw new Error(`unknown trial strategy "${strategy}"`);
  }
  return {
    strategy,
    label: requiredText(body, 'label'),
    library: requiredText(body, 'library'),
    countingVia: requiredText(body, 'countingVia'),
    statements: requiredNumber(body, 'statements'),
    roundTripRatio: requiredString(body, 'roundTripRatio'),
    rows: requiredNumber(body, 'rows'),
    durationMicros: requiredNumber(body, 'durationMicros'),
    timingBand: requiredString(body, 'timingBand'),
    sampleStatements: requiredArray(body, 'sampleStatements').map((sample) => {
      if (typeof sample !== 'string') {
        throw new Error('expected a statement string in sampleStatements');
      }
      return sample;
    }),
  };
}

function unitOfWorkFromJson(body: unknown): ORMProbeUnitOfWork {
  if (!isRecord(body)) throw new Error('unitOfWork must be an object');
  return {
    repositories: requiredArray(body, 'repositories').map((name) => {
      if (typeof name !== 'string' || name === '') {
        throw new Error('expected a non-empty repository name');
      }
      return name;
    }),
    steps: requiredArray(body, 'steps').map(stepFromJson),
    hookApplied: requiredBoolean(body, 'hookApplied'),
    postsBefore: requiredNumber(body, 'postsBefore'),
    postsAfter: requiredNumber(body, 'postsAfter'),
    rolledBack: requiredBoolean(body, 'rolledBack'),
  };
}

function stepFromJson(body: unknown): ORMProbeStep {
  if (!isRecord(body)) throw new Error('unit-of-work step must be an object');
  return {
    action: requiredString(body, 'action'),
    detail: requiredText(body, 'detail'),
  };
}
