import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { SecurityPIIDemo } from './pii_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetSecurityStoryDemoUseCase,
  SECURITY_PII_DEMO_USE_CASE,
} from '../../../domain/demo/get-security-story-demo-usecase';
import type { SecurityStoryDemoData } from '../../../domain/demo/security-story';
import { securityPiiFixture } from './security_story_fixtures';

/**
 * SecurityPIIDemo spec — fixture-driven (happy + failure), the family
 * pattern: the component resolves its use case through the container
 * token, so the spec swaps that registration instead of stubbing
 * fetch. Behavior, not implementation: the demo's own distinctive
 * beats render (classification with the signup-IP lesson, the clean
 * audit in ok green), the derived proofs render, and failures degrade
 * to the shared error panel with recovery.
 */

const fixture: SecurityStoryDemoData = securityPiiFixture();

function stubUseCase(execute: () => Promise<Result<SecurityStoryDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    SECURITY_PII_DEMO_USE_CASE,
    { execute } as unknown as GetSecurityStoryDemoUseCase,
  );
}

describe('SecurityPIIDemo', () => {
  it('renders loading first, then the story with its beats and proofs', async () => {
    stubUseCase(async () => success(fixture));

    render(<SecurityPIIDemo />);

    expect(screen.getByTestId('security-pii-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('security-pii-ready')).toBeInTheDocument());
    expect(screen.getByTestId('security-pii-beat-classification')).toHaveTextContent('sub_ada');
    expect(screen.getByTestId('security-pii-beat-the-audit')).toHaveTextContent(
      'absent everywhere',
    );
    expect(screen.getByTestId('security-pii-proofs')).toHaveTextContent('the log cannot leak');
    expect(screen.getByTestId('security-pii-reasoning')).toBeInTheDocument();
  });

  it('degrades to the error panel on failure, with recovery', async () => {
    const failure = new ServerFailure('The security-pii demo could not run its story.', {
      code: 'internal',
      status: 500,
      requestId: 'rid-pii-500',
    });
    stubUseCase(async () => failed(failure));

    render(<SecurityPIIDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('internal');

    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('security-pii-ready')).toBeInTheDocument());
  });
});
