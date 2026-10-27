import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { SecurityLabDemo } from './lab_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetSecurityStoryDemoUseCase,
  SECURITY_LAB_DEMO_USE_CASE,
} from '../../../domain/demo/get-security-story-demo-usecase';
import type { SecurityStoryDemoData } from '../../../domain/demo/security-story';
import { securityLabFixture } from './security_story_fixtures';

/**
 * SecurityLabDemo spec — fixture-driven (happy + failure), the family
 * pattern: the component resolves its use case through the container
 * token, so the spec swaps that registration instead of stubbing
 * fetch. Behavior, not implementation: the demo's own distinctive
 * beats render with their verdicts and facts (the header set, the 429
 * boundary in rejected red), the derived proofs render, and failures
 * degrade to the shared error panel with recovery.
 */

const fixture: SecurityStoryDemoData = securityLabFixture();

function stubUseCase(execute: () => Promise<Result<SecurityStoryDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    SECURITY_LAB_DEMO_USE_CASE,
    { execute } as unknown as GetSecurityStoryDemoUseCase,
  );
}

describe('SecurityLabDemo', () => {
  it('renders loading first, then the story with its beats and proofs', async () => {
    stubUseCase(async () => success(fixture));

    render(<SecurityLabDemo />);

    expect(screen.getByTestId('security-lab-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('security-lab-ready')).toBeInTheDocument());
    expect(screen.getByTestId('security-lab-beat-the-header-set')).toHaveTextContent(
      'Content-Security-Policy',
    );
    expect(screen.getByTestId('security-lab-beat-the-429-boundary')).toHaveTextContent(
      '429 rate_limited',
    );
    expect(screen.getByTestId('security-lab-proofs')).toHaveTextContent('burst vs sustained');
    expect(screen.getByTestId('security-lab-reasoning')).toBeInTheDocument();
  });

  it('degrades to the error panel on failure, with recovery', async () => {
    const failure = new ServerFailure('The security-lab demo could not run its story.', {
      code: 'rate_limited',
      status: 429,
      requestId: 'rid-security-429',
    });
    stubUseCase(async () => failed(failure));

    render(<SecurityLabDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('rate_limited');

    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('security-lab-ready')).toBeInTheDocument());
  });
});
