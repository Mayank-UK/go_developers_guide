import type {
  ScalingConfigPanel,
  ScalingCounterPanel,
  ScalingCounterRow,
  ScalingDistributionPanel,
  ScalingDistributionRow,
  ScalingStatelessDemoData,
  ScalingStepView,
  ScalingVerdictView,
} from '../../domain/demo/scaling-stateless';
import {
  isRecord,
  requiredArray,
  requiredNumber,
  requiredRecord,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * ScalingStatelessDTO — the wire/transport shape of the statelessness
 * payload. Mirrors the domain model 1:1; the `fromJson` builds ONLY
 * the listed fields (whitelist) and throws on any shape violation,
 * which parseDto upgrades to an InvalidResponseException.
 */

export type ScalingStatelessDemoDto = ScalingStatelessDemoData;

function configFromJson(value: unknown): ScalingConfigPanel {
  if (!isRecord(value)) throw new Error('config panel must be an object');
  return {
    replicas: requiredNumber(value, 'replicas'),
    distributionRequests: requiredNumber(value, 'distributionRequests'),
    clientId: requiredString(value, 'clientId'),
    counterIncrements: requiredNumber(value, 'counterIncrements'),
    localToken: requiredText(value, 'localToken'),
    sharedToken: requiredText(value, 'sharedToken'),
    sharedStoreNote: requiredText(value, 'sharedStoreNote'),
  };
}

function distributionRowFromJson(value: unknown): ScalingDistributionRow {
  if (!isRecord(value)) throw new Error('distribution row must be an object');
  return {
    replicaId: requiredString(value, 'replicaId'),
    served: requiredNumber(value, 'served'),
  };
}

function distributionFromJson(value: unknown): ScalingDistributionPanel {
  if (!isRecord(value)) throw new Error('distribution panel must be an object');
  return {
    totalServed: requiredNumber(value, 'totalServed'),
    rows: requiredArray(value, 'rows').map(distributionRowFromJson),
    note: requiredText(value, 'note'),
  };
}

function stepFromJson(value: unknown): ScalingStepView {
  if (!isRecord(value)) throw new Error('session step must be an object');
  return {
    step: requiredNumber(value, 'step'),
    phase: requiredString(value, 'phase'),
    label: requiredText(value, 'label'),
    route: requiredString(value, 'route'),
    replicaId: requiredString(value, 'replicaId'),
    outcome: requiredString(value, 'outcome'),
    detail: requiredText(value, 'detail'),
    observed: requiredText(value, 'observed'),
  };
}

function counterRowFromJson(value: unknown): ScalingCounterRow {
  if (!isRecord(value)) throw new Error('counter row must be an object');
  return {
    replicaId: requiredString(value, 'replicaId'),
    increments: requiredNumber(value, 'increments'),
  };
}

function counterFromJson(value: unknown): ScalingCounterPanel {
  if (!isRecord(value)) throw new Error('counter panel must be an object');
  return {
    rows: requiredArray(value, 'rows').map(counterRowFromJson),
    total: requiredNumber(value, 'total'),
    note: requiredText(value, 'note'),
  };
}

function verdictFromJson(value: unknown): ScalingVerdictView {
  if (!isRecord(value)) throw new Error('verdict must be an object');
  return {
    id: requiredString(value, 'id'),
    label: requiredText(value, 'label'),
    value: requiredText(value, 'value'),
  };
}

/** Deserialize + validate the GET /api/demo/scaling/stateless payload. */
export function scalingStatelessDtoFromJson(body: unknown): ScalingStatelessDemoDto {
  if (!isRecord(body)) throw new Error('scaling stateless payload must be an object');
  return {
    mode: requiredText(body, 'mode'),
    config: configFromJson(requiredRecord(body, 'config')),
    distribution: distributionFromJson(requiredRecord(body, 'distribution')),
    steps: requiredArray(body, 'steps').map(stepFromJson),
    counter: counterFromJson(requiredRecord(body, 'counter')),
    verdicts: requiredArray(body, 'verdicts').map(verdictFromJson),
    honesty: requiredText(body, 'honesty'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toScalingStatelessDemo(dto: ScalingStatelessDemoDto): ScalingStatelessDemoData {
  return dto;
}
