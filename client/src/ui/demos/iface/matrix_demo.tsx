import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetInterfaceMatrixDemoUseCase,
  INTERFACE_MATRIX_DEMO_USE_CASE,
} from '../../../domain/demo/get-interface-matrix-demo-usecase';
import type {
  EmbeddingRow,
  InterfaceMatrixDemoData,
  InterfaceSpec,
  SatisfactionRow,
  SwitchCaseRow,
} from '../../../domain/demo/interface-matrix';

/**
 * InterfaceMatrixDemo — the interfaces-and-composition topic's live demo
 * (demo block id `iface:matrix`). It renders the payload of
 * GET /api/demo/iface/matrix, which the server computes from real method
 * sets: five interface columns (method signatures rendered from the real
 * reflect types), eleven guide and stdlib types crossed against them with
 * every ✓/✗ computed live via reflect Implements — including the
 * pointer-vs-value *core.Error pair and the sealed content.Block rows that
 * satisfy while listing zero exported methods — three embedding exhibits
 * satisfied purely by promotion, a type switch whose case order decides
 * which of two matching cases wins, and the var _ compile proofs that pin
 * the same cells at build time.
 *
 * Dependency flow mirrors the views: the hook resolves its use case from
 * the container (the token's default parameter is the spec seam — tests
 * substitute fakes through the same token), failures arrive as typed
 * Failures inside Results, and `retry` re-runs the load.
 */

type InterfaceMatrixDemoStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: InterfaceMatrixDemoData }
  | { state: 'error'; failure: Failure };

export function useInterfaceMatrixDemo(
  useCase: GetInterfaceMatrixDemoUseCase = container.resolve(INTERFACE_MATRIX_DEMO_USE_CASE),
): { status: InterfaceMatrixDemoStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<InterfaceMatrixDemoStatus>({ state: 'idle' });

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

export function InterfaceMatrixDemo() {
  const { status, retry } = useInterfaceMatrixDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="interface-matrix-loading" className="text-body-sm text-on-surface-variant">
        computing satisfaction verdicts…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the satisfaction-matrix demo" failure={status.failure} onRetry={retry} />;
  }

  const { interfaces, rows, embeddings, typeSwitch, compileProofs } = status.data;

  return (
    <div className="flex flex-col gap-md">
      <section data-testid="im-interfaces" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          The five columns — one method each
        </h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">interface</th>
                <th scope="col" className="px-sm py-xxs">defining method</th>
                <th scope="col" className="px-sm py-xxs">from</th>
              </tr>
            </thead>
            <tbody>
              {interfaces.map((spec) => (
                <InterfaceSpecCells key={spec.name} spec={spec} />
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section data-testid="im-matrix" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          Types × interfaces — every verdict computed live
        </h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">expression</th>
                <th scope="col" className="px-sm py-xxs">kind</th>
                <th scope="col" className="px-sm py-xxs">methods</th>
                {interfaces.map((spec) => (
                  <th key={spec.name} scope="col" className="px-sm py-xxs whitespace-nowrap">
                    {spec.name}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <SatisfactionRowCells key={row.expression} row={row} />
              ))}
            </tbody>
          </table>
        </div>
        <p className="text-body-sm text-on-surface-variant">
          Find the pair: <code className="font-mono">*core.Error</code> is an error,
          the <code className="font-mono">core.Error</code> value is not — pointer
          receivers. And the TextBlock row satisfies content.Block with an empty
          exported-methods list: blockKind is unexported, the seal itself.
        </p>
      </section>

      <section data-testid="im-embeddings" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          Satisfaction by promotion
        </h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">exhibit</th>
                <th scope="col" className="px-sm py-xxs">embeds</th>
                <th scope="col" className="px-sm py-xxs">satisfies</th>
              </tr>
            </thead>
            <tbody>
              {embeddings.map((row) => (
                <EmbeddingRowCells key={row.expression} row={row} />
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section data-testid="im-type-switch" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          The type switch — first match wins
        </h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">input</th>
                <th scope="col" className="px-sm py-xxs">matched case</th>
                <th scope="col" className="px-sm py-xxs">result</th>
              </tr>
            </thead>
            <tbody>
              {typeSwitch.map((row) => (
                <SwitchCaseRowCells key={row.input} row={row} />
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section
        data-testid="im-proofs"
        className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <h4 className="font-display text-title-sm text-on-surface">
          var _ — the same facts, pinned at build time
        </h4>
        <ul className="flex flex-col gap-xxs">
          {compileProofs.map((proof) => (
            <li key={proof.line} className="font-mono text-caption">
              <span className="text-on-surface">{proof.line}</span>
              <span className="text-on-surface-variant">  ·  pins </span>
              <span className="text-secondary">{proof.pins}</span>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}

function InterfaceSpecCells({ spec }: { spec: InterfaceSpec }) {
  return (
    <tr className="border-t border-outline">
      <td className="px-sm py-xxs">
        <div className="text-secondary">{spec.name}</div>
        <div className="text-caption text-on-surface-variant">{spec.note}</div>
      </td>
      <td className="px-sm py-xxs text-on-surface">{spec.method}</td>
      <td className="px-sm py-xxs text-on-surface-variant">{spec.pkg}</td>
    </tr>
  );
}

function SatisfactionRowCells({ row }: { row: SatisfactionRow }) {
  return (
    <tr className="border-t border-outline">
      <td className="px-sm py-xxs">
        <div className="text-on-surface">{row.expression}</div>
        <div className="text-caption text-on-surface-variant">{row.type}</div>
      </td>
      <td className="px-sm py-xxs text-on-surface-variant">{row.kind}</td>
      <td className="px-sm py-xxs text-on-surface-variant">
        {row.methods.length === 0 ? '—' : row.methods.join(', ')}
      </td>
      {row.cells.map((cell) => (
        <td key={cell.iface} className="px-sm py-xxs">
          {cell.satisfied ? (
            <span className="text-secondary">✓</span>
          ) : (
            <span className="text-on-surface-variant">—</span>
          )}
        </td>
      ))}
    </tr>
  );
}

function EmbeddingRowCells({ row }: { row: EmbeddingRow }) {
  return (
    <tr className="border-t border-outline">
      <td className="px-sm py-xxs">
        <div className="text-on-surface">{row.expression}</div>
        <div className="text-caption text-on-surface-variant">{row.note}</div>
      </td>
      <td className="px-sm py-xxs text-on-surface-variant">{row.embeds}</td>
      <td className="px-sm py-xxs">
        <span className="text-on-surface">{row.iface}</span>{' '}
        {row.satisfied ? (
          <span className="text-secondary">✓</span>
        ) : (
          <span className="text-error">✗</span>
        )}
      </td>
    </tr>
  );
}

function SwitchCaseRowCells({ row }: { row: SwitchCaseRow }) {
  return (
    <tr className="border-t border-outline">
      <td className="px-sm py-xxs text-on-surface">{row.input}</td>
      <td className="px-sm py-xxs text-secondary">{row.matchedCase}</td>
      <td className="px-sm py-xxs">
        <div className="text-on-surface-variant">{row.result}</div>
        {row.note !== undefined && (
          <div className="text-caption text-on-surface-variant">{row.note}</div>
        )}
      </td>
    </tr>
  );
}

export default InterfaceMatrixDemo;
