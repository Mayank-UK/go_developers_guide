import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { AuthOAuthDemo } from './oauth_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetAuthStoryDemoUseCase,
  AUTH_OAUTH_DEMO_USE_CASE,
} from '../../../domain/demo/get-auth-story-demo-usecase';
import type { AuthStoryDemoData } from '../../../domain/demo/auth-story';
import { authStoryFixture } from './auth_story_fixtures';

/**
 * AuthOAuthDemo spec — fixture-driven (happy + failure), the family
 * pattern: the component resolves its use case through the container
 * token, so the spec swaps that registration instead of stubbing
 * fetch. Behavior, not implementation: the demo's own distinctive
 * beats render with their verdicts and facts, the derived proofs
 * render, and failures degrade to the shared error panel with
 * recovery.
 */

const fixture: AuthStoryDemoData = authStoryFixture(
    {
      mechanism: 'oauth',
      beats: [
        { beat: 'exchange (back channel)', detail: 'code + verifier → an access token, the code was burned.', verdict: 'ok', facts: [] },
        { beat: 'code replay', detail: 'the same code presented again → invalid_grant.', verdict: 'rejected', facts: [] },
      ],
      proofs: [{ label: 'one-time codes', value: 'consumed on FIRST presentation' }],
      reasoning: 'the code grant with PKCE, whole and in one process.',
    });

function stubUseCase(execute: () => Promise<Result<AuthStoryDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    AUTH_OAUTH_DEMO_USE_CASE,
    { execute } as unknown as GetAuthStoryDemoUseCase,
  );
}

describe('AuthOAuthDemo', () => {
  it('renders loading first, then the story with its beats and proofs', async () => {
    stubUseCase(async () => success(fixture));

    render(<AuthOAuthDemo />);

    expect(screen.getByTestId('auth-oauth-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('auth-oauth-ready')).toBeInTheDocument());
    expect(screen.getByTestId('auth-oauth-beat-exchange-back-channel')).toHaveTextContent('burned');
    expect(screen.getByTestId('auth-oauth-beat-code-replay')).toHaveTextContent('invalid_grant');
    expect(screen.getByTestId('auth-oauth-proofs')).toHaveTextContent('consumed on FIRST presentation');
    expect(screen.getByTestId('auth-oauth-reasoning')).toBeInTheDocument();
  });

  it('degrades to the error panel on failure, with recovery', async () => {
    const failure = new ServerFailure('The auth demo could not run its story.', {
      code: 'internal',
      status: 500,
      requestId: 'rid-auth-500',
    });
    stubUseCase(async () => failed(failure));

    render(<AuthOAuthDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('internal');

    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('auth-oauth-ready')).toBeInTheDocument());
  });
});
