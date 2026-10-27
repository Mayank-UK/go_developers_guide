import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { MessagingStreamRepository } from '../../domain/demo/messaging-stream-repository';
import type { MessagingStreamDemoData } from '../../domain/demo/messaging-stream';
import { toMessagingStreamDemo } from './messaging-stream-dto';
import type { MessagingStreamApi } from './messaging-stream-api';

/**
 * MessagingStreamRepositoryImpl — adapts the demo API to the domain
 * port. Every thrown transport error (network, non-2xx, malformed
 * payload) is guarded by safeRepositoryCall into a Failed Result, so the
 * demo component never needs try/catch and no rejection goes unhandled.
 */
export class MessagingStreamRepositoryImpl implements MessagingStreamRepository {
  private readonly api: MessagingStreamApi;

  constructor(api: MessagingStreamApi) {
    this.api = api;
  }

  async get(): Promise<Result<MessagingStreamDemoData>> {
    return safeRepositoryCall(async () => toMessagingStreamDemo(await this.api.get()));
  }
}
