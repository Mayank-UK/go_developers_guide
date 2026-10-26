import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { RaceDetectorDemo } from './race_demo';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import type { GetRaceDetectorDemoUseCase } from '../../../domain/demo/get-race-detector-demo-usecase';
import type { RaceDetectorDemoData } from '../../../domain/demo/race-detector';

/**
 * RaceDetectorDemo spec — fixture-driven (happy + failure). The component
 * takes its use case through an optional prop (the container's token
 * wiring for this demo lands with the wave's composition-root change, so
 * the spec injects directly instead of stubbing the container). Behavior,
 * not implementation: the pinned exhibit renders as code and as the
 * toolchain's verbatim verdict, both live totals report exact, the
 * hygiene contrast shows reader present versus absent, and failures
 * degrade to the shared error panel with recovery.
 */

const fixture: RaceDetectorDemoData = {
  race: {
    code: 'func RacyTotal(goroutines, increments int) int {\n\tcounter := 0\n\tvar wg sync.WaitGroup\n',
    command: 'go test -race -count=1 .',
    verdict:
      '==================\nWARNING: DATA RACE\nRead at 0x00c0000141f8 by goroutine 17:\n--- FAIL: TestRacyTotal (0.00s)\n    racy_test.go:7: racy total = 3900, want 5000\nFAIL',
    explanation: 'the race detector instruments loads and stores',
    fix: 'synchronize: guard the shared variable with a mutex, or pass ownership over a channel',
  },
  mutex: {
    strategy: 'sync.Mutex — every increment a critical section',
    goroutines: 200,
    incrementsEach: 25,
    expected: 5000,
    observed: 5000,
    exact: true,
  },
  channel: {
    strategy: 'channel fold — local tallies, one owner folds after the join',
    goroutines: 200,
    incrementsEach: 25,
    expected: 5000,
    observed: 5000,
    exact: true,
  },
  hygiene: [
    {
      case: 'unbuffered send with a waiting reader',
      readerPresent: true,
      outcome: 'delivered "done" — goroutine returned',
    },
    {
      case: 'unbuffered send after the reader is gone, guarded by select+default',
      readerPresent: false,
      outcome:
        'send abandoned by the default arm — goroutine returned (a plain send would block forever: the leak)',
    },
  ],
};

function stubUseCase(execute: () => Promise<Result<RaceDetectorDemoData>>): GetRaceDetectorDemoUseCase {
  return { execute } as unknown as GetRaceDetectorDemoUseCase;
}

describe('RaceDetectorDemo', () => {
  it('renders loading first, then the exhibit, both fixes, and hygiene', async () => {
    render(<RaceDetectorDemo useCase={stubUseCase(async () => success(fixture))} />);

    expect(screen.getByTestId('conc-race-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('rd-exhibit')).toBeInTheDocument());

    // The exhibit: the racy code, the command, the verbatim verdict.
    expect(screen.getByTestId('rd-exhibit')).toHaveTextContent('RacyTotal');
    expect(screen.getByTestId('rd-exhibit')).toHaveTextContent('go test -race -count=1 .');
    expect(screen.getByTestId('rd-verdict')).toHaveTextContent('WARNING: DATA RACE');
    expect(screen.getByTestId('rd-verdict')).toHaveTextContent('racy total = 3900, want 5000');

    // Both fixes run live and land exactly.
    expect(screen.getByTestId('rd-mutex-exact')).toHaveTextContent('exact');
    expect(screen.getByTestId('rd-mutex')).toHaveTextContent('200 goroutines × 25 increments');
    expect(screen.getByTestId('rd-channel-exact')).toHaveTextContent('exact');
    expect(screen.getByTestId('rd-channel')).toHaveTextContent('observed 5000');

    // Hygiene: delivered versus abandoned, reader present versus absent.
    expect(screen.getByTestId('rd-hygiene')).toHaveTextContent('reader present');
    expect(screen.getByTestId('rd-hygiene')).toHaveTextContent('reader absent');
    expect(screen.getByTestId('rd-hygiene')).toHaveTextContent('the leak');
  });

  it('degrades to the shared error panel on a Failed Result, with recovery', async () => {
    let calls = 0;
    render(
      <RaceDetectorDemo
        useCase={stubUseCase(async () => {
          calls += 1;
          return calls === 1
            ? failed(
                new ServerFailure('The demo endpoint is down.', {
                  code: 'service_unavailable',
                  status: 503,
                  requestId: 'req-62',
                }),
              )
            : success(fixture);
        })}
      />,
    );

    await waitFor(() => expect(screen.getByTestId('error-panel')).toBeInTheDocument());
    expect(screen.getByTestId('error-panel')).toHaveTextContent('service_unavailable');

    // Retry re-runs the use case and the payload renders.
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('rd-exhibit')).toBeInTheDocument());
    expect(screen.queryByTestId('error-panel')).not.toBeInTheDocument();
  });
});
