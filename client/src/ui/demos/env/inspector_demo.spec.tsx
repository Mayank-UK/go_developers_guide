import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { EnvInspectorDemo } from './inspector_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetEnvInspectorDemoUseCase,
  ENV_INSPECTOR_DEMO_USE_CASE,
} from '../../../domain/demo/get-env-inspector-demo-usecase';
import type { EnvInspectorDemoData } from '../../../domain/demo/env-inspector';

/**
 * EnvInspectorDemo spec — fixture-driven (happy + failure): the
 * component resolves its use case through the container token.
 * Behavior, not implementation: the variables panel states each row's
 * value, source, and redaction marker; the scorecard renders all
 * twelve verdicts; the flag panel states the ScenarioStore's current
 * state; and failures degrade to the shared error panel with recovery.
 */

const fixture: EnvInspectorDemoData = {
  mode: 'runtime-derived: every row below was resolved by core.LoadConfig',
  env: 'dev',
  variables: [
    {
      field: 'Port',
      envVar: 'GDG_PORT',
      value: '8100',
      source: 'default',
      sensitive: false,
      validatedBy: 'parsed by strconv.Atoi at boot; must be 1–65535 or the boot fails',
    },
    {
      field: 'DatabaseURL',
      envVar: 'GDG_DATABASE_URL',
      value: 'postgres://postgres@postgres:5432/guide',
      source: 'environment',
      sensitive: true,
      validatedBy: 'the pool dials at boot and the boot fails fast',
    },
  ],
  twelveFactor: [
    {
      factor: 'I. Codebase',
      verdict: 'hits',
      how: 'one git tree serves every environment',
    },
    {
      factor: 'VIII. Concurrency',
      verdict: 'differs',
      how: 'the guide hosts the jobs worker IN the API process',
    },
  ],
  flags: { dataMode: 'fake', injectFailure: 'none' },
  redaction: 'sensitive rows are redacted by policy: a DSN’s password is never echoed',
  reasoning: 'names are the contract, defaults make dev zero-config',
};

function stubUseCase(execute: () => Promise<Result<EnvInspectorDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    ENV_INSPECTOR_DEMO_USE_CASE,
    { execute } as unknown as GetEnvInspectorDemoUseCase,
  );
}

describe('EnvInspectorDemo', () => {
  it('renders loading first, then the variables, scorecard, and flags panels', async () => {
    stubUseCase(async () => success(fixture));

    render(<EnvInspectorDemo />);

    expect(screen.getByTestId('env-inspector-loading')).toBeInTheDocument();

    await waitForReady();
    const variables = screen.getByTestId('env-inspector-variables');
    expect(variables).toHaveTextContent('env: dev');
    expect(variables).toHaveTextContent('GDG_PORT');
    expect(variables).toHaveTextContent('8100');
    expect(variables).toHaveTextContent('from default');
    expect(variables).toHaveTextContent('GDG_DATABASE_URL');
    expect(variables).toHaveTextContent('postgres://postgres@postgres:5432/guide');
    expect(variables).toHaveTextContent('[redacted]');
    expect(variables).toHaveTextContent('from environment');

    const scorecard = screen.getByTestId('env-inspector-scorecard');
    expect(scorecard).toHaveTextContent('I. Codebase');
    expect(scorecard).toHaveTextContent('hits');
    expect(scorecard).toHaveTextContent('VIII. Concurrency');
    expect(scorecard).toHaveTextContent('differs');

    const flags = screen.getByTestId('env-inspector-flags');
    expect(flags).toHaveTextContent('dataMode: fake');
    expect(flags).toHaveTextContent('injectFailure: none');

    expect(screen.getByTestId('env-inspector-reasoning')).toHaveTextContent(
      'defaults make dev zero-config',
    );
  });

  it('never renders a credential in a sensitive row — only the redacted shape', async () => {
    stubUseCase(async () => success(fixture));

    render(<EnvInspectorDemo />);
    await waitForReady();

    const variables = screen.getByTestId('env-inspector-variables');
    expect(variables).toHaveTextContent('[redacted]');
    expect(variables.textContent).not.toMatch(/:\/\/[^/@]+:[^/@]+@/);
  });

  it('degrades to the error panel on a transport failure, with recovery', async () => {
    const failure = new ServerFailure(
      'The env inspector payload did not arrive in the expected shape.',
      { code: 'internal', status: 500, requestId: 'rid-env-inspector' },
    );
    stubUseCase(async () => failed(failure));

    render(<EnvInspectorDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('env inspector');

    // Retry re-runs the derive through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitForReady();
  });
});

function waitForReady() {
  return waitFor(() => expect(screen.getByTestId('env-inspector-ready')).toBeInTheDocument());
}
