import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetDepsManifestDemoUseCase,
  DEPS_MANIFEST_DEMO_USE_CASE,
} from '../../../domain/demo/get-deps-manifest-demo-usecase';
import type {
  DepsManifestDemoData,
  DepsRequireRow,
} from '../../../domain/demo/deps-manifest';

/**
 * DepsManifestDemo — the dependency-management topic's live demo
 * (demo block id `deps:manifest`). It renders the payload of GET
 * /api/demo/deps/manifest: the manifest panel (this server's real
 * go.mod parsed live — module, go directive, direct requires with
 * their pinned versions and linked flags, the indirect count, the
 * tool directives) beside the build panel (the serving binary's own
 * debug.ReadBuildInfo), the tools/deps.go anchor rows, the Docker
 * cache volumes, and the govulncheck panel — live when the tool ran
 * for real (reload once if it answers "running": the scan is
 * backgrounded), a clearly-labeled example when it could not.
 */

type ManifestStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: DepsManifestDemoData }
  | { state: 'error'; failure: Failure };

export function useDepsManifestDemo(
  useCase: GetDepsManifestDemoUseCase = container.resolve(DEPS_MANIFEST_DEMO_USE_CASE),
): { status: ManifestStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<ManifestStatus>({ state: 'idle' });

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

export function DepsManifestDemo() {
  const { status, retry } = useDepsManifestDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="deps-manifest-loading" className="text-body-sm text-on-surface-variant">
        reading the module manifest beside the running binary…
      </p>
    );
  }

  if (status.state === 'error') {
    return (
      <ErrorPanel subject="the deps manifest demo" failure={status.failure} onRetry={retry} />
    );
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid="deps-manifest-ready">
      <section
        data-testid="deps-manifest-gomod"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-primary">
          {data.goMod.file}
          {data.goMod.available ? (
            <>
              {' '}
              — module {data.goMod.module} · go {data.goMod.goDirective}
            </>
          ) : (
            ''
          )}
        </p>
        {data.goMod.available ? (
          <>
            <p className="text-caption text-on-surface">
              {String(data.goMod.directRequires.length)} direct requires ·{' '}
              {String(data.goMod.indirectCount)} indirect — every version read from the live
              file, never restated
            </p>
            {data.goMod.directRequires.map((row) => (
              <RequireRow key={row.path} row={row} />
            ))}
            <p className="font-mono text-caption text-on-surface-variant">
              tool directives: {data.goMod.toolDirectives.join(' · ')}
            </p>
          </>
        ) : (
          <p className="text-caption text-on-surface">{data.goMod.note}</p>
        )}
      </section>

      <section
        data-testid="deps-manifest-build"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <p className="font-mono text-caption text-on-surface">
          <span className="text-secondary">{data.build.modulePath}</span>{' '}
          {data.build.mainVersion} · {data.build.goVersion} · links{' '}
          {String(data.build.linkedCount)} modules
        </p>
        <p className="text-caption text-on-surface">{data.build.note}</p>
      </section>

      <section
        data-testid="deps-manifest-anchors"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <p className="font-mono text-caption text-on-surface">
          <span className="text-secondary">{data.anchors.file}</span> — build tag{' '}
          {data.anchors.buildTag}
        </p>
        {data.anchors.available ? (
          data.anchors.anchors.map((row) => <RequireRow key={row.path} row={row} />)
        ) : (
          <p className="text-caption text-on-surface">{data.anchors.note}</p>
        )}
        <p className="text-caption text-on-surface-variant">{data.anchors.note}</p>
      </section>

      <section
        data-testid="deps-manifest-cache"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        {data.cache.volumes.map((volume) => (
          <p key={volume.name} className="font-mono text-caption text-on-surface">
            <span className="text-secondary">{volume.name}</span> → {volume.mount}
          </p>
        ))}
        <p className="text-caption text-on-surface">{data.cache.layerOrderNote}</p>
        <p className="text-caption text-on-surface-variant">{data.cache.externalNote}</p>
      </section>

      <section
        data-testid="deps-manifest-vulncheck"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <p className="font-mono text-caption text-on-surface">
          <span className="text-secondary">{data.vulncheck.command}</span> — mode{' '}
          {data.vulncheck.mode}
          {data.vulncheck.mode === 'live' ? (
            <>
              {' '}
              · exit {String(data.vulncheck.exitCode)} in {String(data.vulncheck.elapsedMs)} ms
            </>
          ) : (
            ''
          )}
        </p>
        {data.vulncheck.summary ? (
          <pre className="overflow-x-auto rounded-sm bg-surface-tint px-sm py-xxs font-mono text-caption text-on-surface">
            {data.vulncheck.summary}
          </pre>
        ) : null}
        {data.vulncheck.fixture ? (
          <pre className="overflow-x-auto rounded-sm bg-surface-tint px-sm py-xxs font-mono text-caption text-on-surface-variant">
            {data.vulncheck.fixture}
          </pre>
        ) : null}
        <p className="text-caption text-on-surface">{data.vulncheck.note}</p>
      </section>

      <p data-testid="deps-manifest-reasoning" className="text-body-sm text-on-surface-variant">
        {data.reasoning}
      </p>
    </div>
  );
}

function RequireRow({ row }: { row: DepsRequireRow }) {
  return (
    <p className="font-mono text-caption text-on-surface">
      <span className="text-secondary">{row.path}</span> {row.version}{' '}
      <span className={row.linked ? 'text-primary' : 'text-on-surface-variant'}>
        {row.linked ? 'linked in this binary' : 'not linked in this binary'}
      </span>
    </p>
  );
}

export default DepsManifestDemo;
