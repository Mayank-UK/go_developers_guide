import type { Result } from '../../core/result';
import type { FilesStoryRepository } from './files-story-repository';
import type { FilesStoryDemoData } from './files-story';

/**
 * The container token for the files topic's workbench demo, so the
 * component has its resolution seam from day one; the composition root
 * registers a GetFilesWorkbenchDemoUseCase under it.
 */
export const FILES_WORKBENCH_DEMO_USE_CASE = Symbol('GetFilesWorkbenchDemoUseCase');

/**
 * GetFilesWorkbenchDemoUseCase — loads the `files:workbench` story
 * payload. Parameterless because the demo has no user inputs: the
 * server runs the whole five-beat story fresh against a private store
 * per request (streamed upload, mid-stream refusal, range math, the
 * traversal sweep with its symlink escape, and the bounded thumbnail),
 * so every fetch tells the whole story again.
 */
export class GetFilesWorkbenchDemoUseCase {
  private readonly repository: FilesStoryRepository;

  constructor(repository: FilesStoryRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<FilesStoryDemoData>> {
    return this.repository.get();
  }
}
