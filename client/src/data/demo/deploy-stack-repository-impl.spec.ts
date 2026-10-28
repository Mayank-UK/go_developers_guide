import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { DeployStackApi } from './deploy-stack-api';
import { DeployStackRepositoryImpl } from './deploy-stack-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * DeployStackRepositoryImpl spec — fixture-driven (happy + failure):
 * the stack payload (compose topology, manifest facts, secret scan)
 * round-trips; malformed payloads and transport failures resolve as
 * Failed Results, never throws.
 */

const demoPayload = {
  mode: 'file-derived: every fact below was parsed at request time',
  compose: {
    source: 'deploy/docker-compose.prod.yml',
    project: 'go_developers_guide_deploy',
    services: [
      {
        name: 'server',
        image: 'gdg_server_prod',
        publishedPorts: ['8100:8100'],
        environment: ['GDG_ENV=prod'],
        dependsOn: ['postgres (service_healthy)'],
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
      facts: [{ label: 'livenessProbe', value: 'GET /api/health on port http every 15s' }],
    },
  ],
  secretScan: {
    status: 'clean',
    rule: 'every Secret data value must be base64 that decodes to a CHANGE_ME_* placeholder',
    checked: ['k8s/secret.json/GDG_DATABASE_URL'],
  },
  reasoning: 'the compose half is the single-host deployment',
};

function makeRepository(): DeployStackRepositoryImpl {
  return new DeployStackRepositoryImpl(new DeployStackApi(createAppConfig()));
}

describe('DeployStackRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/deploy/stack', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.compose.project).toBe('go_developers_guide_deploy');
      expect(result.value.compose.services[0]?.name).toBe('server');
      expect(result.value.kubernetes[0]?.facts[0]?.value).toContain('/api/health');
      expect(result.value.secretScan.status).toBe('clean');
    }
  });

  it('guards a malformed payload into a Failed Result', async () => {
    stubApiFetch([{ url: '/api/demo/deploy/stack', body: { mode: 'half a payload' } }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
    }
  });

  it('guards a network failure into a Failed Result (network_error, status 0)', async () => {
    stubNetworkFailure();

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
    }
  });
});
