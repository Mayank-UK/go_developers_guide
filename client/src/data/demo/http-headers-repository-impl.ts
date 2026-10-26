import type { Result } from '../../core/result';
import { safeRepositoryCall } from '../../core/error-handling/guards';
import type { HttpHeadersRepository } from '../../domain/demo/http-headers-repository';
import type { HttpHeadersDemoData } from '../../domain/demo/http-headers';
import { toHttpHeadersDemo } from './http-headers-dto';
import type { HttpHeadersApi } from './http-headers-api';

/**
 * HttpHeadersRepositoryImpl — adapts the demo API to the domain port.
 * Every thrown transport error (network, non-2xx, malformed payload) is
 * guarded by safeRepositoryCall into a Failed Result, so the demo
 * component never needs try/catch and no rejection goes unhandled.
 */
export class HttpHeadersRepositoryImpl implements HttpHeadersRepository {
  private readonly api: HttpHeadersApi;

  constructor(api: HttpHeadersApi) {
    this.api = api;
  }

  async get(): Promise<Result<HttpHeadersDemoData>> {
    return safeRepositoryCall(async () => toHttpHeadersDemo(await this.api.get()));
  }
}
