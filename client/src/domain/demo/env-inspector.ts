/**
 * Domain models for the environments topic's env inspector demo (demo
 * block id `env:inspector`) — the client-side shape of
 * `GET /api/demo/env/inspector`. Field names match the server's
 * camelCase wire contract 1:1.
 *
 * The payload is RUNTIME-DERIVED: the server derives every row from the
 * LIVE config it booted with — VARIABLES is the resolved-config table
 * (each row: its Go field, env name, validated value, and whether it
 * came from the environment or the default, with the two
 * credential-bearing slots redacted by policy), TWELVEFACTOR is the
 * honest scorecard as data, and FLAGS snapshots the ScenarioStore's
 * CURRENT state — flip it with PATCH /api/demo/scenario and reload to
 * watch the panel follow.
 */

/** One row of the resolved-config table. */
export interface EnvInspectorVariable {
  field: string;
  envVar: string;
  value: string;
  source: string;
  sensitive: boolean;
  validatedBy: string;
}

/** One factor of the twelve-factor scorecard: verdict + how it was earned. */
export interface EnvInspectorScoreRow {
  factor: string;
  verdict: string;
  how: string;
}

/** The live flag panel: the ScenarioStore's current state. */
export interface EnvInspectorFlags {
  dataMode: string;
  injectFailure: string;
}

/** The whole env inspector payload. */
export interface EnvInspectorDemoData {
  mode: string;
  env: string;
  variables: EnvInspectorVariable[];
  twelveFactor: EnvInspectorScoreRow[];
  flags: EnvInspectorFlags;
  redaction: string;
  reasoning: string;
}
