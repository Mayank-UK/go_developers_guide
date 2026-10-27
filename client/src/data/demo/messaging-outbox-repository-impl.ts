import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { MessagingOutboxRepository } from '../../domain/demo/messaging-outbox-repository';
import type { MessagingOutboxDemoData } from '../../domain/demo/messaging-outbox';
import { toMessagingOutboxDemo } from './messaging-outbox-dto';
import type { MessagingOutboxApi } from './messaging-outbox-api';

/**
 * MessagingOutboxRepositoryImpl — adapts the demo API to the domain
 * port. Every thrown transport error (network, non-2xx, malformed
 * payload) is guarded by safeRepositoryCall into a Failed Result, so the
 * demo component never needs try/catch and no rejection goes unhandled.
 */
export class MessagingOutboxRepositoryImpl implements MessagingOutboxRepository {
  private readonly api: MessagingOutboxApi;

  constructor(api: MessagingOutboxApi) {
    this.api = api;
  }

  async get(): Promise<Result<MessagingOutboxDemoData>> {
    return safeRepositoryCall(async () => toMessagingOutboxDemo(await this.api.get()));
  }
}
