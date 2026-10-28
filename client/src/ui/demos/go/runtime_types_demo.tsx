import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import type { GetRuntimeTypesDemoUseCase } from '../../../domain/demo/get-runtime-types-demo-usecase';
import type {
  AssertionRow,
  RuntimeTypesDemoData,
  TypeRow,
  ZeroValueRow,
} from '../../../domain/demo/runtime-types';

/**
 * RuntimeTypesDemo — the go-language topic's live demo (demo block id
 * `go:runtime-types`). It renders the payload of
 * GET /api/demo/go/runtime-types, which the server computes from this
 * repo's real objects: reflect.TypeOf/Kind/PkgPath walks over nine values,
 * the Section struct's fields with their runtime-readable JSON tags, what
 * one []content.Block slot holds per element (plus a comma-ok miss that
 * reports false instead of panicking), declared-and-never-initialized zero
 * values confirmed by reflect's IsZero, the compiler's verbatim rejection
 * of a wrong-typed call (the one beat that cannot RUN, by design), and
 * encoding/json refusing a number where a string field is declared.
 *
 * Dependency flow mirrors the views: the hook resolves the use case from
 * the container (its default parameter is the spec seam — tests substitute
 * fakes through the container token instead), failures arrive as typed
 * Failures inside Results, and `retry` re-runs the load.
 */

type RuntimeTypesDemoStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: RuntimeTypesDemoData }
  | { state: 'error'; failure: Failure };

export function useRuntimeTypesDemo(
  useCase: GetRuntimeTypesDemoUseCase = container.getRuntimeTypesDemoUseCase,
): { status: RuntimeTypesDemoStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<RuntimeTypesDemoStatus>({ state: 'idle' });

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

export function RuntimeTypesDemo() {
  const { status, retry } = useRuntimeTypesDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="runtime-types-loading" className="text-body-sm text-on-surface-variant">
        computing runtime types…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the runtime-types demo" failure={status.failure} onRetry={retry} />;
  }

  const {
    inspections,
    fields,
    assertions,
    zeroValues,
    enforcement,
    boundary,
    fieldsNote,
    assertionsNote,
  } = status.data;

  return (
    <div className="flex flex-col gap-md">
      <section data-testid="rt-inspections" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          reflect.TypeOf — what the runtime knows
        </h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">expression</th>
                <th scope="col" className="px-sm py-xxs">type</th>
                <th scope="col" className="px-sm py-xxs">kind</th>
                <th scope="col" className="px-sm py-xxs">pkgPath</th>
                <th scope="col" className="px-sm py-xxs">size</th>
              </tr>
            </thead>
            <tbody>
              {inspections.map((row) => (
                <TypeRowCells key={row.expression} row={row} />
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section data-testid="rt-fields" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          Struct metadata on the wire — content.Section, field by field
        </h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">field</th>
                <th scope="col" className="px-sm py-xxs">type</th>
                <th scope="col" className="px-sm py-xxs">json tag</th>
              </tr>
            </thead>
            <tbody>
              {fields.map((field) => (
                <tr key={field.name} className="border-t border-outline">
                  <td className="px-sm py-xxs text-on-surface">{field.name}</td>
                  <td className="px-sm py-xxs text-on-surface-variant">{field.type}</td>
                  <td className="px-sm py-xxs text-secondary">{field.jsonTag}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="text-body-sm text-on-surface-variant">
          {fieldsNote}
        </p>
      </section>

      <section data-testid="rt-assertions" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          What one []content.Block slot holds
        </h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">assertion</th>
                <th scope="col" className="px-sm py-xxs">dynamic type</th>
                <th scope="col" className="px-sm py-xxs">verdict</th>
                <th scope="col" className="px-sm py-xxs">block id</th>
              </tr>
            </thead>
            <tbody>
              {assertions.map((row) => (
                <AssertionRowCells key={row.expression} row={row} />
              ))}
            </tbody>
          </table>
        </div>
        <p className="text-body-sm text-on-surface-variant">
          {assertionsNote}
        </p>
      </section>

      <section data-testid="rt-zero-values" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          Declared, never initialized
        </h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">declaration</th>
                <th scope="col" className="px-sm py-xxs">type</th>
                <th scope="col" className="px-sm py-xxs">zero value</th>
                <th scope="col" className="px-sm py-xxs">reflect</th>
              </tr>
            </thead>
            <tbody>
              {zeroValues.map((row) => (
                <ZeroValueRowCells key={row.declaration} row={row} />
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section
        data-testid="rt-enforcement"
        className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <h4 className="font-display text-title-sm text-on-surface">It never compiles</h4>
        <p className="font-mono text-body-sm text-on-surface">
          {enforcement.signature}
        </p>
        <p className="font-mono text-body-sm">
          <span className="text-on-surface">{enforcement.call}</span>
          <span className="text-on-surface-variant"> → </span>
          <span data-testid="rt-verdict" className="text-error">
            build failed
          </span>
        </p>
        <p data-testid="rt-compiler-verdict" className="font-mono text-caption text-error">
          {enforcement.compilerVerdict}
        </p>
        <p className="text-body-sm text-on-surface-variant">
          {enforcement.runtimeOutcome}
        </p>
      </section>

      <section
        data-testid="rt-boundary"
        className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <h4 className="font-display text-title-sm text-on-surface">
          The boundary enforces — encoding/json
        </h4>
        <p className="text-body-sm text-on-surface">
          json.Unmarshal into <code className="font-mono">{boundary.decodeTarget}</code> with{' '}
          <code className="font-mono">{boundary.badInput}</code> →{' '}
          <span className="font-mono text-error">{boundary.outcome}</span>
        </p>
        <p className="font-mono text-caption text-on-surface-variant">
          {boundary.errorType} · {boundary.message}
        </p>
      </section>
    </div>
  );
}

function TypeRowCells({ row }: { row: TypeRow }) {
  return (
    <tr className="border-t border-outline">
      <td className="px-sm py-xxs text-on-surface">{row.expression}</td>
      <td className="px-sm py-xxs text-secondary">{row.type}</td>
      <td className="px-sm py-xxs text-on-surface-variant">{row.kind}</td>
      <td className="px-sm py-xxs text-on-surface-variant">
        {row.pkgPath === '' ? '—' : row.pkgPath}
      </td>
      <td className="px-sm py-xxs text-on-surface-variant">{row.sizeBytes}</td>
    </tr>
  );
}

function AssertionRowCells({ row }: { row: AssertionRow }) {
  return (
    <tr className="border-t border-outline">
      <td className="px-sm py-xxs text-on-surface">{row.expression}</td>
      <td className="px-sm py-xxs text-secondary">{row.dynamicType}</td>
      <td className="px-sm py-xxs">
        {row.ok ? (
          <span className="text-secondary">matched</span>
        ) : (
          <span className="text-error">ok=false</span>
        )}
      </td>
      <td className="px-sm py-xxs text-on-surface-variant">
        {row.blockID === '' ? '—' : row.blockID}
      </td>
    </tr>
  );
}

function ZeroValueRowCells({ row }: { row: ZeroValueRow }) {
  return (
    <tr className="border-t border-outline">
      <td className="px-sm py-xxs text-on-surface">{row.declaration}</td>
      <td className="px-sm py-xxs text-on-surface-variant">{row.type}</td>
      <td className="px-sm py-xxs text-secondary">{row.display}</td>
      <td className="px-sm py-xxs">
        {row.isZero ? (
          <span className="text-secondary">IsZero() = true</span>
        ) : (
          <span className="text-error">IsZero() = false</span>
        )}
      </td>
    </tr>
  );
}

export default RuntimeTypesDemo;
