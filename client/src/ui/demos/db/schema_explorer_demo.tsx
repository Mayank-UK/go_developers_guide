import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import {
  GetSchemaExplorerDemoUseCase,
  SCHEMA_EXPLORER_DEMO_USE_CASE,
} from '../../../domain/demo/get-schema-explorer-demo-usecase';
import type {
  SchemaExplorerColumn,
  SchemaExplorerConstraint,
  SchemaExplorerDemoData,
  SchemaExplorerIndex,
  SchemaExplorerTable,
} from '../../../domain/demo/schema-explorer';

/**
 * SchemaExplorerDemo — the sql-schema topic's live demo (demo block id
 * `db:schema-explorer`). It renders the payload of GET
 * /api/demo/db/schema-explorer: the live PostgreSQL's own account of the
 * guide's schema — every public table with its columns in declaration
 * order and their real format_type spellings, every constraint with the
 * database's own pg_get_constraintdef rendering (ON DELETE behavior
 * included), every index with its access method and columns, goose's
 * migration ledger, and the curated rationale notes.
 *
 * The introspected half is the catalog's truth, re-read on every load;
 * if the schema and the topic prose ever disagree, this panel shows the
 * schema's side. An unreachable database degrades to the typed error
 * panel (the server's honest 503), never to a stale answer.
 */

type SchemaExplorerStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: SchemaExplorerDemoData }
  | { state: 'error'; failure: Failure };

export function useSchemaExplorerDemo(
  useCase: GetSchemaExplorerDemoUseCase = container.resolve(SCHEMA_EXPLORER_DEMO_USE_CASE),
): { status: SchemaExplorerStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<SchemaExplorerStatus>({ state: 'idle' });

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

export function SchemaExplorerDemo() {
  const { status, retry } = useSchemaExplorerDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="schema-explorer-loading" className="text-body-sm text-on-surface-variant">
        asking the catalog what it knows…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the schema explorer" failure={status.failure} onRetry={retry} />;
  }

  const { tables, constraints, indexes, history, notes } = status.data;

  return (
    <div className="flex flex-col gap-md" data-testid="schema-explorer-ready">
      <section data-testid="se-migrations" className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm">
        <h4 className="font-display text-title-sm text-on-surface">Migration ledger (goose_db_version)</h4>
        <div className="flex flex-wrap gap-xxs">
          {history.map((row) => (
            <span
              key={`${row.version}-${row.appliedAt}`}
              className="rounded border border-outline px-xs py-xxs font-mono text-caption text-on-surface-variant"
            >
              v{row.version} {row.applied ? '✓ applied' : '✗'} — {row.appliedAt}
            </span>
          ))}
        </div>
        <p className="text-caption text-on-surface-variant">
          Version 0 is goose's own baseline row — the table that makes re-running Up a no-op.
        </p>
      </section>

      <section data-testid="se-tables" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          Tables, columns, and real types ({tables.length})
        </h4>
        <div className="flex flex-col gap-sm">
          {tables.map((table) => (
            <TableCard key={table.name} table={table} />
          ))}
        </div>
      </section>

      <section data-testid="se-constraints" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          Constraints, as pg_get_constraintdef states them ({constraints.length})
        </h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">table</th>
                <th scope="col" className="px-sm py-xxs">name</th>
                <th scope="col" className="px-sm py-xxs">definition</th>
              </tr>
            </thead>
            <tbody>
              {constraints.map((constraint) => (
                <ConstraintCells key={constraint.name} constraint={constraint} />
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section data-testid="se-indexes" className="flex flex-col gap-sm">
        <h4 className="font-display text-title-sm text-on-surface">
          Indexes, kinds and all — primary-key backing included ({indexes.length})
        </h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">table</th>
                <th scope="col" className="px-sm py-xxs">index</th>
                <th scope="col" className="px-sm py-xxs">kind</th>
                <th scope="col" className="px-sm py-xxs">columns</th>
              </tr>
            </thead>
            <tbody>
              {indexes.map((index) => (
                <IndexCells key={index.name} index={index} />
              ))}
            </tbody>
          </table>
        </div>
        <p className="text-body-sm text-on-surface-variant">
          PostgreSQL never auto-indexes foreign key columns — every index here was a decision.
        </p>
      </section>

      {notes.length > 0 ? (
        <section data-testid="se-notes" className="flex flex-col gap-xs">
          <h4 className="font-display text-title-sm text-on-surface">Why it looks like this</h4>
          <ul className="flex flex-col gap-xxs">
            {notes.map((note) => (
              <li key={note.subject} className="text-body-sm text-on-surface-variant">
                <span className="font-mono text-on-surface">{note.subject}</span> — {note.note}
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </div>
  );
}

function TableCard({ table }: { table: SchemaExplorerTable }) {
  return (
    <div className="flex flex-col gap-xs rounded-md border border-outline px-md py-sm">
      <div className="flex flex-wrap items-baseline justify-between gap-xs">
        <h5 className="font-mono text-title-sm text-on-surface">{table.name}</h5>
        <span className="font-mono text-caption text-secondary">{table.columns.length} columns</span>
      </div>
      {table.purpose ? (
        <p className="text-body-sm text-on-surface-variant">{table.purpose}</p>
      ) : null}
      <div className="overflow-x-auto">
        <table className="w-full text-left font-mono text-caption">
          <thead className="text-on-surface-variant">
            <tr>
              <th scope="col" className="pr-sm py-xxs">column</th>
              <th scope="col" className="pr-sm py-xxs">type</th>
              <th scope="col" className="pr-sm py-xxs">null</th>
              <th scope="col" className="py-xxs">default</th>
            </tr>
          </thead>
          <tbody>
            {table.columns.map((column) => (
              <ColumnCells key={column.name} column={column} />
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function ColumnCells({ column }: { column: SchemaExplorerColumn }) {
  return (
    <tr className="border-t border-outline">
      <td className="pr-sm py-xxs text-on-surface">{column.name}</td>
      <td className="pr-sm py-xxs text-secondary">{column.type}</td>
      <td className={`pr-sm py-xxs ${column.nullable ? 'text-error' : 'text-on-surface-variant'}`}>
        {column.nullable ? 'NULL' : 'NOT NULL'}
      </td>
      <td className="py-xxs text-on-surface-variant">{column.default || '—'}</td>
    </tr>
  );
}

function ConstraintCells({ constraint }: { constraint: SchemaExplorerConstraint }) {
  const onDelete = /ON DELETE (CASCADE|SET NULL|RESTRICT)/.exec(constraint.definition)?.[1];
  return (
    <tr className="border-t border-outline">
      <td className="px-sm py-xxs text-on-surface">{constraint.table}</td>
      <td className="px-sm py-xxs text-secondary">{constraint.name}</td>
      <td className="px-sm py-xxs text-on-surface-variant">
        {constraint.definition}
        {onDelete ? (
          <span className={`ml-xs rounded px-xs ${onDelete === 'SET NULL' ? 'bg-secondary text-surface' : 'bg-surface-tint text-secondary'}`}>
            {onDelete}
          </span>
        ) : null}
      </td>
    </tr>
  );
}

function IndexCells({ index }: { index: SchemaExplorerIndex }) {
  return (
    <tr className="border-t border-outline">
      <td className="px-sm py-xxs text-on-surface">{index.table}</td>
      <td className="px-sm py-xxs text-secondary">
        {index.name}
        {index.primary ? <span className="ml-xs rounded bg-surface-tint px-xs text-on-surface-variant">PK</span> : null}
        {index.unique && !index.primary ? <span className="ml-xs rounded bg-surface-tint px-xs text-on-surface-variant">unique</span> : null}
      </td>
      <td className="px-sm py-xxs text-on-surface-variant">{index.kind}</td>
      <td className="px-sm py-xxs text-on-surface-variant">({index.columns.join(', ')})</td>
    </tr>
  );
}

export default SchemaExplorerDemo;
