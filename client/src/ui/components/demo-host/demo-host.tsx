import { Suspense } from 'react';
import { Example } from '../example';
import { demoRegistry, type DemoRegistry } from '../../demos/demo-registry';

export interface DemoHostProps {
  demoId: string;
  title: string;
  description?: string;
  /**
   * The id → lazy-component map to resolve against. Defaults to the live
   * registry; specs substitute a fixture registry (the live one is empty until
   * Phase 3's first demos land).
   */
  registry?: DemoRegistry;
}

/**
 * DemoHost — renders the Example panel for a demo block and mounts the
 * registered component inside it, lazy-loading its chunk behind a local
 * Suspense fallback. Unknown demo ids (content shipped before the demo landed,
 * or an id typo) render the panel with a friendly "not wired yet" note —
 * never an error, never a crash.
 */
export function DemoHost({ demoId, title, description, registry = demoRegistry }: DemoHostProps) {
  const Demo = registry[demoId];

  return (
    <Example title={title} description={description}>
      {Demo === undefined ? (
        <div
          data-testid="demo-not-wired"
          className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm"
        >
          <p className="font-mono text-body-sm text-on-surface-variant">
            demo not wired yet — this panel appears when its interactive demo lands.
          </p>
          <p className="font-mono text-caption text-on-surface-variant/70">
            demoId: {demoId}
          </p>
        </div>
      ) : (
        <Suspense
          fallback={
            <p data-testid="demo-loading" className="text-body-sm text-on-surface-variant">
              loading demo…
            </p>
          }
        >
          <Demo />
        </Suspense>
      )}
    </Example>
  );
}
