import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { AuthCompareDemo } from './compare_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetAuthStoryDemoUseCase,
  AUTH_COMPARE_DEMO_USE_CASE,
} from '../../../domain/demo/get-auth-story-demo-usecase';
import type { AuthStoryDemoData } from '../../../domain/demo/auth-story';
import { authStoryFixture } from './auth_story_fixtures';

/**
 * AuthCompareDemo spec — fixture-driven (happy + failure), the family
 * pattern: the component resolves its use case through the container
 * token, so the spec swaps that registration instead of stubbing
 * fetch. Behavior, not implementation: the demo's own distinctive
 * beats render with their verdicts and facts, the derived proofs
 * render, and failures degrade to the shared error panel with
 * recovery.
 */

const fixture: AuthStoryDemoData = authStoryFixture(
    {
      mechanism: 'compare',
      beats: [
        { beat: 'session cookie', detail: 'verification took 0.1 ms and consults the session store.', verdict: 'ok',
          facts: [{ label: 'after revoke', value: 'the cookie resolves to nothing — dead' }] },
        { beat: 'jwt (HS256)', detail: 'verification took 0.0 ms and consults nothing but the signature.', verdict: 'ok',
          facts: [{ label: 'after revoke', value: 'STILL VALID (valid) — and it will be until exp' }] },
      ],
      proofs: [{ label: 'jwt', value: 'revocation is approximate, expiry is the only lever' }],
      reasoning: 'server-held truth versus self-carried truth, measured live.',
    });

function stubUseCase(execute: () => Promise<Result<AuthStoryDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    AUTH_COMPARE_DEMO_USE_CASE,
    { execute } as unknown as GetAuthStoryDemoUseCase,
  );
}

describe('AuthCompareDemo', () => {
  it('renders loading first, then the story with its beats and proofs', async () => {
    stubUseCase(async () => success(fixture));

    render(<AuthCompareDemo />);

    expect(screen.getByTestId('auth-compare-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('auth-compare-ready')).toBeInTheDocument());
    expect(screen.getByTestId('auth-compare-beat-jwt-hs256')).toHaveTextContent('STILL VALID');
    expect(screen.getByTestId('auth-compare-beat-session-cookie')).toHaveTextContent('dead');
    expect(screen.getByTestId('auth-compare-proofs')).toHaveTextContent('expiry is the only lever');
    expect(screen.getByTestId('auth-compare-reasoning')).toBeInTheDocument();
  });

  it('degrades to the error panel on failure, with recovery', async () => {
    const failure = new ServerFailure('The auth demo could not run its story.', {
      code: 'internal',
      status: 500,
      requestId: 'rid-auth-500',
    });
    stubUseCase(async () => failed(failure));

    render(<AuthCompareDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('internal');

    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('auth-compare-ready')).toBeInTheDocument());
  });
});
