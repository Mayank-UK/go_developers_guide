import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { DeployStackDemo } from './stack_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetDeployStackDemoUseCase,
  DEPLOY_STACK_DEMO_USE_CASE,
} from '../../../domain/demo/get-deploy-stack-demo-usecase';
import type { DeployStackDemoData } from '../../../domain/demo/deploy-stack';

/**
 * DeployStackDemo spec — fixture-driven (happy + failure): the
 * component resolves its use case through the container token.
 * Behavior, not implementation: the compose panel states each
 * service's image/ports and provenance source; the kubernetes panel
 * renders each manifest's facts (the probe contract visible); the
 * secret scan panel states its verdict; and failures degrade to the
 * shared error panel with recovery.
 */

const fixture: DeployStackDemoData = {
  mode: 'file-derived: every service, port, healthcheck, probe path below was parsed at request time',
  compose: {
    source: 'deploy/docker-compose.prod.yml',
    project: 'go_developers_guide_deploy',
    services: [
      {
        name: 'postgres',
        image: 'postgres:17',
        publishedPorts: [],
        environment: ['POSTGRES_USER=guide'],
        healthcheck: 'CMD-SHELL pg_isready -U guide -d guide',
        dependsOn: [],
        resourceHint: 'limits 1.0 cpu / 512M memory',
      },
      {
        name: 'server',
        image: 'gdg_server_prod',
        buildsFrom: './.docker/server/Dockerfile.prod',
        publishedPorts: ['8100:8100'],
        environment: ['GDG_ENV=prod', 'GDG_DATABASE_URL=postgres://guide:${POSTGRES_PASSWORD:?rail}'],
        dependsOn: ['postgres (service_healthy)'],
        resourceHint: 'limits 1.0 cpu / 256M memory',
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
      namespace: 'go-developers-guide',
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

function stubUseCase(execute: () => Promise<Result<DeployStackDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    DEPLOY_STACK_DEMO_USE_CASE,
    { execute } as unknown as GetDeployStackDemoUseCase,
  );
}

describe('DeployStackDemo', () => {
  it('renders loading first, then the compose, kubernetes, and secret-scan panels', async () => {
    stubUseCase(async () => success(fixture));

    render(<DeployStackDemo />);

    expect(screen.getByTestId('deploy-stack-loading')).toBeInTheDocument();

    await waitForReady();
    const compose = screen.getByTestId('deploy-stack-compose');
    expect(compose).toHaveTextContent('go_developers_guide_deploy');
    expect(compose).toHaveTextContent('2 services');
    expect(compose).toHaveTextContent('postgres:17');
    expect(compose).toHaveTextContent('gdg_server_prod');
    expect(compose).toHaveTextContent('8100:8100');
    expect(compose).toHaveTextContent('postgres (service_healthy)');
    expect(compose).toHaveTextContent('source: deploy/docker-compose.prod.yml');

    const kubernetes = screen.getByTestId('deploy-stack-kubernetes');
    expect(kubernetes).toHaveTextContent('Deployment guide');
    expect(kubernetes).toHaveTextContent('GET /api/health');
    expect(kubernetes).toHaveTextContent('GET /api/ready');
    expect(kubernetes).toHaveTextContent('Secret guide-secret');
    expect(kubernetes).toHaveTextContent('CHANGE_ME_DB_PASSWORD');

    const secrets = screen.getByTestId('deploy-stack-secrets');
    expect(secrets).toHaveTextContent('secret scan · clean');
    expect(secrets).toHaveTextContent('k8s/secret.json/GDG_DATABASE_URL');

    expect(screen.getByTestId('deploy-stack-reasoning')).toHaveTextContent(
      'single-host deployment',
    );
  });

  it('renders the placeholder DSN, never a credential shape', async () => {
    stubUseCase(async () => success(fixture));

    render(<DeployStackDemo />);
    await waitForReady();

    const kubernetes = screen.getByTestId('deploy-stack-kubernetes');
    expect(kubernetes).toHaveTextContent('CHANGE_ME_DB_PASSWORD');
    expect(kubernetes.textContent).not.toMatch(/:\/\/[^/@]+:(?!CHANGE_ME)[^/@]*@/);
  });

  it('degrades to the error panel on a transport failure, with recovery', async () => {
    const failure = new ServerFailure(
      'The deployment story is parsed live from the repo artifacts, which are not beside this process.',
      { code: 'service_unavailable', status: 503, requestId: 'rid-deploy-stack' },
    );
    stubUseCase(async () => failed(failure));

    render(<DeployStackDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('deploy stack');

    // Retry re-parses the declarations through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitForReady();
  });
});

function waitForReady() {
  return waitFor(() => expect(screen.getByTestId('deploy-stack-ready')).toBeInTheDocument());
}
