import type {
  PerfBenchDemoData,
  PerfCompressionRow,
  PerfEncodeRow,
  PerfEscapePanel,
  PerfEscapeVerdict,
  PerfHotPathRow,
  PerfRunPanel,
  PerfSample,
} from '../../domain/demo/perf-bench';
import {
  isRecord,
  optionalString,
  requiredArray,
  requiredBoolean,
  requiredNumber,
  requiredRecord,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * PerfBenchDTO — the wire/transport shape of the perf bench payload.
 * Mirrors the domain model 1:1; the `fromJson` builds ONLY the listed
 * fields (whitelist) and throws on any shape violation, which
 * parseDto upgrades to an InvalidResponseException.
 */

export type PerfBenchDemoDto = PerfBenchDemoData;

function sampleFromJson(value: unknown): PerfSample {
  if (!isRecord(value)) throw new Error('benchmark sample must be an object');
  return {
    iterations: requiredNumber(value, 'iterations'),
    nsPerOp: requiredNumber(value, 'nsPerOp'),
    bytesPerOp: requiredNumber(value, 'bytesPerOp'),
    allocsPerOp: requiredNumber(value, 'allocsPerOp'),
    hasMemStats: requiredBoolean(value, 'hasMemStats'),
  };
}

function samplesOf(source: Record<string, unknown>): PerfSample[] {
  return requiredArray(source, 'samples').map(sampleFromJson);
}

function runPanelFromJson(value: unknown): PerfRunPanel {
  if (!isRecord(value)) throw new Error('run panel must be an object');
  return {
    state: requiredString(value, 'state'),
    reason: optionalString(value, 'reason'),
    command: requiredText(value, 'command'),
    startedAt: optionalString(value, 'startedAt'),
    finishedAt: optionalString(value, 'finishedAt'),
    durationMs: requiredNumber(value, 'durationMs'),
    ageSeconds: requiredNumber(value, 'ageSeconds'),
    exitCode: requiredNumber(value, 'exitCode'),
    goos: optionalString(value, 'goos'),
    goarch: optionalString(value, 'goarch'),
    cpu: optionalString(value, 'cpu'),
    pkg: optionalString(value, 'pkg'),
    benchtime: requiredString(value, 'benchtime'),
    count: requiredNumber(value, 'count'),
    failedBenchmarks: requiredNumber(value, 'failedBenchmarks'),
  };
}

function compressionRowFromJson(value: unknown): PerfCompressionRow {
  if (!isRecord(value)) throw new Error('compression row must be an object');
  return {
    level: requiredNumber(value, 'level'),
    size: requiredString(value, 'size'),
    rawBytes: requiredNumber(value, 'rawBytes'),
    compressedBytes: requiredNumber(value, 'compressedBytes'),
    ratio: requiredString(value, 'ratio'),
    samples: samplesOf(value),
  };
}

function encodeRowFromJson(value: unknown): PerfEncodeRow {
  if (!isRecord(value)) throw new Error('encode row must be an object');
  return {
    approach: requiredString(value, 'approach'),
    subtest: requiredString(value, 'subtest'),
    note: requiredText(value, 'note'),
    samples: samplesOf(value),
  };
}

function hotPathRowFromJson(value: unknown): PerfHotPathRow {
  if (!isRecord(value)) throw new Error('hot path row must be an object');
  return {
    benchmark: requiredString(value, 'benchmark'),
    note: requiredText(value, 'note'),
    samples: samplesOf(value),
  };
}

function verdictFromJson(value: unknown): PerfEscapeVerdict {
  if (!isRecord(value)) throw new Error('escape verdict must be an object');
  return {
    site: requiredString(value, 'site'),
    verdict: requiredString(value, 'verdict'),
    line: requiredText(value, 'line'),
  };
}

function escapePanelFromJson(value: unknown): PerfEscapePanel {
  if (!isRecord(value)) throw new Error('escape panel must be an object');
  return {
    command: requiredText(value, 'command'),
    state: requiredString(value, 'state'),
    reason: optionalString(value, 'reason'),
    verdicts: requiredArray(value, 'verdicts').map(verdictFromJson),
  };
}

function checklistFromJson(body: Record<string, unknown>): string[] {
  return requiredArray(body, 'checklist').map((item) => {
    if (typeof item !== 'string' || item === '') {
      throw new Error('checklist items must be non-empty strings');
    }
    return item;
  });
}

/** Deserialize + validate the GET /api/demo/perf/bench payload. */
export function perfBenchDtoFromJson(body: unknown): PerfBenchDemoDto {
  if (!isRecord(body)) throw new Error('perf bench payload must be an object');
  return {
    mode: requiredText(body, 'mode'),
    run: runPanelFromJson(requiredRecord(body, 'run')),
    compression: requiredArray(body, 'compression').map(compressionRowFromJson),
    encoding: requiredArray(body, 'encoding').map(encodeRowFromJson),
    hotPath: hotPathRowFromJson(requiredRecord(body, 'hotPath')),
    escape: escapePanelFromJson(requiredRecord(body, 'escape')),
    checklist: checklistFromJson(body),
    reasoning: requiredText(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toPerfBenchDemo(dto: PerfBenchDemoDto): PerfBenchDemoData {
  return dto;
}
