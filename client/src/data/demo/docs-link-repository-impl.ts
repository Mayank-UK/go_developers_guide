import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { DocsLinkRepository } from '../../domain/demo/docs-link-repository';
import type { DocsLinkDemoData } from '../../domain/demo/docs-link';
import { toDocsLinkDemo } from './docs-link-dto';
import type { DocsLinkApi } from './docs-link-api';

/**
 * DocsLinkRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload) is
 * guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class DocsLinkRepositoryImpl implements DocsLinkRepository {
  private readonly api: DocsLinkApi;

  constructor(api: DocsLinkApi) {
    this.api = api;
  }

  async get(): Promise<Result<DocsLinkDemoData>> {
    return safeRepositoryCall(async () => toDocsLinkDemo(await this.api.get()));
  }
}
