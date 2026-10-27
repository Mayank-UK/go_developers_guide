import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { AuthHashingDemo } from './hashing_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetAuthStoryDemoUseCase,
  AUTH_HASHING_DEMO_USE_CASE,
} from '../../../domain/demo/get-auth-story-demo-usecase';
import type { AuthStoryDemoData } from '../../../domain/demo/auth-story';
import { authStoryFixture } from './auth_story_fixtures';

/**
 * AuthHashingDemo spec — fixture-driven (happy + failure), the family
 * pattern: the component resolves its use case through the container
 * token, so the spec swaps that registration instead of stubbing
 * fetch. Behavior, not implementation: the demo's own distinctive
 * beats render with their verdicts and facts, the derived proofs
 * render, and failures degrade to the shared error panel with
 * recovery.
 */

const fixture: AuthStoryDemoData = authStoryFixture(
    {
      mechanism: 'hashing',
      beats: [
        { beat: 'hash the same password twice', detail: 'distinct=true, a fresh 16-byte salt rides inside every hash.', verdict: 'ok', facts: [] },
        { beat: 'verify: wrong password', detail: 'the rejection still burned the full 41.9 ms run.', verdict: 'rejected', facts: [] },
      ],
      proofs: [{ label: 'salting', value: 'the same password hashed twice is distinct (true)' }],
      reasoning: 'hashing is one-way; the cost is the point.',
    });

function stubUseCase(execute: () => Promise<Result<AuthStoryDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    AUTH_HASHING_DEMO_USE_CASE,
    { execute } as unknown as GetAuthStoryDemoUseCase,
  );
}

describe('AuthHashingDemo', () => {
  it('renders loading first, then the story with its beats and proofs', async () => {
    stubUseCase(async () => success(fixture));

    render(<AuthHashingDemo />);

    expect(screen.getByTestId('auth-hashing-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('auth-hashing-ready')).toBeInTheDocument());
    expect(screen.getByTestId('auth-hashing-beat-hash-the-same-password-twice')).toHaveTextContent('distinct=true');
    expect(screen.getByTestId('auth-hashing-beat-verify-wrong-password')).toHaveTextContent('burned the full');
    expect(screen.getByTestId('auth-hashing-proofs')).toHaveTextContent('distinct (true)');
    expect(screen.getByTestId('auth-hashing-reasoning')).toBeInTheDocument();
  });

  it('degrades to the error panel on failure, with recovery', async () => {
    const failure = new ServerFailure('The auth demo could not run its story.', {
      code: 'internal',
      status: 500,
      requestId: 'rid-auth-500',
    });
    stubUseCase(async () => failed(failure));

    render(<AuthHashingDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('internal');

    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('auth-hashing-ready')).toBeInTheDocument());
  });
});
