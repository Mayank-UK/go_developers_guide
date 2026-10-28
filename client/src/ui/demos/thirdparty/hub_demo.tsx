import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import type {
  ThirdPartyRunRow,
  ThirdPartyHubDemoData,
  ThirdPartyVendorPanel,
} from '../../../domain/demo/thirdparty-hub';
import {
  GetThirdPartyHubDemoUseCase,
  THIRD_PARTY_HUB_DEMO_USE_CASE,
} from '../../../domain/demo/get-thirdparty-hub-demo-usecase';

/**
 * ThirdPartyHubDemo — the third-party topic's adapter-surface hub
 * (demo block id `thirdparty:hub`). It renders the payload of
 * GET /api/demo/thirdparty/hub: the DI switch's current position and
 * per-vendor rows (which adapter the shared scenario store's data mode
 * selects, with one observed probe through the ACTIVE adapter), the
 * three vendor panels — the port with its compile-time-asserted
 * implementations, the sandbox's observed script rows with every
 * failure classified onto the taxonomy, the counted totals, and the
 * live skeleton's honest refusal — the vendor-error taxonomy map, and
 * the honesty note. The sandbox runs are deterministic, so the replay
 * button re-serves identical vendor panels; only the wiring panel
 * moves when the scenario is PATCHed. The client never teaches; it
 * renders what the server observed.
 */

type PanelStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: ThirdPartyHubDemoData }
  | { state: 'error'; failure: Failure };

/** outcomeTone colors an observed outcome for scanning. */
function outcomeTone(outcome: string): string {
  if (outcome.startsWith('accepted') || outcome.startsWith('verified') || outcome.startsWith('charged')) {
    return 'text-primary';
  }
  if (outcome.startsWith('refused') || outcome.startsWith('rejected')) return 'text-error';
  if (outcome.startsWith('deduped')) return 'text-on-surface';
  return 'text-on-surface-variant';
}

/** classificationLabel renders a failure row's taxonomy fields compactly. */
function classificationLabel(row: ThirdPartyRunRow): string {
  if (row.kind === undefined) return '';
  const vendor = row.vendorCode ? `${row.vendorCode} (vendor ${row.vendorStatus ?? '?'})` : 'no vendor code';
  return `${vendor} → ${row.kind} ${row.httpStatus ?? ''}${row.retryable ? ' · retryable' : ' · permanent'}`.trim();
}

/** One observed script row: the call, the outcome, the classification, the detail. */
function RunLine({ row }: { row: ThirdPartyRunRow }) {
  return (
    <div className="flex flex-col gap-xxs" data-testid={`tp-row-${row.step}`}>
      <p className="text-body-sm text-on-surface">
        <span className="font-mono text-caption text-on-surface-variant">{row.step}.</span>{' '}
        <span className="font-mono text-caption">{row.call}</span>{' '}
        <span className={`font-mono text-caption uppercase ${outcomeTone(row.outcome)}`}>{row.outcome}</span>
      </p>
      <p className="text-caption text-on-surface-variant pl-md">{row.input}</p>
      <p className="font-mono text-caption text-on-surface-variant pl-md">{row.detail}</p>
      {row.kind !== undefined && (
        <p className="font-mono text-caption text-on-surface pl-md">{classificationLabel(row)}</p>
      )}
    </div>
  );
}

/** One vendor panel: port, sandbox run, live refusal. */
function VendorPanel({ vendor }: { vendor: ThirdPartyVendorPanel }) {
  return (
    <section
      key={vendor.id}
      data-testid={`tp-vendor-${vendor.id}`}
      className="flex flex-col gap-xs rounded-md border border-outline px-md py-sm"
    >
      <p className="text-body-sm font-medium text-on-surface">{vendor.title}</p>
      <p className="text-caption text-on-surface-variant">{vendor.lead}</p>

      <div data-testid={`tp-port-${vendor.id}`} className="flex flex-col gap-xxs">
        <p className="text-caption font-medium text-on-surface">
          port {vendor.port.name} — {vendor.port.note}
        </p>
        {vendor.port.methods.map((method) => (
          <p key={method.name} className="font-mono text-caption text-on-surface pl-md">
            {method.signature}
          </p>
        ))}
        {vendor.port.implementations.map((implementation) => (
          <p key={implementation} className="text-caption text-on-surface-variant pl-md">
            {implementation}
          </p>
        ))}
      </div>

      <div data-testid={`tp-sandbox-${vendor.id}`} className="flex flex-col gap-xxs">
        <p className="text-caption font-medium text-on-surface">the sandbox's observed run</p>
        {vendor.sandbox.config.map((line) => (
          <p key={line} className="text-caption text-on-surface-variant pl-md">
            {line}
          </p>
        ))}
        {vendor.sandbox.rows.map((row) => (
          <RunLine key={row.step} row={row} />
        ))}
        {vendor.sandbox.counts.map((count) => (
          <p key={count.label} className="font-mono text-caption text-on-surface pl-md" data-testid={`tp-count-${vendor.id}`}>
            {count.value}× {count.label}
          </p>
        ))}
      </div>

      <div data-testid={`tp-live-${vendor.id}`} className="flex flex-col gap-xxs">
        <p className="text-caption font-medium text-on-surface">
          the live skeleton: {vendor.live.vendor} over {vendor.live.apiBaseUrl}
        </p>
        <p className="font-mono text-caption text-on-surface-variant pl-md">
          {vendor.live.keyEnvVar} → {vendor.live.outcome.outcome} ({vendor.live.outcome.kind}{' '}
          {vendor.live.outcome.httpStatus})
        </p>
        <p className="text-caption text-on-surface-variant pl-md">{vendor.live.outcome.detail}</p>
        <p className="text-caption text-on-surface-variant pl-md">{vendor.live.note}</p>
      </div>
    </section>
  );
}

export function useThirdPartyHubDemo(
  useCase: GetThirdPartyHubDemoUseCase = container.resolve(THIRD_PARTY_HUB_DEMO_USE_CASE),
): { status: PanelStatus; replay: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<PanelStatus>({ state: 'idle' });

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

  const replay = () => setAttempt((current) => current + 1);
  return { status, replay };
}

export function ThirdPartyHubDemo() {
  const { status, replay } = useThirdPartyHubDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="thirdparty-hub-loading" className="text-body-sm text-on-surface-variant">
        driving the vendor adapters…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the third-party hub" failure={status.failure} onRetry={replay} />;
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid="thirdparty-hub-ready">
      <section
        data-testid="thirdparty-hub-wiring"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="text-caption text-on-surface-variant">{data.mode}</p>
        <p className="font-mono text-caption text-on-surface">
          dataMode: {data.wiring.current} — {data.wiring.source}
        </p>
        {data.wiring.rows.map((row) => (
          <div key={row.category} className="flex flex-col gap-xxs" data-testid={`tp-wiring-${row.category}`}>
            <p className="text-body-sm text-on-surface">
              {row.category} ({row.port}):{' '}
              <span className={row.active === 'fakeAdapter' ? 'text-primary' : 'text-error'}>
                {row.active === 'fakeAdapter' ? 'sandbox' : 'live skeleton'}
              </span>
            </p>
            <p className="font-mono text-caption text-on-surface-variant pl-md">{row.activeOutcome}</p>
          </div>
        ))}
        <p className="text-caption text-on-surface-variant">{data.wiring.note}</p>
      </section>

      {data.vendors.map((vendor) => (
        <VendorPanel key={vendor.id} vendor={vendor} />
      ))}

      <section
        data-testid="thirdparty-hub-taxonomy"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <p className="text-body-sm font-medium text-on-surface">
          The vendor-error taxonomy map ({data.taxonomyMap.length} codes)
        </p>
        {data.taxonomyMap.map((row) => (
          <p key={row.vendorCode} className="font-mono text-caption text-on-surface-variant">
            {row.vendorCode} (vendor {row.vendorStatus}) → {row.kind} {row.httpStatus}
            {row.viaRawStatus ? ' · via raw status' : ''}
            {row.retryable ? ' · retryable' : ' · permanent'}
          </p>
        ))}
      </section>

      <p data-testid="thirdparty-hub-honesty" className="text-body-sm text-on-surface-variant">
        {data.honestyNote}
      </p>

      <button
        type="button"
        onClick={replay}
        data-testid="thirdparty-hub-replay"
        className="self-start rounded-md border border-outline px-md py-xxs text-caption text-on-surface hover:bg-surface-tint"
      >
        replay the hub
      </button>
    </div>
  );
}

export default ThirdPartyHubDemo;
