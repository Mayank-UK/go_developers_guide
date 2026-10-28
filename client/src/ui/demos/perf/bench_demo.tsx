import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import type { PerfBenchDemoData, PerfSample } from '../../../domain/demo/perf-bench';
import {
  GetPerfBenchDemoUseCase,
  PERF_BENCH_DEMO_USE_CASE,
} from '../../../domain/demo/get-perf-bench-demo-usecase';

/**
 * PerfBenchDemo — the performance topic's bench demo (demo block id
 * `perf:bench`). It renders the payload of
 * GET /api/demo/perf/bench: the run's provenance (exact command,
 * benchtime, count, wall-clock duration, age, environment), the
 * measured gzip curve (levels × sizes with the computed ratio and
 * BOTH count samples side by side — the distance between them is the
 * noise floor), the JSON marshal-versus-stream A/B, the serving hot
 * path's allocation tax, and the compiler's escape-analysis verdicts
 * verbatim. Every number is parsed from a real run; nothing is
 * averaged or invented.
 */

type BenchStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: PerfBenchDemoData }
  | { state: 'error'; failure: Failure };

/** formatNs renders an ns/op figure readably without rounding into lies. */
function formatNs(ns: number): string {
  if (ns >= 1_000_000) return `${(ns / 1_000_000).toFixed(2)}ms`;
  if (ns >= 1_000) return `${(ns / 1_000).toFixed(1)}µs`;
  return `${Math.round(ns)}ns`;
}

/** formatBytes renders a B/op or size figure with its binary unit. */
function formatBytes(bytes: number): string {
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)}KB`;
  return `${Math.round(bytes)}B`;
}

/** One sample rendered with only the columns the run actually carried. */
function SampleLine({ sample }: { sample: PerfSample }) {
  const mem = sample.hasMemStats
    ? ` · ${formatBytes(sample.bytesPerOp)}/op · ${sample.allocsPerOp} allocs/op`
    : ' · no mem stats';
  return (
    <span className="text-on-surface-variant">
      {' '}
      {formatNs(sample.nsPerOp)}
      {mem}
    </span>
  );
}

export function usePerfBenchDemo(
  useCase: GetPerfBenchDemoUseCase = container.resolve(PERF_BENCH_DEMO_USE_CASE),
): { status: BenchStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<BenchStatus>({ state: 'idle' });

  useEffect(() => {
    let cancelled = false;
    setStatus({ state: 'loading' });

    void useCase.execute().then((result) => {
      if (cancelled) return;
      if (result.ok) {
        setStatus({ state: 'ready', data: result.value });
      } else {
        setStatus({ state: 'error', failure: result.failure });
      }
    });

    return () => {
      cancelled = true;
    };
  }, [useCase, attempt]);

  const retry = () => setAttempt((current) => current + 1);
  return { status, retry };
}

export function PerfBenchDemo() {
  const { status, retry } = usePerfBenchDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="perf-bench-loading" className="text-body-sm text-on-surface-variant">
        running the curated benchmarks…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the perf bench demo" failure={status.failure} onRetry={retry} />;
  }

  const { data } = status;
  const run = data.run;

  return (
    <div className="flex flex-col gap-md" data-testid="perf-bench-ready">
      <section
        data-testid="perf-bench-run"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="text-caption text-on-surface-variant">{data.mode}</p>
        <p className="font-mono text-caption text-on-surface">
          {run.state}
          {run.state === 'running' ? ' — the boot run is still in flight; samples land when it finishes' : ''}
        </p>
        <p className="font-mono text-caption text-on-surface-variant">
          <span className="text-on-surface">{run.command}</span>
        </p>
        <p className="font-mono text-caption text-on-surface-variant">
          benchtime {run.benchtime} · count {run.count} ·{' '}
          {run.durationMs > 0
            ? `${(run.durationMs / 1000).toFixed(1)}s wall · ${Math.round(run.ageSeconds)}s ago`
            : 'not yet finished'}
          {run.cpu ? ` · ${run.goos}/${run.goarch} · ${run.cpu}` : ''}
        </p>
        {run.failedBenchmarks > 0 ? (
          <p className="font-mono text-caption text-on-surface">
            {run.failedBenchmarks} failed benchmark lines parsed from the run
          </p>
        ) : null}
      </section>

      <section
        data-testid="perf-bench-compression"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        {data.compression.map((cell) => (
          <p key={`gzip-${cell.level}-${cell.size}`} className="font-mono text-caption">
            <span className="text-on-surface">
              gzip {cell.level} · {cell.size}
            </span>
            {cell.compressedBytes > 0 ? (
              <span className="text-on-surface-variant">
                {' '}
                {formatBytes(cell.rawBytes)} → {formatBytes(cell.compressedBytes)} ({cell.ratio})
              </span>
            ) : null}
            {cell.samples.map((sample, index) => (
              <SampleLine key={index} sample={sample} />
            ))}
          </p>
        ))}
      </section>

      <section
        data-testid="perf-bench-encoding"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        {data.encoding.map((row) => (
          <div key={row.subtest} className="flex flex-col gap-xxs">
            <p className="font-mono text-caption">
              <span className="text-on-surface">{row.approach}</span>
              {row.samples.map((sample, index) => (
                <SampleLine key={index} sample={sample} />
              ))}
            </p>
            <p className="text-caption text-on-surface-variant">{row.note}</p>
          </div>
        ))}
        <div className="flex flex-col gap-xxs">
          <p className="font-mono text-caption">
            <span className="text-on-surface">{data.hotPath.benchmark}</span>
            {data.hotPath.samples.map((sample, index) => (
              <SampleLine key={index} sample={sample} />
            ))}
          </p>
          <p className="text-caption text-on-surface-variant">{data.hotPath.note}</p>
        </div>
      </section>

      <section
        data-testid="perf-bench-escape"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <p className="font-mono text-caption text-on-surface">
          escape analysis · {data.escape.state} · {data.escape.command}
        </p>
        {data.escape.verdicts.length > 0 ? (
          data.escape.verdicts.map((verdict) => (
            <p key={verdict.site + verdict.verdict} className="font-mono text-caption text-on-surface-variant">
              <span className="text-on-surface">{verdict.site}</span> — {verdict.verdict}
            </p>
          ))
        ) : (
          <p className="text-caption text-on-surface-variant">
            {data.escape.reason ??
              'the build said nothing about the focus lines — an honest empty, never filled in'}
          </p>
        )}
      </section>

      <section
        data-testid="perf-bench-checklist"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        {data.checklist.map((item) => (
          <p key={item} className="text-caption text-on-surface-variant">
            {item}
          </p>
        ))}
      </section>

      <p data-testid="perf-bench-reasoning" className="text-body-sm text-on-surface-variant">
        {data.reasoning}
      </p>
    </div>
  );
}

export default PerfBenchDemo;
