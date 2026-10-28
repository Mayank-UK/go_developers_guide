import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { BestChecklistDemo } from './checklist_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetBestChecklistDemoUseCase,
  BEST_CHECKLIST_DEMO_USE_CASE,
} from '../../../domain/demo/get-best-checklist-demo-usecase';
import type { BestChecklistDemoData } from '../../../domain/demo/best-checklist';

/**
 * BestChecklistDemo spec — fixture-driven (happy + placeholder +
 * failure): the component resolves its use case through the container
 * token. Behavior, not implementation: the summary counts render
 * unspun; every verdict row renders with its check kind, its evidence
 * lines, and — for manual rows — its why instead; the anti-checklist
 * renders its not-dos with reasons; the honest computing placeholder
 * (empty areas) is rendered as data, not an error; the replay button
 * drives a fresh execute; and failures degrade to the shared error
 * panel with recovery.
 */

const fixture: BestChecklistDemoData = {
  mode: 'boot-cached: the whole audit executes ONCE at process boot',
  computedAt: '2026-10-28T09:15:04Z',
  areas: [
    {
      id: 'setup',
      title: 'Setup',
      note: 'Toolchain discipline — format and pins that hold the floor.',
      items: [
        {
          id: 'gofmt-clean',
          practice: 'gofmt is non-negotiable — gofmt -l over the server tree lists nothing',
          checkKind: 'exec',
          verdict: 'pass',
          evidence: ['command: gofmt -l . (in server/)'],
        },
        {
          id: 'module-pins',
          practice: 'Dependencies are pinned, exactly',
          checkKind: 'file',
          verdict: 'pass',
          evidence: ['server/go.mod: go 1.27.3 directive, 15 direct requires (all exact-pinned)'],
        },
      ],
    },
    {
      id: 'human-practices',
      title: 'Human practices',
      note: 'The judgments no checklist can make for you.',
      items: [
        {
          id: 'prod-secrets-real',
          practice: 'Production overrides every dev-default secret with real values',
          checkKind: 'manual',
          verdict: 'manual',
          evidence: [],
          whyManual: 'an operator fact no local check can see',
        },
      ],
    },
  ],
  notDones: [
    {
      id: 'stdlib-first',
      notDone: 'No web framework, no test framework — stdlib first',
      reason: 'net/http is the backbone; chi, gin, and echo are taught by comparison only.',
      artifacts: ['server/cmd/guide/main.go — the whole surface on net/http'],
    },
  ],
  summary: { items: 15, passed: 13, failed: 0, manual: 2 },
  honestyNote: 'Every machine verdict on this panel was computed from machinery that actually executed.',
};

const placeholder: BestChecklistDemoData = {
  mode: 'computing: the boot audit is still executing',
  computedAt: '',
  areas: [],
  notDones: [],
  summary: { items: 0, passed: 0, failed: 0, manual: 0 },
  honestyNote: '',
};

function stubUseCase(execute: () => Promise<Result<BestChecklistDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    BEST_CHECKLIST_DEMO_USE_CASE,
    { execute } as unknown as GetBestChecklistDemoUseCase,
  );
}

describe('BestChecklistDemo', () => {
  it('renders loading first, then the summary, areas, evidence rows, not-dos, and honesty', async () => {
    stubUseCase(async () => success(structuredClone(fixture)));

    render(<BestChecklistDemo />);

    expect(screen.getByTestId('best-checklist-loading')).toBeInTheDocument();

    await screen.findByTestId('best-checklist-ready');

    const summary = screen.getByTestId('best-checklist-summary');
    expect(summary).toHaveTextContent('13 pass · 0 fail · 2 manual · of 15 rows');
    expect(summary).toHaveTextContent('computed at 2026-10-28T09:15:04Z');

    expect(screen.getByTestId('bp-area-setup')).toHaveTextContent('Toolchain discipline');
    expect(screen.getByTestId('bp-row-gofmt-clean')).toHaveTextContent('gofmt -l . (in server/)');
    expect(screen.getByTestId('bp-row-module-pins')).toHaveTextContent('15 direct requires');

    const manual = screen.getByTestId('bp-row-prod-secrets-real');
    expect(manual).toHaveTextContent('manual');
    expect(manual).toHaveTextContent('why manual: an operator fact no local check can see');

    const notDone = screen.getByTestId('bp-not-done-stdlib-first');
    expect(notDone).toHaveTextContent('No web framework, no test framework');
    expect(notDone).toHaveTextContent('taught by comparison only');

    expect(screen.getByTestId('best-checklist-honesty')).toHaveTextContent('machinery that actually executed');
  });

  it('renders the honest computing placeholder as data, never as an error', async () => {
    stubUseCase(async () => success(structuredClone(placeholder)));

    render(<BestChecklistDemo />);

    await screen.findByTestId('best-checklist-ready');

    expect(screen.getByTestId('best-checklist-computing')).toBeInTheDocument();
    expect(screen.getByTestId('best-checklist-summary')).toHaveTextContent('0 pass · 0 fail · 0 manual');
    expect(screen.queryByTestId('best-checklist-not-dones')).not.toBeInTheDocument();
  });

  it('re-serves the same snapshot when the replay button fires', async () => {
    const execute = vi.fn(async () => success(structuredClone(fixture)));
    stubUseCase(execute);

    render(<BestChecklistDemo />);
    await screen.findByTestId('best-checklist-ready');

    fireEvent.click(screen.getByTestId('best-checklist-replay'));

    await waitFor(() => expect(execute).toHaveBeenCalledTimes(2));
    expect(await screen.findByTestId('best-checklist-ready')).toBeInTheDocument();
  });

  it('degrades to the shared error panel with recovery', async () => {
    stubUseCase(async () =>
      failed(new ServerFailure('the self-audit endpoint is unreachable')),
    );

    render(<BestChecklistDemo />);

    const errorPanel = await screen.findByTestId('error-panel');
    expect(errorPanel).toHaveTextContent('the self-audit endpoint is unreachable');
  });
});
