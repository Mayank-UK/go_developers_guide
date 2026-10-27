import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { AuthRefreshDemo } from './refresh_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetAuthStoryDemoUseCase,
  AUTH_REFRESH_DEMO_USE_CASE,
} from '../../../domain/demo/get-auth-story-demo-usecase';
import type { AuthStoryDemoData } from '../../../domain/demo/auth-story';
import { authStoryFixture } from './auth_story_fixtures';

/**
 * AuthRefreshDemo spec — fixture-driven (happy + failure), the family
 * pattern: the component resolves its use case through the container
 * token, so the spec swaps that registration instead of stubbing
 * fetch. Behavior, not implementation: the demo's own distinctive
 * beats render with their verdicts and facts, the derived proofs
 * render, and failures degrade to the shared error panel with
 * recovery.
 */

const fixture: AuthStoryDemoData = authStoryFixture(
    {
      mechanism: 'refresh',
      beats: [
        { beat: 'login', detail: 'family fam_abc born for ada at position 1.', verdict: 'ok', facts: [] },
        { beat: 'REUSE detected', detail: 'the ENTIRE family is revoked — 3 tokens killed. This is the alarm working.', verdict: 'alarm', facts: [] },
      ],
      proofs: [{ label: 'reuse detection', value: 'a spent token presented again revoked the whole family: 3 tokens killed' }],
      reasoning: 'rotation plus reuse detection turns a stolen token into an alarm.',
    });

function stubUseCase(execute: () => Promise<Result<AuthStoryDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    AUTH_REFRESH_DEMO_USE_CASE,
    { execute } as unknown as GetAuthStoryDemoUseCase,
  );
}

describe('AuthRefreshDemo', () => {
  it('renders loading first, then the story with its beats and proofs', async () => {
    stubUseCase(async () => success(fixture));

    render(<AuthRefreshDemo />);

    expect(screen.getByTestId('auth-refresh-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('auth-refresh-ready')).toBeInTheDocument());
    expect(screen.getByTestId('auth-refresh-beat-reuse-detected')).toHaveTextContent('alarm');
    expect(screen.getByTestId('auth-refresh-beat-login')).toHaveTextContent('fam_abc');
    expect(screen.getByTestId('auth-refresh-proofs')).toHaveTextContent('3 tokens killed');
    expect(screen.getByTestId('auth-refresh-reasoning')).toBeInTheDocument();
  });

  it('degrades to the error panel on failure, with recovery', async () => {
    const failure = new ServerFailure('The auth demo could not run its story.', {
      code: 'internal',
      status: 500,
      requestId: 'rid-auth-500',
    });
    stubUseCase(async () => failed(failure));

    render(<AuthRefreshDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('internal');

    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('auth-refresh-ready')).toBeInTheDocument());
  });
});
