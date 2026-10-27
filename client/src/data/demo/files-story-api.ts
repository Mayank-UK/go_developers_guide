import type { AppConfig } from '../../core/config/app-config';
import { guardedApiCall, guardedGetJson, parseDto } from '../../core/error-handling/guards';
import { filesStoryDtoFromJson, type FilesStoryDemoDto } from './files-story-dto';

/**
 * FilesStoryApi — the transport for the files topic's workbench story
 * endpoint (GET /api/demo/files/workbench). The request goes through
 * the shared guarded fetch pipeline (network down / non-JSON / non-2xx
 * error envelope all become typed AppExceptions), then the JSON
 * payload is validated into a DTO. The story is server-run against a
 * fresh private store per request, so this adapter is a plain GET with
 * no inputs — the five beats are always the server's own latest
 * answers.
 */
export class FilesStoryApi {
  private readonly config: AppConfig;
  private readonly path: string;

  constructor(config: AppConfig, path: string) {
    this.config = config;
    this.path = path;
  }

  /** GET {apiBaseUrl}<path> — the story's beats, proofs, and reasoning. */
  async get(): Promise<FilesStoryDemoDto> {
    return guardedApiCall(async () => {
      const { body, status } = await guardedGetJson(this.config, this.path);
      return parseDto(body, status, filesStoryDtoFromJson);
    });
  }
}
