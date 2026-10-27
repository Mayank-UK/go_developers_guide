import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { AuthJWTDemo } from './jwt_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetAuthStoryDemoUseCase,
  AUTH_JWT_DEMO_USE_CASE,
} from '../../../domain/demo/get-auth-story-demo-usecase';
import type { AuthStoryDemoData } from '../../../domain/demo/auth-story';
import { authStoryFixture } from './auth_story_fixtures';

/**
 * AuthJWTDemo spec — fixture-driven (happy + failure), the family
 * pattern: the component resolves its use case through the container
 * token, so the spec swaps that registration instead of stubbing
 * fetch. Behavior, not implementation: the demo's own distinctive
 * beats render with their verdicts and facts, the derived proofs
 * render, and failures degrade to the shared error panel with
 * recovery.
 */

const fixture: AuthStoryDemoData = authStoryFixture(
    {
      mechanism: 'jwt',
      beats: [
        { beat: 'mint', detail: 'three dots-joined parts for ada: header, claims, HMAC signature.', verdict: 'ok',
          facts: [{ label: 'header', value: '{"alg":"HS256","typ":"JWT"}' }] },
        { beat: 'alg=none attack', detail: '→ alg-none-rejected. the header declares alg=none — an UNSIGNED token.', verdict: 'rejected', facts: [] },
      ],
      proofs: [{ label: 'alg=none', value: 'refused BEFORE any signature exists to check' }],
      reasoning: 'the mechanism is three base64url parts and an HMAC.',
    });

function stubUseCase(execute: () => Promise<Result<AuthStoryDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    AUTH_JWT_DEMO_USE_CASE,
    { execute } as unknown as GetAuthStoryDemoUseCase,
  );
}

describe('AuthJWTDemo', () => {
  it('renders loading first, then the story with its beats and proofs', async () => {
    stubUseCase(async () => success(fixture));

    render(<AuthJWTDemo />);

    expect(screen.getByTestId('auth-jwt-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('auth-jwt-ready')).toBeInTheDocument());
    expect(screen.getByTestId('auth-jwt-beat-alg-none-attack')).toHaveTextContent('alg-none-rejected');
    expect(screen.getByTestId('auth-jwt-beat-mint')).toHaveTextContent('HS256');
    expect(screen.getByTestId('auth-jwt-proofs')).toHaveTextContent('refused BEFORE any signature exists');
    expect(screen.getByTestId('auth-jwt-reasoning')).toBeInTheDocument();
  });

  it('degrades to the error panel on failure, with recovery', async () => {
    const failure = new ServerFailure('The auth demo could not run its story.', {
      code: 'internal',
      status: 500,
      requestId: 'rid-auth-500',
    });
    stubUseCase(async () => failed(failure));

    render(<AuthJWTDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('internal');

    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('auth-jwt-ready')).toBeInTheDocument());
  });
});
