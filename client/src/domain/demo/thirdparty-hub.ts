/**
 * Domain models for the third-party topic's adapter-surface hub (demo
 * block id `thirdparty:hub`) — the client-side shape of
 * `GET /api/demo/thirdparty/hub`. Field names match the server's
 * camelCase wire contract 1:1.
 *
 * The payload is REAL machinery observed on the very request that
 * served it: three vendor ports (mailer, payments, object storage),
 * each driven through a fixed script against a deterministic sandbox
 * adapter (signing, idempotency keys, quota accounting, error-taxonomy
 * mapping) beside its credential-gated live skeleton's honest refusal;
 * the WIRING panel shows which adapter the shared scenario store's
 * data mode currently selects, with one observed probe through each
 * ACTIVE adapter; the TAXONOMY map is the vendor-error classification
 * table with HTTP statuses derived through the server's real
 * core.HTTPStatusFor. No vendor network call is ever made, and every
 * count was counted during the run — never invented.
 */

/** One observed call of a vendor script: the call, the answer, the classification. */
export interface ThirdPartyRunRow {
  step: number;
  call: string;
  input: string;
  outcome: string;
  vendorCode?: string;
  vendorStatus?: number;
  kind?: string;
  httpStatus?: number;
  retryable: boolean;
  detail: string;
}

/** One counted observation — counted during the run, never asserted. */
export interface ThirdPartyCountRow {
  label: string;
  value: number;
}

/** One method of a vendor port. */
export interface ThirdPartyPortMethod {
  name: string;
  signature: string;
}

/** A vendor port as served: methods, asserted implementations, the seam note. */
export interface ThirdPartyPortPanel {
  name: string;
  methods: ThirdPartyPortMethod[];
  implementations: string[];
  note: string;
}

/** One vendor's sandbox run: the config, the observed rows, the counts. */
export interface ThirdPartySandboxPanel {
  config: string[];
  rows: ThirdPartyRunRow[];
  counts: ThirdPartyCountRow[];
}

/** One vendor's live half: the config, the honest refusal outcome, the seam note. */
export interface ThirdPartyLivePanel {
  vendor: string;
  apiBaseUrl: string;
  keyEnvVar: string;
  callSite: string;
  outcome: ThirdPartyRunRow;
  note: string;
}

/** One vendor category end to end: the port, the sandbox run, the live skeleton. */
export interface ThirdPartyVendorPanel {
  id: string;
  title: string;
  lead: string;
  port: ThirdPartyPortPanel;
  sandbox: ThirdPartySandboxPanel;
  live: ThirdPartyLivePanel;
}

/** One vendor's switch position: both adapters, the active one, one observed probe. */
export interface ThirdPartyWiringRow {
  category: string;
  port: string;
  fakeAdapter: string;
  liveAdapter: string;
  active: string;
  activeOutcome: string;
}

/** The DI switch as served: its source, position, and per-vendor rows. */
export interface ThirdPartyWiringPanel {
  source: string;
  current: string;
  getEndpoint: string;
  patchEndpoint: string;
  rows: ThirdPartyWiringRow[];
  note: string;
}

/** One vendor-error mapping row, HTTP status derived via the real taxonomy function. */
export interface ThirdPartyTaxonomyRow {
  vendorCode: string;
  vendorStatus: number;
  kind: string;
  httpStatus: number;
  retryable: boolean;
  viaRawStatus: boolean;
  note: string;
}

/** The whole third-party hub payload. */
export interface ThirdPartyHubDemoData {
  mode: string;
  honestyNote: string;
  wiring: ThirdPartyWiringPanel;
  vendors: ThirdPartyVendorPanel[];
  taxonomyMap: ThirdPartyTaxonomyRow[];
}
