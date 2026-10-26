import type {
  CancellationLedger,
  ChoreographyFacts,
  ContextCancellationDemoData,
  DeadlineOutcome,
  PropagationRow,
  TreeEdge,
  WorkRow,
} from '../../domain/demo/context-cancellation';
import {
  isRecord,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredString,
} from '../content/validate';

/**
 * ContextCancellationDTO — the wire/transport shape of the
 * context-cancellation demo payload. Mirrors the domain model 1:1 (the
 * interface is the seam where wire and domain may diverge); the
 * `fromJson` builds ONLY the listed fields (whitelist) and throws on any
 * shape violation, which parseDto upgrades to an
 * InvalidResponseException. Every string field on this payload is real
 * data (no empty-string-as-data cases), so `requiredString` suffices.
 */

export type ContextCancellationDemoDto = ContextCancellationDemoData;

/** Deserialize + validate the GET /api/demo/conc/context payload. */
export function contextCancellationDtoFromJson(body: unknown): ContextCancellationDemoDto {
  if (!isRecord(body)) throw new Error('context-cancellation payload must be an object');
  return {
    choreography: choreographyFromJson(body.choreography),
    workPhase: requiredArray(body, 'workPhase').map(workRowFromJson),
    propagation: requiredArray(body, 'propagation').map(propagationRowFromJson),
    deadlineBeat: deadlineOutcomeFromJson(body.deadlineBeat),
    ledger: ledgerFromJson(body.ledger),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toContextCancellationDemo(dto: ContextCancellationDemoDto): ContextCancellationDemoData {
  return dto;
}

/* ---- nested payload validators (throw on shape violation) ---- */

function choreographyFromJson(body: unknown): ChoreographyFacts {
  if (!isRecord(body)) throw new Error('choreography must be an object');
  return {
    deadlineBudgetMs: requiredNumber(body, 'deadlineBudgetMs'),
    workItemsPerWorker: requiredNumber(body, 'workItemsPerWorker'),
    trigger: requiredString(body, 'trigger'),
    tree: requiredArray(body, 'tree').map(treeEdgeFromJson),
    ordering: requiredString(body, 'ordering'),
  };
}

function treeEdgeFromJson(body: unknown): TreeEdge {
  if (!isRecord(body)) throw new Error('tree edge must be an object');
  return {
    from: requiredString(body, 'from'),
    to: requiredString(body, 'to'),
  };
}

function workRowFromJson(body: unknown): WorkRow {
  if (!isRecord(body)) throw new Error('work row must be an object');
  return {
    workerId: requiredString(body, 'workerId'),
    generation: requiredNumber(body, 'generation'),
    itemsProcessed: requiredNumber(body, 'itemsProcessed'),
  };
}

function propagationRowFromJson(body: unknown): PropagationRow {
  if (!isRecord(body)) throw new Error('propagation row must be an object');
  return {
    workerId: requiredString(body, 'workerId'),
    generation: requiredNumber(body, 'generation'),
    derivedFrom: requiredString(body, 'derivedFrom'),
    observedPosition: requiredNumber(body, 'observedPosition'),
    cause: requiredString(body, 'cause'),
  };
}

function deadlineOutcomeFromJson(body: unknown): DeadlineOutcome {
  if (!isRecord(body)) throw new Error('deadline outcome must be an object');
  return {
    budgetMs: requiredNumber(body, 'budgetMs'),
    cancelCalled: requiredBoolean(body, 'cancelCalled'),
    outcome: requiredString(body, 'outcome'),
    cause: requiredString(body, 'cause'),
    deadlineExceeded: requiredBoolean(body, 'deadlineExceeded'),
  };
}

function ledgerFromJson(body: unknown): CancellationLedger {
  if (!isRecord(body)) throw new Error('ledger must be an object');
  return {
    goroutinesLaunched: requiredNumber(body, 'goroutinesLaunched'),
    goroutinesJoined: requiredNumber(body, 'goroutinesJoined'),
    contextsCreated: requiredNumber(body, 'contextsCreated'),
    contextsCancelled: requiredNumber(body, 'contextsCancelled'),
    outcome: requiredString(body, 'outcome'),
  };
}
