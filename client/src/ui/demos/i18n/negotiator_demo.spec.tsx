import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { I18nNegotiatorDemo } from './negotiator_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetI18nNegotiatorDemoUseCase,
  I18N_NEGOTIATOR_DEMO_USE_CASE,
} from '../../../domain/demo/get-i18n-negotiator-demo-usecase';
import type { I18nStoryDemoData } from '../../../domain/demo/i18n-story';
import { i18nNegotiatorFixture } from './i18n_story_fixtures';

/**
 * I18nNegotiatorDemo spec — fixture-driven (happy + failure), the
 * family pattern: the component resolves its use case through the
 * container token, so the spec swaps that registration instead of
 * stubbing fetch. Behavior, not implementation: the topic's own seven
 * beats render with their verdicts and facts (the parsed header in
 * muted info, the match in ok green), the derived proofs render, and
 * failures degrade to the shared error panel with recovery.
 */

const fixture: I18nStoryDemoData = i18nNegotiatorFixture();

function stubUseCase(execute: () => Promise<Result<I18nStoryDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    I18N_NEGOTIATOR_DEMO_USE_CASE,
    { execute } as unknown as GetI18nNegotiatorDemoUseCase,
  );
}

describe('I18nNegotiatorDemo', () => {
  it('renders loading first, then the story with its seven beats and proofs', async () => {
    stubUseCase(async () => success(fixture));

    render(<I18nNegotiatorDemo />);

    expect(screen.getByTestId('i18n-negotiator-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('i18n-negotiator-ready')).toBeInTheDocument());
    expect(screen.getByTestId('i18n-negotiator-beat-the-header-parsed')).toHaveTextContent(
      'fr-CH, fr;q=0.9',
    );
    expect(screen.getByTestId('i18n-negotiator-beat-the-match')).toHaveTextContent('"fr" catalog via base');
    expect(screen.getByTestId('i18n-negotiator-beat-one-message-every-catalog')).toHaveTextContent(
      'Willkommen zur\u00fcck, Ada!',
    );
    expect(screen.getByTestId('i18n-negotiator-beat-plurals-across-locales')).toHaveTextContent(
      'fr: 0 and 1',
    );
    expect(screen.getByTestId('i18n-negotiator-beat-locale-formats')).toHaveTextContent(
      '18,99 \u20ac',
    );
    expect(screen.getByTestId('i18n-negotiator-beat-timezone-is-not-locale')).toHaveTextContent(
      'unknown time zone Mars/Olympus',
    );
    expect(screen.getByTestId('i18n-negotiator-beat-the-honest-edges')).toHaveTextContent(
      'never a 500',
    );
    expect(screen.getByTestId('i18n-negotiator-proofs')).toHaveTextContent('separators at a glance');
    expect(screen.getByTestId('i18n-negotiator-reasoning')).toBeInTheDocument();
  });

  it('colors info beats muted and match beats ok', async () => {
    stubUseCase(async () => success(fixture));

    render(<I18nNegotiatorDemo />);

    await waitFor(() => expect(screen.getByTestId('i18n-negotiator-ready')).toBeInTheDocument());
    expect(screen.getByTestId('i18n-negotiator-beat-the-header-parsed')).toHaveTextContent('info');
    expect(screen.getByTestId('i18n-negotiator-beat-the-match')).toHaveTextContent('ok');
  });

  it('degrades to the error panel on failure, with recovery', async () => {
    const failure = new ServerFailure('The i18n negotiator demo could not run its story.', {
      code: 'service_unavailable',
      status: 503,
      requestId: 'rid-i18n-503',
    });
    stubUseCase(async () => failed(failure));

    render(<I18nNegotiatorDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('service_unavailable');

    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('i18n-negotiator-ready')).toBeInTheDocument());
  });
});
