import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { RaceDemo } from './race_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { NetworkFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetRaceDemoUseCase,
  DB_RACE_DEMO_USE_CASE,
} from '../../../domain/demo/get-db-race-demo-usecase';
import type { RaceDemoData } from '../../../domain/demo/db-race';

const fixture: RaceDemoData = {
  isolation: 'read committed',
  scratchBefore: 'race demo scratch',
  scratchAfter: 'race demo scratch (updated)',
  steps: [
    { phase: 'uncommitted-write', actor: 'T1 (writer)', action: 'BEGIN; UPDATE … (uncommitted)', observed: 'title updated inside T1 only', note: 'invisible to others' },
    { phase: 'reader-looks', actor: 'T2 (reader)', action: 'SELECT title …', observed: 'race demo scratch', note: 'fresh snapshot per statement' },
    { phase: 'reader-looks-again', actor: 'T2 (reader)', action: 'SELECT title … (again)', observed: 'race demo scratch (updated)', note: 'the commit is visible now' },
    { phase: 'lock-waited', actor: 'T2 (reader)', action: 'SELECT … FOR UPDATE (waits)', observed: 'waited for the lock, then read "race demo scratch (updated)"', note: 'writers serialize on the row' },
  ],
  lockWaited: true,
  serializableNote: 'Under SERIALIZABLE one transaction would abort and the app would retry.',
};

function stubUseCase(execute: () => Promise<Result<RaceDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(DB_RACE_DEMO_USE_CASE, { execute } as unknown as GetRaceDemoUseCase);
}

describe('RaceDemo', () => {
  it('renders the choreography with the old/new observations and the lock verdict', async () => {
    stubUseCase(async () => success(fixture));
    render(<RaceDemo />);
    await waitFor(() => expect(screen.getByTestId('race-ready')).toBeInTheDocument());
    expect(screen.getByTestId('race-header')).toHaveTextContent('read committed');
    expect(screen.getByTestId('race-header')).toHaveTextContent('race demo scratch → race demo scratch (updated)');
    const steps = screen.getByTestId('race-steps');
    expect(steps).toHaveTextContent('saw: race demo scratch');
    expect(steps).toHaveTextContent('saw: race demo scratch (updated)');
    expect(steps).toHaveTextContent('waited for the lock');
    expect(screen.getByTestId('race-verdict')).toHaveTextContent('the reader WAITED for the row lock');
    expect(screen.getByTestId('race-verdict')).toHaveTextContent('SERIALIZABLE');
  });

  it('degrades to the error panel on transport failure', async () => {
    stubUseCase(async () => failed(new NetworkFailure('Could not reach the server.')));
    render(<RaceDemo />);
    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('network_error');
  });
});
