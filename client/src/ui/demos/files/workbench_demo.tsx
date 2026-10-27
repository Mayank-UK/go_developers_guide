import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import type { GetFilesWorkbenchDemoUseCase } from '../../../domain/demo/get-files-workbench-demo-usecase';
import { FILES_WORKBENCH_DEMO_USE_CASE } from '../../../domain/demo/get-files-workbench-demo-usecase';
import type { FilesStoryDemoData } from '../../../domain/demo/files-story';

/**
 * FilesWorkbenchDemo — the files topic's `files:workbench` demo: one
 * request renders the server-run story of the whole streaming doctrine
 * (five beats — the streamed upload with its allocation-honesty number,
 * the mid-stream size refusal, the range math with the 416 census, the
 * traversal sweep with its symlink escape, the bounded thumbnail). The
 * story is single-demo (unlike the security topic's shared view), so
 * the hook and rendering live here per the family's single-file
 * component rule. Verdict colors: ok → primary, rejected → error (a
 * refusal IS the success story here), info → muted.
 */

type StoryStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: FilesStoryDemoData }
  | { state: 'error'; failure: Failure };

function verdictClass(verdict: string): string {
  if (verdict === 'ok') return 'text-primary';
  if (verdict === 'rejected') return 'text-error';
  return 'text-on-surface-variant';
}

function useFilesStory(useCase: GetFilesWorkbenchDemoUseCase): {
  status: StoryStatus;
  retry: () => void;
} {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<StoryStatus>({ state: 'idle' });

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

export function FilesWorkbenchDemo() {
  // Resolved per RENDER (the family's hook-default pattern): a retry
  // re-resolves the token, so a spec — or the container — can swap the
  // registration between renders and the demo picks it up.
  const useCase = container.resolve<GetFilesWorkbenchDemoUseCase>(FILES_WORKBENCH_DEMO_USE_CASE);
  const { status, retry } = useFilesStory(useCase);

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="files-workbench-loading" className="text-body-sm text-on-surface-variant">
        streaming an upload, tripping the bound, and sweeping the jail against the real store…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the files workbench demo" failure={status.failure} onRetry={retry} />;
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid="files-workbench-ready">
      <p className="font-mono text-caption text-on-surface-variant">
        mechanism <span className="text-secondary">{data.mechanism}</span> — one request, five
        beats run live against the guide&apos;s streaming store
      </p>

      {data.beats.map((beat) => (
        <section
          key={beat.beat}
          data-testid={`files-workbench-beat-${slug(beat.beat)}`}
          className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
        >
          <p className={`font-mono text-caption ${verdictClass(beat.verdict)}`}>
            <span className="text-secondary">{beat.beat}</span> · {beat.verdict}
          </p>
          <p className="text-caption text-on-surface">{beat.detail}</p>
          {beat.facts.map((fact) => (
            <p key={fact.label} className="font-mono text-caption text-on-surface-variant">
              {fact.label}: <span className="text-on-surface">{fact.value}</span>
            </p>
          ))}
        </section>
      ))}

      <section
        data-testid="files-workbench-proofs"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="font-mono text-caption text-primary">the derived proofs</p>
        {data.proofs.map((proof) => (
          <p key={proof.label} className="text-caption text-on-surface">
            <span className="font-mono text-secondary">{proof.label}</span> — {proof.value}
          </p>
        ))}
      </section>

      <p data-testid="files-workbench-reasoning" className="text-body-sm text-on-surface-variant">
        {data.reasoning}
      </p>
    </div>
  );
}

/** slug turns a beat label into a stable testid fragment. */
function slug(label: string): string {
  return label
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

export default FilesWorkbenchDemo;
