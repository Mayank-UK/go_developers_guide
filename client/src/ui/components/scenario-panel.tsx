import { useCallback, useEffect, useRef, useState } from 'react';
import { container } from '../../core/container';
import type {
  DataMode,
  InjectFailure,
  ScenarioPatch,
  ScenarioState,
} from '../../domain/scenario/scenario';
import type { GetScenarioUseCase } from '../../domain/scenario/get-scenario-usecase';
import type { PatchScenarioUseCase } from '../../domain/scenario/patch-scenario-usecase';

/**
 * ScenarioPanel — the shared demo-scenario control, embedded inline by the
 * demos that drive the scenario store (errors:explorer, reliability:breaker):
 * two compact selects (data mode, failure injection) backed by GET/PATCH
 * /api/demo/scenario. It is deliberately
 * impossible for this panel to break the shell: a failed initial GET degrades
 * to a muted "scenario unavailable" note, a failed PATCH reverts the select to
 * the last server-confirmed state and shows a brief inline error, and the
 * selects disable while a PATCH is in flight so a second change can never
 * interleave with a pending round (an in-flight guard enforces the same rule
 * even if an event slips through). PATCHes fire only on user change — never
 * on mount — so React StrictMode's dev double-mount issues two idempotent
 * GETs and zero double PATCHes.
 */

/** What the panel renders for the snapshot itself. */
export type ScenarioPanelStatus =
  | { state: 'loading' }
  | { state: 'ready'; scenario: ScenarioState }
  | { state: 'unavailable' };

export interface ScenarioPanelModel {
  status: ScenarioPanelStatus;
  /** True while a PATCH round is in flight (selects disable). */
  patching: boolean;
  /** A failed PATCH's user-facing message; null when the last round succeeded. */
  error: string | null;
  /** Optimistically apply a change, then confirm or revert on the round's outcome. */
  update: (patch: ScenarioPatch) => void;
}

/**
 * useScenarioPanel — loads the scenario snapshot once per mount and narrows
 * the Result into render statuses. The use cases are injected as default
 * parameters (react-guide `usePosts` style) so specs can substitute fakes;
 * production resolves them from the container, never by importing data/.
 */
export function useScenarioPanel(
  getScenarioUseCase: GetScenarioUseCase = container.getScenarioUseCase,
  patchScenarioUseCase: PatchScenarioUseCase = container.patchScenarioUseCase,
): ScenarioPanelModel {
  const [status, setStatus] = useState<ScenarioPanelStatus>({ state: 'loading' });
  const [patching, setPatching] = useState(false);
  const [error, setError] = useState<string | null>(null);
  /** The last server-confirmed state — what a failed PATCH reverts to. */
  const lastGood = useRef<ScenarioState>({ dataMode: 'fake', injectFailure: 'none' });
  /** Hard guard: only one PATCH round may be in flight at a time. */
  const patchInFlight = useRef(false);
  /** False once the panel unmounts — pending rounds must not setState. */
  const alive = useRef(true);

  useEffect(() => {
    alive.current = true;
    let cancelled = false;

    void getScenarioUseCase.execute().then((result) => {
      if (cancelled || !alive.current) return;
      if (result.ok) {
        lastGood.current = result.value;
        setStatus({ state: 'ready', scenario: result.value });
      } else {
        setStatus({ state: 'unavailable' });
      }
    });

    return () => {
      cancelled = true;
      alive.current = false;
    };
  }, [getScenarioUseCase]);

  const update = useCallback(
    (patch: ScenarioPatch) => {
      if (patchInFlight.current) return;
      patchInFlight.current = true;
      setError(null);
      // Optimistic: show the user's pick immediately; the round either
      // confirms it or reverts below.
      setStatus((current) =>
        current.state === 'ready'
          ? { state: 'ready', scenario: { ...current.scenario, ...patch } }
          : current,
      );
      setPatching(true);

      void patchScenarioUseCase.execute(patch).then((result) => {
        if (!alive.current) return;
        patchInFlight.current = false;
        setPatching(false);
        if (result.ok) {
          lastGood.current = result.value;
          setStatus({ state: 'ready', scenario: result.value });
        } else {
          setStatus({ state: 'ready', scenario: lastGood.current });
          setError(result.failure.message);
        }
      });
    },
    [patchScenarioUseCase],
  );

  return { status, patching, error, update };
}

const SELECT_CLASSES =
  'rounded-md border border-outline bg-surface px-sm py-xxs font-mono text-body-sm text-on-surface-variant hover:border-primary hover:text-on-surface disabled:cursor-not-allowed disabled:opacity-50';

export function ScenarioPanel() {
  const { status, patching, error, update } = useScenarioPanel();

  if (status.state === 'loading') {
    return (
      <span
        data-testid="scenario-loading"
        aria-hidden
        className="font-mono text-caption text-on-surface-variant/60"
      >
        scenario…
      </span>
    );
  }

  if (status.state === 'unavailable') {
    return (
      <span
        data-testid="scenario-unavailable"
        title="The demo scenario API is not reachable right now."
        className="font-mono text-caption text-on-surface-variant/60"
      >
        scenario unavailable
      </span>
    );
  }

  return (
    <div className="flex items-center gap-xs" data-testid="scenario-panel">
      <select
        aria-label="Data mode"
        title="Which backing store the demos answer from"
        disabled={patching}
        value={status.scenario.dataMode}
        onChange={(event) => {
          update({ dataMode: event.target.value as DataMode });
        }}
        className={SELECT_CLASSES}
      >
        <option value="fake">fake data</option>
        <option value="live">live data</option>
      </select>
      <select
        aria-label="Inject failure"
        title="Which failure path the demos inject"
        disabled={patching}
        value={status.scenario.injectFailure}
        onChange={(event) => {
          update({ injectFailure: event.target.value as InjectFailure });
        }}
        className={SELECT_CLASSES}
      >
        <option value="none">no failure</option>
        <option value="error">error</option>
        <option value="latency">latency</option>
      </select>
      {error !== null && (
        <span
          role="status"
          data-testid="scenario-error"
          title={error}
          className="font-mono text-caption text-error"
        >
          update failed
        </span>
      )}
    </div>
  );
}
