import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { FilesStoryRepository } from '../../domain/demo/files-story-repository';
import type { FilesStoryDemoData } from '../../domain/demo/files-story';
import { toFilesStoryDemo } from './files-story-dto';
import type { FilesStoryApi } from './files-story-api';

/**
 * FilesStoryRepositoryImpl — adapts the workbench story API to the
 * domain port. Every thrown transport error (network, non-2xx,
 * malformed payload) is guarded by safeRepositoryCall into a Failed
 * Result, so the demo component never needs try/catch and no rejection
 * goes unhandled.
 */
export class FilesStoryRepositoryImpl implements FilesStoryRepository {
  private readonly api: FilesStoryApi;

  constructor(api: FilesStoryApi) {
    this.api = api;
  }

  async get(): Promise<Result<FilesStoryDemoData>> {
    return safeRepositoryCall(async () => toFilesStoryDemo(await this.api.get()));
  }
}
