import type { Result } from '../../core/result';
import type { TestingRunnerRepository } from './testing-runner-repository';
import type { TestingRunnerDemoData } from './testing-runner';

/**
 * TESTING_RUNNER_DEMO_USE_CASE — the container token this demo's use
 * case is registered under (same aliasing story as every demo's token);
 * the composition root registers the real pipeline under this exact
 * symbol:
 *
 *   container.register(TESTING_RUNNER_DEMO_USE_CASE, new GetTestingRunnerDemoUseCase(repository));
 */
export const TESTING_RUNNER_DEMO_USE_CASE = Symbol('GetTestingRunnerDemoUseCase');

/**
 * GetTestingRunnerDemoUseCase — loads the testing topic's runner
 * payload. Parameterless: the server executed its curated run once at
 * boot, so each load re-serves the cached timeline with a fresh
 * wall-clock age.
 */
export class GetTestingRunnerDemoUseCase {
  private readonly repository: TestingRunnerRepository;

  constructor(repository: TestingRunnerRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<TestingRunnerDemoData>> {
    return this.repository.get();
  }
}
