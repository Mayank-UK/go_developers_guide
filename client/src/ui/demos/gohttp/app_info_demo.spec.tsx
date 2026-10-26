import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { AppInfoDemo } from './app_info_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import type { GetAppInfoDemoUseCase } from '../../../domain/demo/get-app-info-demo-usecase';
import { GetAppInfoDemoUseCaseToken } from '../../../domain/demo/get-app-info-demo-usecase';
import type { AppInfoDemoData } from '../../../domain/demo/app-info';

/**
 * AppInfoDemo spec — fixture-driven (happy + failure). The component
 * resolves its use case through the container under
 * GetAppInfoDemoUseCaseToken (the wave seam this slice owns until
 * container.ts grows the family's token pair + getter), so the spec swaps
 * that registration — the container's documented replace-a-singleton seam —
 * instead of stubbing fetch. Behavior, not implementation: the four payload
 * faces render, the zero-dependency fact is displayed as the proof it is,
 * empty data values (an unset build flag) render honestly as a dash, and
 * failures degrade to the shared error panel with recovery.
 *
 * The demo-registry entry for `gohttp:app-info` lands with the master's
 * wave-close wiring and is asserted there, not here.
 */

const fixture: AppInfoDemoData = {
  module: { path: 'guide', version: '(devel)', goVersion: 'go1.27.3', dependencyCount: 0 },
  runtime: { numCPU: 8, goMaxProcs: 8 },
  environment: { port: 8100, env: 'dev', logLevel: 'INFO' },
  buildSettings: [
    { key: '-buildmode', value: 'exe' },
    { key: 'CGO_CFLAGS', value: '' },
    { key: 'GOARCH', value: 'arm64' },
  ],
};

function stubUseCase(execute: () => Promise<Result<AppInfoDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    GetAppInfoDemoUseCaseToken,
    { execute } as unknown as GetAppInfoDemoUseCase,
  );
}

describe('AppInfoDemo', () => {
  it('renders loading first, then all four payload faces', async () => {
    stubUseCase(async () => success(fixture));

    render(<AppInfoDemo />);

    expect(screen.getByTestId('gohttp-app-info-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('ai-module')).toBeInTheDocument());

    // Module: the guide binary's identity, zero dependencies as the proof.
    expect(screen.getByTestId('ai-module')).toHaveTextContent('guide');
    expect(screen.getByTestId('ai-module')).toHaveTextContent('(devel)');
    expect(screen.getByTestId('ai-module')).toHaveTextContent('go1.27.3');
    expect(screen.getByTestId('ai-dependency-count')).toHaveTextContent('0 dependencies');

    // Runtime: both scheduler facts, and the equal-unless note.
    expect(screen.getByTestId('ai-runtime')).toHaveTextContent('NumCPU 8');
    expect(screen.getByTestId('ai-runtime')).toHaveTextContent('GOMAXPROCS 8');

    // Environment: the booted config values, each labeled.
    expect(screen.getByTestId('ai-environment')).toHaveTextContent('port:');
    expect(screen.getByTestId('ai-environment')).toHaveTextContent('8100');
    expect(screen.getByTestId('ai-environment')).toHaveTextContent('logLevel:');
    expect(screen.getByTestId('ai-environment')).toHaveTextContent('INFO');

    // Build settings: every row; an unset flag's empty value renders as a
    // dash, not as a blank guess.
    expect(screen.getByTestId('ai-settings')).toHaveTextContent('-buildmode');
    expect(screen.getByTestId('ai-settings')).toHaveTextContent('CGO_CFLAGS');
    expect(screen.getByTestId('ai-settings')).toHaveTextContent('—');
  });

  it('notes when GOMAXPROCS differs from NumCPU (container reality)', async () => {
    stubUseCase(async () =>
      success({ ...fixture, runtime: { numCPU: 8, goMaxProcs: 2 } }),
    );

    render(<AppInfoDemo />);

    await waitFor(() => expect(screen.getByTestId('ai-runtime')).toBeInTheDocument());
    expect(screen.getByTestId('ai-runtime')).toHaveTextContent('GOMAXPROCS 2');
    expect(screen.getByTestId('ai-runtime')).toHaveTextContent('differs from NumCPU');
  });

  it('degrades to the shared error panel on a Failed Result, with recovery', async () => {
    let calls = 0;
    stubUseCase(async () => {
      calls += 1;
      return calls === 1
        ? failed(
            new ServerFailure('The demo endpoint is down.', {
              code: 'service_unavailable',
              status: 503,
              requestId: 'req-44',
            }),
          )
        : success(fixture);
    });

    render(<AppInfoDemo />);

    await waitFor(() => expect(screen.getByTestId('error-panel')).toBeInTheDocument());
    expect(screen.getByTestId('error-panel')).toHaveTextContent('service_unavailable');

    // Retry re-runs the use case and the payload renders.
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('ai-module')).toBeInTheDocument());
    expect(screen.queryByTestId('error-panel')).not.toBeInTheDocument();
  });
});
