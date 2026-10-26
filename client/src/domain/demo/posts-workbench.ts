/**
 * Domain models for the api-design topic's posts-workbench demo — the
 * client-side shape of `GET /api/demo/api/posts-workbench`, the contract
 * card served beside the live CRUD workbench. Field names match the
 * server's camelCase wire contract 1:1.
 */

/** The resource the whole surface serves. */
export interface WorkbenchResource {
  name: string;
  collectionPath: string;
  itemPath: string;
  wireShape: string;
}

/** A route's success contract. */
export interface WorkbenchOutcome {
  status: number;
  note: string;
}

/** One error case a route can answer (the envelope's row). */
export interface WorkbenchErrorContract {
  status: number;
  code: string;
  when: string;
}

/** One row of the route table: method, path, purpose, success, errors. */
export interface WorkbenchRoute {
  id: string;
  method: string;
  path: string;
  purpose: string;
  success: WorkbenchOutcome;
  errors: WorkbenchErrorContract[];
}

/** The list route's paging contract. */
export interface WorkbenchPagination {
  defaultLimit: number;
  maxLimit: number;
  defaultPage: number;
  defaultSort: string;
  sortOptions: string[];
  envelopeFields: string[];
  emptyPageRule: string;
}

/** The filter semantics. */
export interface WorkbenchFiltering {
  param: string;
  semantics: string;
  example: string;
}

/** The versioning stance. */
export interface WorkbenchVersioning {
  prefix: string;
  strategy: string;
  contentExample: string;
  whenToVersion: string;
}

/** The recorded DELETE decision. */
export interface WorkbenchDeleteSemantics {
  onMiss: number;
  chosen: string;
  rationale: string;
  alternative: string;
}

/** One member of the error envelope with its meaning. */
export interface WorkbenchEnvelopeMember {
  name: string;
  meaning: string;
}

/** The error envelope's anatomy plus the RFC 9457 comparison. */
export interface WorkbenchEnvelope {
  wireShape: string;
  members: WorkbenchEnvelopeMember[];
  rfc9457Verdict: string;
}

/** The whole contract-card payload. */
export interface PostsWorkbenchDemoData {
  resource: WorkbenchResource;
  routes: WorkbenchRoute[];
  pagination: WorkbenchPagination;
  filtering: WorkbenchFiltering;
  versioning: WorkbenchVersioning;
  deleteSemantics: WorkbenchDeleteSemantics;
  envelope: WorkbenchEnvelope;
}
