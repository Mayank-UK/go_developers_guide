import type { Result } from '../../core/result';
import type { GRPCConsoleRepository } from './grpc-console-repository';
import type { GRPCConsoleDemoData } from './grpc-console';

/**
 * API_GRPC_CONSOLE_DEMO_USE_CASE — the container token this demo's use
 * case is registered under. Lives here (not in container.ts's
 * ContainerTokens) so the demo component has its resolution seam from day
 * one; the composition root registers the real pipeline under this exact
 * symbol:
 *
 *   container.register(API_GRPC_CONSOLE_DEMO_USE_CASE, new GetGRPCConsoleDemoUseCase(repository));
 */
export const API_GRPC_CONSOLE_DEMO_USE_CASE = Symbol('GetGRPCConsoleDemoUseCase');

/**
 * GetGRPCConsoleDemoUseCase — loads the grpc topic's console demo
 * payload. Thin orchestration seam over the repository port
 * (react-guide use-case style); parameterless because the exhibit script
 * is fixed server-side — every fetch runs the whole in-process stack
 * (server, bufconn dial, six calls, teardown) and answers with the
 * reports.
 */
export class GetGRPCConsoleDemoUseCase {
  private readonly repository: GRPCConsoleRepository;

  constructor(repository: GRPCConsoleRepository) {
    this.repository = repository;
  }

  execute(): Promise<Result<GRPCConsoleDemoData>> {
    return this.repository.get();
  }
}
