import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { AuthzMatrixDemo } from './matrix_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetAuthzMatrixDemoUseCase,
  AUTHZ_MATRIX_DEMO_USE_CASE,
} from '../../../domain/demo/get-authz-matrix-demo-usecase';
import type { AuthzMatrixDemoData } from '../../../domain/demo/authz-matrix';

/**
 * AuthzMatrixDemo spec — fixture-driven (happy + failure): the component
 * resolves its use case through the container token (the documented
 * register-a-singleton seam), so the spec swaps that registration instead
 * of stubbing fetch. Behavior, not implementation: every beat renders
 * with its verdict and facts, the roster beat carries the registry rows,
 * the teaching beats carry the evaluator's own cell citations, the proofs
 * render with their derived tallies, and failures degrade to the shared
 * error panel with recovery.
 */

const fixture: AuthzMatrixDemoData = {
  mechanism: 'rbac-matrix',
  beats: [
    {
      beat: 'the roster',
      detail: 'RBAC starts as a table: role → permission set, defined in ONE registry.',
      verdict: 'info',
      facts: [
        { label: 'reader', value: 'posts:read' },
        { label: 'admin', value: 'posts:read posts:write users:manage' },
      ],
    },
    {
      beat: 'the principals',
      detail: 'Six callers, one for every row of the matrix.',
      verdict: 'info',
      facts: [
        { label: 'ada (author)', value: 'scopes [posts:read posts:write] via session' },
        { label: 'guest ()', value: 'scopes [*] via session — the deny-by-default control' },
      ],
    },
    {
      beat: 'owner, editor, stranger',
      detail:
        'ada posts:write p-sessions: allow — object-level pass: ada owns p-sessions · grace posts:write p-sessions: allow — editor writes span every post · ada posts:write p-draft: deny — object-level refusal: grace owns p-draft and ada is neither its owner, an editor, nor an admin.',
      verdict: 'ok',
      facts: [],
    },
    {
      beat: 'scopes narrow roles',
      detail: 'kim posts:write p-narrowed: deny — the credential narrowed what the role granted.',
      verdict: 'ok',
      facts: [],
    },
    {
      beat: 'deny by default',
      detail: 'guest posts:write p-sessions: deny — no role grants. anonymous denies first.',
      verdict: 'rejected',
      facts: [],
    },
    {
      beat: 'namespaces isolate',
      detail: 'root users:manage -: allow · ada users:manage -: deny.',
      verdict: 'ok',
      facts: [],
    },
  ],
  proofs: [
    { label: 'matrix size', value: '45 evaluated cells (7 subjects × actions × objects)' },
    { label: 'write column', value: '9 of 21 write cells allow — the rest deny at scope, role, or object gates' },
    { label: 'deny by default', value: 'guest (no role) denied every action: true' },
  ],
  reasoning:
    'Each gate only ever narrows the one before it, which is what makes the whole stack auditable.',
};

function stubUseCase(execute: () => Promise<Result<AuthzMatrixDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    AUTHZ_MATRIX_DEMO_USE_CASE,
    { execute } as unknown as GetAuthzMatrixDemoUseCase,
  );
}

describe('AuthzMatrixDemo', () => {
  it('renders loading first, then the whole matrix', async () => {
    stubUseCase(async () => success(fixture));

    render(<AuthzMatrixDemo />);

    expect(screen.getByTestId('authz-matrix-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('authz-matrix-ready')).toBeInTheDocument());

    // The mechanism line and every beat with its verdict and citations.
    expect(screen.getByTestId('authz-matrix-ready')).toHaveTextContent('rbac-matrix');
    const roster = screen.getByTestId('authz-matrix-beat-the-roster');
    expect(roster).toHaveTextContent('info');
    expect(roster).toHaveTextContent('admin: posts:read posts:write users:manage');
    const ladder = screen.getByTestId('authz-matrix-beat-owner-editor-stranger');
    expect(ladder).toHaveTextContent('object-level pass: ada owns p-sessions');
    expect(ladder).toHaveTextContent('object-level refusal: grace owns p-draft');
    expect(screen.getByTestId('authz-matrix-beat-scopes-narrow-roles')).toHaveTextContent(
      'the credential narrowed what the role granted',
    );
    expect(screen.getByTestId('authz-matrix-beat-deny-by-default')).toHaveTextContent('rejected');

    // The derived proofs and the closing reasoning.
    expect(screen.getByTestId('authz-matrix-proofs')).toHaveTextContent('45 evaluated cells');
    expect(screen.getByTestId('authz-matrix-proofs')).toHaveTextContent('9 of 21 write cells allow');
    expect(screen.getByTestId('authz-matrix-reasoning')).toHaveTextContent(
      'only ever narrows the one before it',
    );
  });

  it('degrades to the error panel on the server\u2019s honest failure, with recovery', async () => {
    const failure = new ServerFailure(
      'The authorization matrix demo could not run its story.',
      { code: 'internal', status: 500, requestId: 'rid-authz-500' },
    );
    stubUseCase(async () => failed(failure));

    render(<AuthzMatrixDemo />);

    const panel = await screen.findByRole('alert');
    expect(panel).toHaveTextContent('internal');
    expect(panel).toHaveTextContent('could not run its story');

    // Retry re-runs the load through the same use case seam.
    stubUseCase(async () => success(fixture));
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('authz-matrix-ready')).toBeInTheDocument());
  });
});
