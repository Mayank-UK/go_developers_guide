import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { MongoConsoleRepository } from '../../domain/demo/mongo-console-repository';
import type { MongoConsoleDemoData } from '../../domain/demo/mongo-console';
import { toMongoConsoleDemo } from './mongo-console-dto';
import type { MongoConsoleApi } from './mongo-console-api';

/**
 * MongoConsoleRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload) is
 * guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class MongoConsoleRepositoryImpl implements MongoConsoleRepository {
  private readonly api: MongoConsoleApi;

  constructor(api: MongoConsoleApi) {
    this.api = api;
  }

  async get(): Promise<Result<MongoConsoleDemoData>> {
    return safeRepositoryCall(async () => toMongoConsoleDemo(await this.api.get()));
  }
}
