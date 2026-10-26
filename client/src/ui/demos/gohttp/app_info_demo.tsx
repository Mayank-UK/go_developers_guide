import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import type { GetAppInfoDemoUseCase } from '../../../domain/demo/get-app-info-demo-usecase';
import { GetAppInfoDemoUseCaseToken } from '../../../domain/demo/get-app-info-demo-usecase';
import type {
  AppInfoDemoData,
  AppInfoEnvironment,
  AppInfoModule,
  AppInfoRuntime,
} from '../../../domain/demo/app-info';

/**
 * AppInfoDemo — the go-http-fundamentals topic's live demo (demo block id
 * `gohttp:app-info`). It renders the payload of
 * GET /api/demo/gohttp/app-info, which the server computes by
 * introspecting the very binary serving it: the module and toolchain
 * runtime/debug.ReadBuildInfo reports (with the dependency count — zero,
 * the stdlib-only manifest confirmed live), the CPU facts the scheduler
 * sees, the port/env/log-level the process actually booted with, and the
 * toolchain's build settings, sorted by key.
 *
 * Dependency flow mirrors the other demos: the hook resolves the use case
 * through the container — under GetAppInfoDemoUseCaseToken, the stable DI
 * seam this wave's slice owns until container.ts grows the family's token
 * pair + getter at wave close (specs substitute fakes through the same
 * token). Failures arrive as typed Failures inside Results, and `retry`
 * re-runs the load.
 */

type AppInfoDemoStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: AppInfoDemoData }
  | { state: 'error'; failure: Failure };

export function useAppInfoDemo(
  useCase: GetAppInfoDemoUseCase = container.resolve<GetAppInfoDemoUseCase>(
    GetAppInfoDemoUseCaseToken,
  ),
): { status: AppInfoDemoStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<AppInfoDemoStatus>({ state: 'idle' });

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

export function AppInfoDemo() {
  const { status, retry } = useAppInfoDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="gohttp-app-info-loading" className="text-body-sm text-on-surface-variant">
        introspecting the running server…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the app-info demo" failure={status.failure} onRetry={retry} />;
  }

  const { module: moduleInfo, runtime, environment, buildSettings } = status.data;

  return (
    <div className="flex flex-col gap-md">
      <ModuleFacts moduleInfo={moduleInfo} />
      <RuntimeFacts runtime={runtime} />
      <EnvironmentFacts environment={environment} />
      <BuildSettingsTable rows={buildSettings} />
    </div>
  );
}

/** The server's identity: module path, version, toolchain, dependency count. */
function ModuleFacts({ moduleInfo }: { moduleInfo: AppInfoModule }) {
  return (
    <section data-testid="ai-module" className="flex flex-wrap items-center gap-xs">
      <span className="font-display text-title-sm text-on-surface">{moduleInfo.path}</span>
      <span className="rounded-pill border border-outline px-sm py-xxs font-mono text-caption text-on-surface-variant">
        v{moduleInfo.version}
      </span>
      <span className="rounded-pill border border-outline px-sm py-xxs font-mono text-caption text-on-surface-variant">
        {moduleInfo.goVersion}
      </span>
      <span
        data-testid="ai-dependency-count"
        className="rounded-pill border border-outline bg-surface-tint px-sm py-xxs font-mono text-caption text-secondary"
      >
        {moduleInfo.dependencyCount} dependencies
      </span>
    </section>
  );
}

/** The machine as the scheduler sees it. */
function RuntimeFacts({ runtime }: { runtime: AppInfoRuntime }) {
  return (
    <section data-testid="ai-runtime" className="flex flex-wrap items-center gap-xs">
      <span className="text-caption text-on-surface-variant">the runtime:</span>
      <span className="rounded-pill border border-outline px-sm py-xxs font-mono text-caption text-on-surface-variant">
        NumCPU {runtime.numCPU}
      </span>
      <span className="rounded-pill border border-outline px-sm py-xxs font-mono text-caption text-on-surface-variant">
        GOMAXPROCS {runtime.goMaxProcs}
      </span>
      <span className="text-caption text-on-surface-variant">
        {runtime.goMaxProcs === runtime.numCPU
          ? '— equal unless a container limit or an explicit setting says otherwise (topic 02).'
          : '— GOMAXPROCS differs from NumCPU: an explicit setting or a container CPU limit is in play.'}
      </span>
    </section>
  );
}

/** The typed configuration the process actually holds. */
function EnvironmentFacts({ environment }: { environment: AppInfoEnvironment }) {
  const rows: Array<[string, string]> = [
    ['port', String(environment.port)],
    ['env', environment.env],
    ['logLevel', environment.logLevel],
  ];
  return (
    <section data-testid="ai-environment" className="flex flex-col gap-xxs">
      <p className="text-caption text-on-surface-variant">
        the booted configuration (GDG_PORT / GDG_ENV / GDG_LOG_LEVEL, resolved at startup):
      </p>
      <div className="flex flex-wrap gap-xs">
        {rows.map(([key, value]) => (
          <span
            key={key}
            className="rounded-pill border border-outline px-sm py-xxs font-mono text-caption text-on-surface-variant"
          >
            {key}: <span className="text-secondary">{value}</span>
          </span>
        ))}
      </div>
    </section>
  );
}

/** The toolchain flags stamped into the binary, sorted by key. */
function BuildSettingsTable({ rows }: { rows: Array<AppInfoDemoData['buildSettings'][number]> }) {
  return (
    <section data-testid="ai-settings" className="flex flex-col gap-sm">
      <p className="text-caption text-on-surface-variant">
        build settings the toolchain stamped into this binary ({rows.length} flags, from
        runtime/debug.ReadBuildInfo — an empty value is an unset flag, honestly rendered):
      </p>
      <div className="overflow-x-auto rounded-md border border-outline">
        <table className="w-full text-left font-mono text-caption">
          <thead className="bg-surface-tint text-on-surface-variant">
            <tr>
              <th scope="col" className="px-sm py-xxs">flag</th>
              <th scope="col" className="px-sm py-xxs">value</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.key} className="border-t border-outline">
                <td className="px-sm py-xxs text-on-surface">{row.key}</td>
                <td className="px-sm py-xxs text-secondary">{row.value === '' ? '—' : row.value}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

export default AppInfoDemo;
