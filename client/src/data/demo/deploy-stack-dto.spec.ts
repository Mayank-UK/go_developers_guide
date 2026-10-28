import { describe, expect, it } from 'vitest';
import { deployStackDtoFromJson } from './deploy-stack-dto';

/**
 * DeployStackDTO spec — the fromJson whitelist: a full payload builds
 * the domain shape (the compose topology with its services and
 * environment rails, the k8s manifest facts, the secret-scan verdict);
 * every violation (missing panels, wrong types, a manifest without
 * facts) throws so parseDto can upgrade it to an
 * InvalidResponseException.
 */

const fullPayload = {
  mode: 'file-derived: every fact below was parsed at request time from the committed artifacts',
  compose: {
    source: 'deploy/docker-compose.prod.yml',
    project: 'go_developers_guide_deploy',
    services: [
      {
        name: 'server',
        image: 'gdg_server_prod',
        buildsFrom: './.docker/server/Dockerfile.prod',
        publishedPorts: ['8100:8100'],
        environment: ['GDG_ENV=prod', 'GDG_DATABASE_URL=postgres://guide:${POSTGRES_PASSWORD:?rail}'],
        healthcheck: '',
        dependsOn: ['postgres (service_healthy)', 'redis (service_healthy)'],
        resourceHint: 'limits 1.0 cpu / 256M memory; reservations 0.25 cpu / 64M memory',
      },
      {
        name: 'postgres',
        image: 'postgres:17',
        publishedPorts: [],
        environment: ['POSTGRES_USER=guide'],
        healthcheck: 'CMD-SHELL pg_isready -U guide -d guide',
        dependsOn: [],
      },
    ],
    networks: ['app_internal'],
  },
  kubernetes: [
    {
      file: 'deploy/k8s/deployment.json',
      kind: 'Deployment',
      name: 'guide',
      apiVersion: 'apps/v1',
      namespace: 'go-developers-guide',
      facts: [
        { label: 'replicas', value: '2 (minReadySeconds 5)' },
        { label: 'livenessProbe', value: 'GET /api/health on port http every 15s' },
        { label: 'readinessProbe', value: 'GET /api/ready on port http every 10s' },
      ],
    },
    {
      file: 'deploy/k8s/secret.json',
      kind: 'Secret',
      name: 'guide-secret',
      apiVersion: 'v1',
      facts: [
        {
          label: 'GDG_DATABASE_URL',
          value: 'base64, decodes to the placeholder: postgres://guide:CHANGE_ME_DB_PASSWORD@postgres:5432',
        },
      ],
    },
  ],
  secretScan: {
    status: 'clean',
    rule: 'every Secret data value must be base64 that decodes to a CHANGE_ME_* placeholder',
    checked: ['k8s/secret.json/GDG_DATABASE_URL'],
  },
  reasoning: 'the compose half is the single-host deployment; the kubernetes half is the cluster story',
};

describe('deployStackDtoFromJson', () => {
  it('builds the domain payload from a valid wire body', () => {
    const dto = deployStackDtoFromJson(fullPayload);

    expect(dto.compose.source).toBe('deploy/docker-compose.prod.yml');
    expect(dto.compose.services).toHaveLength(2);
    expect(dto.compose.services[0]?.dependsOn[0]).toBe('postgres (service_healthy)');
    expect(dto.compose.services[1]?.healthcheck).toBe('CMD-SHELL pg_isready -U guide -d guide');
    expect(dto.compose.services[1]?.image).toBe('postgres:17');
    expect(dto.compose.services[1]?.buildsFrom).toBeUndefined();
    expect(dto.kubernetes[0]?.facts[1]?.value).toContain('/api/health');
    expect(dto.secretScan.status).toBe('clean');
    expect(dto.secretScan.checked[0]).toBe('k8s/secret.json/GDG_DATABASE_URL');
  });

  it('throws when the body is not an object', () => {
    expect(() => deployStackDtoFromJson(null)).toThrow();
    expect(() => deployStackDtoFromJson('deploy stack')).toThrow();
  });

  it('throws when a top-level panel is missing', () => {
    const { compose, ...withoutCompose } = fullPayload;
    expect(() => deployStackDtoFromJson(withoutCompose)).toThrow(/compose/i);

    const { kubernetes, ...withoutManifests } = fullPayload;
    expect(() => deployStackDtoFromJson(withoutManifests)).toThrow(/kubernetes/i);

    const { secretScan, ...withoutScan } = fullPayload;
    expect(() => deployStackDtoFromJson(withoutScan)).toThrow(/secretScan/i);
  });

  it('throws when a service row carries the wrong types', () => {
    expect(() =>
      deployStackDtoFromJson({
        ...fullPayload,
        compose: {
          ...fullPayload.compose,
          services: [{ ...fullPayload.compose.services[0], publishedPorts: '8100' }],
        },
      }),
    ).toThrow();
    expect(() =>
      deployStackDtoFromJson({
        ...fullPayload,
        compose: { ...fullPayload.compose, services: [{ ...fullPayload.compose.services[0], name: 42 }] },
      }),
    ).toThrow();
  });

  it('throws when a manifest lacks its facts', () => {
    expect(() =>
      deployStackDtoFromJson({
        ...fullPayload,
        kubernetes: [{ ...fullPayload.kubernetes[0], facts: 'none' }],
      }),
    ).toThrow(/facts/);
  });
});
