/**
 * Domain models for the best-practices topic's self-audit demo (demo
 * block id `best:checklist`) — the client-side shape of
 * `GET /api/demo/best/checklist`. Field names match the server's
 * camelCase wire contract 1:1.
 *
 * The payload is a BOOT-CACHED audit, not narration: the server
 * executed every machine verdict once at process boot — live probes
 * through the real handler chain, real tool runs (gofmt, the snippet
 * ratchet, a -race round), real file reads — and serves the snapshot.
 * AREAS carry the verdict rows grouped by practice area, each row with
 * its check kind and its real evidence; NOT_DONES are the repo's
 * deliberate not-dos with reasons and artifacts; SUMMARY counts pass,
 * fail, and manual without spinning.
 */

/** One practice, one verdict, and the evidence that produced it. */
export interface BestChecklistItemRow {
  id: string;
  practice: string;
  checkKind: 'live' | 'exec' | 'file' | 'manual';
  verdict: 'pass' | 'fail' | 'manual';
  evidence: string[];
  whyManual?: string;
}

/** One practice area: its rows plus a one-line frame. */
export interface BestChecklistAreaRow {
  id: string;
  title: string;
  note: string;
  items: BestChecklistItemRow[];
}

/** One deliberate NOT-done: the decision, its reason, its artifacts. */
export interface BestNotDoneRow {
  id: string;
  notDone: string;
  reason: string;
  artifacts: string[];
}

/** The panel's score line — counts, nothing spun. */
export interface BestChecklistSummary {
  items: number;
  passed: number;
  failed: number;
  manual: number;
}

/** The whole self-audit payload. */
export interface BestChecklistDemoData {
  mode: string;
  computedAt: string;
  areas: BestChecklistAreaRow[];
  notDones: BestNotDoneRow[];
  summary: BestChecklistSummary;
  honestyNote: string;
}
