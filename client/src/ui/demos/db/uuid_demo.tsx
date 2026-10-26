import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import { GetUUIDLabDemoUseCase, DB_UUID_DEMO_USE_CASE } from '../../../domain/demo/get-uuid-lab-demo-usecase';
import type { UUIDLabDemoData } from '../../../domain/demo/uuid-lab';

/**
 * UUIDLabDemo — the sql-schema topic's ids demo (demo block id
 * `db:uuid`). It renders the payload of GET /api/demo/db/uuid: the
 * guide's hand-rolled UUIDv7 generator's field-by-field layout, one
 * freshly minted value dissected (timestamp, version, variant), the
 * ordered batch beside its lexically sorted copy with the verdict badge,
 * the 10,000-id collision check, and the bigserial / uuidv7 / uuidv4
 * comparison table with PostgreSQL's own gen_random_uuid() in the last
 * row.
 *
 * Every load mints a FRESH batch — values change, properties never: the
 * view asserts nothing about specific uuid literals, and neither should
 * anyone reading it. `retry` re-mints, which is half the fun.
 */

type UUIDLabStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: UUIDLabDemoData }
  | { state: 'error'; failure: Failure };

export function useUUIDLabDemo(
  useCase: GetUUIDLabDemoUseCase = container.resolve(DB_UUID_DEMO_USE_CASE),
): { status: UUIDLabStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<UUIDLabStatus>({ state: 'idle' });

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

export function UUIDLabDemo() {
  const { status, retry } = useUUIDLabDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="uuid-lab-loading" className="text-body-sm text-on-surface-variant">
        minting a fresh batch…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the uuid lab" failure={status.failure} onRetry={retry} />;
  }

  const { generator, layout, anatomy, batch, collision, comparison } = status.data;
  const sortedVerdict = batch.sortingMatchesGenerationOrder;

  return (
    <div className="flex flex-col gap-md" data-testid="uuid-lab-ready">
      <section data-testid="ul-anatomy" className="flex flex-col gap-xs rounded-md border border-outline bg-surface-tint px-md py-sm">
        <h4 className="font-display text-title-sm text-on-surface">One fresh value, dissected</h4>
        <p className="font-mono text-body-sm break-all text-on-surface">{anatomy.value}</p>
        <p className="font-mono text-caption text-on-surface-variant">
          unix_ms {anatomy.unixMs} · ver {anatomy.version} · {anatomy.variant} · {anatomy.length} chars
        </p>
        <p className="text-caption text-on-surface-variant">{generator.description}</p>
      </section>

      <section data-testid="ul-layout" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">The 128 bits, field by field</h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">field</th>
                <th scope="col" className="px-sm py-xxs">bits</th>
                <th scope="col" className="px-sm py-xxs">contents</th>
              </tr>
            </thead>
            <tbody>
              {layout.map((row) => (
                <tr key={row.field} className="border-t border-outline">
                  <td className="px-sm py-xxs text-on-surface">{row.field}</td>
                  <td className="px-sm py-xxs text-secondary">{row.bits}</td>
                  <td className="px-sm py-xxs text-on-surface-variant">{row.contents}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section data-testid="ul-batch" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">
          Mint order vs lexical sort — {batch.count} ids
        </h4>
        <p data-testid="ul-sorted-verdict" className={`text-body-sm ${sortedVerdict ? 'text-secondary' : 'text-error'}`}>
          {sortedVerdict
            ? 'Sorted lexically == minted order, member for member — time order IS lexical order.'
            : 'Mismatch — the monotonicity contract broke!'}
        </p>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">#</th>
                <th scope="col" className="px-sm py-xxs">generated</th>
                <th scope="col" className="px-sm py-xxs">sorted</th>
              </tr>
            </thead>
            <tbody>
              {batch.generatedInOrder.map((value, index) => (
                <tr key={value} className="border-t border-outline">
                  <td className="px-sm py-xxs text-on-surface-variant">{index + 1}</td>
                  <td className="px-sm py-xxs text-on-surface">{value}</td>
                  <td className={`px-sm py-xxs ${batch.sortedLexically[index] === value ? 'text-on-surface-variant' : 'text-error'}`}>
                    {batch.sortedLexically[index]}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section data-testid="ul-collision" className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm">
        <h4 className="font-display text-title-sm text-on-surface">Collision check at volume</h4>
        <p className="font-mono text-body-sm text-on-surface">
          {collision.distinct} distinct of {collision.generated} minted —{' '}
          <span className={collision.collisionFree ? 'text-secondary' : 'text-error'}>
            {collision.collisionFree ? 'collision-free' : 'COLLISIONS FOUND'}
          </span>
        </p>
        <p className="text-caption text-on-surface-variant">
          {collision.randomnessBits} random bits per millisecond slot ({generator.randomSource}).
        </p>
      </section>

      <section data-testid="ul-comparison" className="flex flex-col gap-xs">
        <h4 className="font-display text-title-sm text-on-surface">The id-strategy trade-off</h4>
        <div className="overflow-x-auto rounded-md border border-outline">
          <table className="w-full text-left font-mono text-caption">
            <thead className="bg-surface-tint text-on-surface-variant">
              <tr>
                <th scope="col" className="px-sm py-xxs">strategy</th>
                <th scope="col" className="px-sm py-xxs">storage</th>
                <th scope="col" className="px-sm py-xxs">index locality</th>
                <th scope="col" className="px-sm py-xxs">URLs</th>
                <th scope="col" className="px-sm py-xxs">scope</th>
              </tr>
            </thead>
            <tbody>
              {comparison.map((row) => (
                <tr key={row.strategy} className="border-t border-outline">
                  <td className="px-sm py-xxs text-on-surface">{row.strategy}</td>
                  <td className="px-sm py-xxs text-secondary">{row.storage}</td>
                  <td className="px-sm py-xxs text-on-surface-variant">{row.indexLocality}</td>
                  <td className="px-sm py-xxs text-on-surface-variant">{row.urlGuessability}</td>
                  <td className="px-sm py-xxs text-on-surface-variant">{row.uniquenessScope}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <ul className="flex flex-col gap-xxs">
          {comparison.map((row) => (
            <li key={row.strategy} className="text-body-sm text-on-surface-variant">
              <span className="font-mono text-on-surface">{row.strategy}</span> — {row.notes}
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}

export default UUIDLabDemo;
