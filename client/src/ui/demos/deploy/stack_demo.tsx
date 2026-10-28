import { useEffect, useState } from 'react';
import { container } from '../../../core/container';
import type { Failure } from '../../../core/error-handling/failures';
import { ErrorPanel } from '../../components/error-panel';
import type { DeployStackDemoData } from '../../../domain/demo/deploy-stack';
import {
  GetDeployStackDemoUseCase,
  DEPLOY_STACK_DEMO_USE_CASE,
} from '../../../domain/demo/get-deploy-stack-demo-usecase';

/**
 * DeployStackDemo — the deployment topic's stack viewer demo (demo
 * block id `deploy:stack`). It renders the payload of
 * GET /api/demo/deploy/stack: the compose half (the five stack
 * services with their images/ports, environment rails, dependency
 * conditions, healthchecks, and resource hints), the Kubernetes half
 * (each manifest's derived facts — the probe contract against the REAL
 * /api/health and /api/ready endpoints, the rollout knobs, the
 * placeholder-only secret), and the secret scan's verdict — every unit
 * carrying the repo-relative file it was parsed from.
 */

type StackStatus =
  | { state: 'idle' }
  | { state: 'loading' }
  | { state: 'ready'; data: DeployStackDemoData }
  | { state: 'error'; failure: Failure };

export function useDeployStackDemo(
  useCase: GetDeployStackDemoUseCase = container.resolve(DEPLOY_STACK_DEMO_USE_CASE),
): { status: StackStatus; retry: () => void } {
  const [attempt, setAttempt] = useState(0);
  const [status, setStatus] = useState<StackStatus>({ state: 'idle' });

  useEffect(() => {
    let cancelled = false;
    setStatus({ state: 'loading' });

    void useCase.execute().then((result) => {
      if (cancelled) return;
      if (result.ok) {
        setStatus({ state: 'ready', data: result.value });
      } else {
        setStatus({ state: 'error', failure: result.failure });
      }
    });

    return () => {
      cancelled = true;
    };
  }, [useCase, attempt]);

  const retry = () => setAttempt((current) => current + 1);
  return { status, retry };
}

export function DeployStackDemo() {
  const { status, retry } = useDeployStackDemo();

  if (status.state === 'idle' || status.state === 'loading') {
    return (
      <p data-testid="deploy-stack-loading" className="text-body-sm text-on-surface-variant">
        parsing the repo’s deployment artifacts…
      </p>
    );
  }

  if (status.state === 'error') {
    return <ErrorPanel subject="the deploy stack demo" failure={status.failure} onRetry={retry} />;
  }

  const { data } = status;

  return (
    <div className="flex flex-col gap-md" data-testid="deploy-stack-ready">
      <section
        data-testid="deploy-stack-compose"
        className="flex flex-col gap-xxs rounded-md border border-outline bg-surface-tint px-md py-sm"
      >
        <p className="text-caption text-on-surface-variant">{data.mode}</p>
        <p className="font-mono text-caption text-on-surface">
          {data.compose.project} · {String(data.compose.services.length)} services ·{' '}
          {data.compose.networks.join(', ')}
        </p>
        {data.compose.services.map((service) => (
          <div key={service.name} className="flex flex-col gap-xxs">
            <p className="font-mono text-caption text-on-surface-variant">
              <span className="text-on-surface">{service.name}</span>{' '}
              {service.image ?? service.buildsFrom ?? ''}{' '}
              {service.publishedPorts.length > 0
                ? `ports ${service.publishedPorts.join(', ')}`
                : 'no published ports'}
              {service.healthcheck ? ` · healthcheck: ${service.healthcheck}` : ' · platform-probed'}
              {service.resourceHint ? ` · ${service.resourceHint}` : ''}
            </p>
            {service.environment.length > 0 ? (
              <p className="font-mono text-caption text-on-surface-variant">
                env: {service.environment.join(' · ')}
              </p>
            ) : null}
            {service.dependsOn.length > 0 ? (
              <p className="font-mono text-caption text-on-surface-variant">
                depends on: {service.dependsOn.join(', ')}
              </p>
            ) : null}
          </div>
        ))}
        <p className="text-caption text-on-surface-variant">source: {data.compose.source}</p>
      </section>

      <section
        data-testid="deploy-stack-kubernetes"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        {data.kubernetes.map((manifest) => (
          <div key={manifest.file} className="flex flex-col gap-xxs">
            <p className="font-mono text-caption text-on-surface">
              {manifest.kind} {manifest.name}
              {manifest.namespace ? ` (${manifest.namespace})` : ''}
            </p>
            {manifest.facts.map((fact) => (
              <p key={fact.label + fact.value} className="font-mono text-caption text-on-surface-variant">
                <span className="text-on-surface">{fact.label}</span> — {fact.value}
              </p>
            ))}
          </div>
        ))}
      </section>

      <section
        data-testid="deploy-stack-secrets"
        className="flex flex-col gap-xxs rounded-md border border-outline px-md py-sm"
      >
        <p className="font-mono text-caption text-on-surface">
          secret scan · {data.secretScan.status} · {data.secretScan.checked.join(', ')}
        </p>
        <p className="text-caption text-on-surface-variant">{data.secretScan.rule}</p>
      </section>

      <p data-testid="deploy-stack-reasoning" className="text-body-sm text-on-surface-variant">
        {data.reasoning}
      </p>
    </div>
  );
}

export default DeployStackDemo;
