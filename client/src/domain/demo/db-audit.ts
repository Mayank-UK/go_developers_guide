/**
 * Domain models for the databases-sql topic's audit demo — the
 * client-side shape of `GET /api/demo/db/audit`. Field names match the
 * server's camelCase wire contract 1:1.
 *
 * Titles and counts are pinned by the choreography; both verdicts are
 * derived server-side from the observed world.
 */

/** One observed moment of the commit/rollback story. */
export interface AuditStep {
  phase: string;
  action: string;
  liveTitle: string;
  auditRows: number;
  note: string;
}

export interface AuditDemoData {
  steps: AuditStep[];
  committedTogether: boolean;
  rolledBackTogether: boolean;
  summary: string;
}
