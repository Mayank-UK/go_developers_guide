/**
 * Domain models for the deployment topic's stack viewer demo (demo
 * block id `deploy:stack`) — the client-side shape of
 * `GET /api/demo/deploy/stack`. Field names match the server's
 * camelCase wire contract 1:1.
 *
 * The payload is FILE-DERIVED: the server parsed the repo's committed
 * deploy/ artifacts (the prod compose file + every k8s manifest) at
 * request time, so COMPOSE is the single-host declaration (services
 * with images, published ports, environment rails, dependency
 * conditions, healthchecks, resource hints), KUBERNETES is the cluster
 * declaration (one view per manifest with its derived facts — probe
 * paths against the real endpoints, rollout knobs, the placeholder
 * secret), and SECRETSCAN states the verdict of the gate the payload
 * rode through. Every unit carries its provenance file path.
 */

/** One service of the prod compose declaration. */
export interface DeployComposeService {
  name: string;
  image?: string;
  buildsFrom?: string;
  publishedPorts: string[];
  environment: string[];
  healthcheck?: string;
  dependsOn: string[];
  resourceHint?: string;
}

/** The compose half of the payload: the single-host stack. */
export interface DeployComposeTopology {
  source: string;
  project: string;
  services: DeployComposeService[];
  networks: string[];
}

/** One derived fact of a k8s manifest (label + value). */
export interface DeployK8sFact {
  label: string;
  value: string;
}

/** One k8s manifest view: envelope, provenance, derived facts. */
export interface DeployK8sManifest {
  file: string;
  kind: string;
  name: string;
  apiVersion: string;
  namespace?: string;
  facts: DeployK8sFact[];
}

/** The verdict of the placeholder-secret gate. */
export interface DeploySecretScan {
  status: string;
  rule: string;
  checked: string[];
}

/** The whole deploy stack payload. */
export interface DeployStackDemoData {
  mode: string;
  compose: DeployComposeTopology;
  kubernetes: DeployK8sManifest[];
  secretScan: DeploySecretScan;
  reasoning: string;
}
