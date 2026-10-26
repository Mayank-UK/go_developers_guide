import type { Result } from '../../core/result';
import type { RaceDetectorRepository } from './race-detector-repository';
import type { RaceDetectorDemoData } from './race-detector';

/**
 * GetRaceDetectorDemoUseCase — loads the goroutines-and-channels topic's
 * race-detector demo payload. Thin orchestration seam over the repository
 * port (react-guide use-case style); parameterless because the demo has
 * no inputs — the pinned exhibit is constant and the synchronized totals
 * are computed live by the server.
 */
export class GetRaceDetectorDemoUseCase {
  private readonly repository: RaceDetectorRepository;

  constructor(repository: RaceDetectorRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<RaceDetectorDemoData>> {
    return this.repository.get();
  }
}
