import type {
  DeployComposeService,
  DeployComposeTopology,
  DeployK8sFact,
  DeployK8sManifest,
  DeploySecretScan,
  DeployStackDemoData,
} from '../../domain/demo/deploy-stack';
import {
  isRecord,
  optionalString,
  requiredArray,
  requiredRecord,
  requiredString,
  requiredText,
} from '../content/validate';

/**
 * DeployStackDTO — the wire/transport shape of the deploy stack
 * payload. Mirrors the domain model 1:1; the `fromJson` builds ONLY
 * the listed fields (whitelist) and throws on any shape violation,
 * which parseDto upgrades to an InvalidResponseException.
 */

export type DeployStackDemoDto = DeployStackDemoData;

/**
 * stringItem validates one array element — a local helper because
 * `.map(requiredString)` would pass the array INDEX as requiredString's
 * key parameter (its `source[key]` lookup would slice a single
 * character off a string item instead of validating it).
 */
function stringItem(value: unknown): string {
  if (typeof value !== 'string' || value === '') {
    throw new Error('expected a non-empty string item');
  }
  return value;
}

function composeServiceFromJson(value: unknown): DeployComposeService {
  if (!isRecord(value)) throw new Error('compose service must be an object');
  return {
    name: requiredString(value, 'name'),
    image: optionalString(value, 'image'),
    buildsFrom: optionalString(value, 'buildsFrom'),
    publishedPorts: requiredArray(value, 'publishedPorts').map(stringItem),
    environment: requiredArray(value, 'environment').map(stringItem),
    healthcheck: optionalString(value, 'healthcheck'),
    dependsOn: requiredArray(value, 'dependsOn').map(stringItem),
    resourceHint: optionalString(value, 'resourceHint'),
  };
}

function composeTopologyFromJson(value: unknown): DeployComposeTopology {
  if (!isRecord(value)) throw new Error('compose topology must be an object');
  return {
    source: requiredText(value, 'source'),
    project: requiredString(value, 'project'),
    services: requiredArray(value, 'services').map(composeServiceFromJson),
    networks: requiredArray(value, 'networks').map(stringItem),
  };
}

function k8sFactFromJson(value: unknown): DeployK8sFact {
  if (!isRecord(value)) throw new Error('k8s fact must be an object');
  return {
    label: requiredString(value, 'label'),
    value: requiredString(value, 'value'),
  };
}

function k8sManifestFromJson(value: unknown): DeployK8sManifest {
  if (!isRecord(value)) throw new Error('k8s manifest must be an object');
  return {
    file: requiredText(value, 'file'),
    kind: requiredString(value, 'kind'),
    name: requiredString(value, 'name'),
    apiVersion: requiredString(value, 'apiVersion'),
    namespace: optionalString(value, 'namespace'),
    facts: requiredArray(value, 'facts').map(k8sFactFromJson),
  };
}

function secretScanFromJson(body: Record<string, unknown>): DeploySecretScan {
  const scan = requiredRecord(body, 'secretScan');
  return {
    status: requiredString(scan, 'status'),
    rule: requiredText(scan, 'rule'),
    checked: requiredArray(scan, 'checked').map(stringItem),
  };
}

/** Deserialize + validate the GET /api/demo/deploy/stack payload. */
export function deployStackDtoFromJson(body: unknown): DeployStackDemoDto {
  if (!isRecord(body)) throw new Error('deploy stack payload must be an object');
  return {
    mode: requiredText(body, 'mode'),
    compose: composeTopologyFromJson(body.compose),
    kubernetes: requiredArray(body, 'kubernetes').map(k8sManifestFromJson),
    secretScan: secretScanFromJson(body),
    reasoning: requiredText(body, 'reasoning'),
  };
}

/** Convert a transport DTO into a domain model (identity while 1:1). */
export function toDeployStackDemo(dto: DeployStackDemoDto): DeployStackDemoData {
  return dto;
}
