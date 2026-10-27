import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { FilesWorkbenchDemo } from './workbench_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetFilesWorkbenchDemoUseCase,
  FILES_WORKBENCH_DEMO_USE_CASE,
} from '../../../domain/demo/get-files-workbench-demo-usecase';
import type { FilesStoryDemoData } from '../../../domain/demo/files-story';
import { filesWorkbenchFixture } from './files_story_fixtures';

/**
 * FilesWorkbenchDemo spec — fixture-driven (happy + failure), the
 * family pattern: the component resolves its use case through the
 * container token, so the spec swaps that registration instead of
 * stubbing fetch. Behavior, not implementation: the topic's own five
 * beats render with their verdicts and facts (the upload in ok green,
 * the mid-stream refusal and the sweep in rejected red), the derived
 * proofs render, and failures degrade to the shared error panel with
 * recovery.
 */

const fixture: FilesStoryDemoData = filesWorkbenchFixture();

function stubUseCase(execute: () => Promise<Result<FilesStoryDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    FILES_WORKBENCH_DEMO_USE_CASE,
    { execute } as unknown as GetFilesWorkbenchDemoUseCase,
  );
}

describe('FilesWorkbenchDemo', () => {
  it('renders loading first, then the story with its five beats and proofs', async () => {
    stubUseCase(async () => success(fixture));

    render(<FilesWorkbenchDemo />);

    expect(screen.getByTestId('files-workbench-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('files-workbench-ready')).toBeInTheDocument());
    expect(screen.getByTestId('files-workbench-beat-the-streamed-upload')).toHaveTextContent(
      '4 MiB',
    );
    expect(screen.getByTestId('files-workbench-beat-the-size-bound-refuses-mid-stream')).toHaveTextContent(
      'nothing landed',
    );
    expect(screen.getByTestId('files-workbench-beat-the-ranged-download')).toHaveTextContent(
      'bytes */4194304',
    );
    expect(screen.getByTestId('files-workbench-beat-the-traversal-sweep')).toHaveTextContent(
      'EvalSymlinks',
    );
    expect(screen.getByTestId('files-workbench-beat-the-thumbnail')).toHaveTextContent(
      '30000×30000',
    );
    expect(screen.getByTestId('files-workbench-proofs')).toHaveTextContent('streaming honesty');
    expect(screen.getByTestId('files-workbench-reasoning')).toBeInTheDocument();
  });

  it('colors refusals as rejected and successes as ok', async () => {
    stubUseCase(async () => success(fixture));

    render(<FilesWorkbenchDemo />);

    await waitFor(() => expect(screen.getByTestId('files-workbench-ready')).toBeInTheDocument());
    expect(screen.getByTestId('files-workbench-beat-the-streamed-upload')).toHaveTextContent('ok');
    expect(screen.getByTestId('files-workbench-beat-the-size-bound-refuses-mid-stream')).toHaveTextContent(
      'rejected',
    );
  });

  it('degrades to the error panel on failure, with recovery', async () => {
    const failure = new ServerFailure('The files workbench demo could not run its story.', {
      code: 'service_unavailable',
      status: 503,
      requestId: 'rid-files-503',
    });
    stubUseCase(async () => failed(failure));

    render(<FilesWorkbenchDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('service_unavailable');

    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('files-workbench-ready')).toBeInTheDocument());
  });
});
