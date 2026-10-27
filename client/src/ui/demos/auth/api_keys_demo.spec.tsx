import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { AuthAPIKeysDemo } from './api_keys_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetAuthStoryDemoUseCase,
  AUTH_API_KEYS_DEMO_USE_CASE,
} from '../../../domain/demo/get-auth-story-demo-usecase';
import type { AuthStoryDemoData } from '../../../domain/demo/auth-story';
import { authStoryFixture } from './auth_story_fixtures';

/**
 * AuthAPIKeysDemo spec — fixture-driven (happy + failure), the family
 * pattern: the component resolves its use case through the container
 * token, so the spec swaps that registration instead of stubbing
 * fetch. Behavior, not implementation: the demo's own distinctive
 * beats render with their verdicts and facts, the derived proofs
 * render, and failures degrade to the shared error panel with
 * recovery.
 */

const fixture: AuthStoryDemoData = authStoryFixture(
    {
      mechanism: 'api-keys',
      beats: [
        { beat: 'generate', detail: 'minted gdg_0123abcd: the store holds the SHA-256.', verdict: 'ok',
          facts: [{ label: 'raw key (shown once)', value: 'gdg_0123abcd.raw-secret' }] },
        { beat: 'call after revoke', detail: 'the very same key → refused.', verdict: 'rejected', facts: [] },
      ],
      proofs: [{ label: 'hashed at rest', value: 'the raw value appears in exactly one response' }],
      reasoning: 'generate, verify, revoke — three rules, all live.',
    });

function stubUseCase(execute: () => Promise<Result<AuthStoryDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    AUTH_API_KEYS_DEMO_USE_CASE,
    { execute } as unknown as GetAuthStoryDemoUseCase,
  );
}

describe('AuthAPIKeysDemo', () => {
  it('renders loading first, then the story with its beats and proofs', async () => {
    stubUseCase(async () => success(fixture));

    render(<AuthAPIKeysDemo />);

    expect(screen.getByTestId('auth-api-keys-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('auth-api-keys-ready')).toBeInTheDocument());
    expect(screen.getByTestId('auth-api-keys-beat-generate')).toHaveTextContent('gdg_0123abcd.raw-secret');
    expect(screen.getByTestId('auth-api-keys-beat-call-after-revoke')).toHaveTextContent('refused');
    expect(screen.getByTestId('auth-api-keys-proofs')).toHaveTextContent('hashed at rest');
    expect(screen.getByTestId('auth-api-keys-reasoning')).toBeInTheDocument();
  });

  it('degrades to the error panel on failure, with recovery', async () => {
    const failure = new ServerFailure('The auth demo could not run its story.', {
      code: 'internal',
      status: 500,
      requestId: 'rid-auth-500',
    });
    stubUseCase(async () => failed(failure));

    render(<AuthAPIKeysDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('internal');

    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('auth-api-keys-ready')).toBeInTheDocument());
  });
});
