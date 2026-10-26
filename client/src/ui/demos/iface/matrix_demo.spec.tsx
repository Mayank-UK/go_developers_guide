import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { InterfaceMatrixDemo } from './matrix_demo';
import { container } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import {
  GetInterfaceMatrixDemoUseCase,
  INTERFACE_MATRIX_DEMO_USE_CASE,
} from '../../../domain/demo/get-interface-matrix-demo-usecase';
import type { InterfaceMatrixDemoData } from '../../../domain/demo/interface-matrix';

/**
 * InterfaceMatrixDemo spec — fixture-driven (happy + failure): the
 * component resolves its use case through the container token (the
 * documented register-a-singleton seam), so the spec swaps that
 * registration instead of stubbing fetch. Behavior, not implementation:
 * all five payload faces render, verdicts render as computed ✓/— marks,
 * the empty exported-methods list and the absent optional note render
 * honestly, and failures degrade to the shared error panel with recovery.
 *
 * (The demo-registry binding test lands with the composition-root wiring —
 * this slice ships before demo-registry.tsx grows its lazy entry.)
 */

const fixture: InterfaceMatrixDemoData = {
  interfaces: [
    { name: 'fmt.Stringer', method: 'String() string', pkg: 'fmt', note: "One method — fmt's whole formatting world keys off it." },
    { name: 'error', method: 'Error() string', pkg: 'builtin', note: 'The language\u2019s own interface.' },
    { name: 'io.Reader', method: 'Read([]byte) (int, error)', pkg: 'io', note: 'The canon.' },
    { name: 'http.Handler', method: 'ServeHTTP(http.ResponseWriter, *http.Request)', pkg: 'net/http', note: 'One method, the entire HTTP layer.' },
    { name: 'content.Block', method: 'blockKind() string', pkg: 'guide/internal/content', note: 'Unexported method — a sealed hierarchy.' },
  ],
  rows: [
    {
      expression: '*core.Error',
      type: '*core.Error',
      kind: 'ptr',
      methods: ['Error', 'Is', 'Unwrap'],
      cells: [
        { iface: 'fmt.Stringer', satisfied: false },
        { iface: 'error', satisfied: true },
        { iface: 'io.Reader', satisfied: false },
        { iface: 'http.Handler', satisfied: false },
        { iface: 'content.Block', satisfied: false },
      ],
    },
    {
      expression: 'core.Error',
      type: 'core.Error',
      kind: 'struct',
      methods: [],
      cells: [
        { iface: 'fmt.Stringer', satisfied: false },
        { iface: 'error', satisfied: false },
        { iface: 'io.Reader', satisfied: false },
        { iface: 'http.Handler', satisfied: false },
        { iface: 'content.Block', satisfied: false },
      ],
    },
    {
      expression: 'content.TextBlock{}',
      type: 'content.TextBlock',
      kind: 'struct',
      methods: [],
      cells: [
        { iface: 'fmt.Stringer', satisfied: false },
        { iface: 'error', satisfied: false },
        { iface: 'io.Reader', satisfied: false },
        { iface: 'http.Handler', satisfied: false },
        { iface: 'content.Block', satisfied: true },
      ],
    },
  ],
  embeddings: [
    {
      expression: 'type taggedReader struct { *strings.Reader; tag string }',
      embeds: '*strings.Reader (concrete type)',
      type: 'demos.taggedReader',
      iface: 'io.Reader',
      satisfied: true,
      note: 'Read arrives by promotion — taggedReader declares zero methods.',
    },
    {
      expression: 'type delegatingHandler struct { http.Handler }',
      embeds: 'http.Handler (interface)',
      type: 'demos.delegatingHandler',
      iface: 'http.Handler',
      satisfied: true,
      note: 'An embedded interface promotes its whole method set.',
    },
  ],
  typeSwitch: [
    { input: 'content.Topic{}', matchedCase: 'default', result: 'unhandled content.Topic' },
    {
      input: '*bytes.Buffer',
      matchedCase: 'case io.Reader',
      result: 'io.Reader case — dynamic type *bytes.Buffer',
      note: '*bytes.Buffer satisfies fmt.Stringer too — cases run top to bottom, and the first match wins.',
    },
    { input: 'time.Duration', matchedCase: 'case fmt.Stringer', result: 'fmt.Stringer case — 1m30s' },
  ],
  compileProofs: [
    { line: '_ error         = (*core.Error)(nil)', pins: 'error × *core.Error' },
    { line: '_ io.Reader     = taggedReader{}', pins: 'io.Reader × taggedReader (promoted)' },
  ],
};

function stubUseCase(execute: () => Promise<Result<InterfaceMatrixDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    INTERFACE_MATRIX_DEMO_USE_CASE,
    { execute } as unknown as GetInterfaceMatrixDemoUseCase,
  );
}

describe('InterfaceMatrixDemo', () => {
  it('renders loading first, then all five payload faces', async () => {
    stubUseCase(async () => success(fixture));

    render(<InterfaceMatrixDemo />);

    expect(screen.getByTestId('interface-matrix-loading')).toBeInTheDocument();

    await waitFor(() => expect(screen.getByTestId('im-interfaces')).toBeInTheDocument());

    // Columns: the five interfaces with their real method signatures.
    expect(screen.getByTestId('im-interfaces')).toHaveTextContent('Read([]byte) (int, error)');
    expect(screen.getByTestId('im-interfaces')).toHaveTextContent('blockKind() string');
    expect(screen.getByTestId('im-interfaces')).toHaveTextContent('guide/internal/content');

    // The matrix: every row renders, verdicts render as marks, and the
    // pointer-vs-value pair reads at a glance (✓ exactly where satisfied).
    const matrix = screen.getByTestId('im-matrix');
    expect(matrix).toHaveTextContent('*core.Error');
    expect(matrix).toHaveTextContent('core.Error');
    expect(matrix).toHaveTextContent('content.TextBlock{}');
    expect(matrix).toHaveTextContent('✓');
    expect(matrix).toHaveTextContent('Error, Is, Unwrap');

    // Embeddings: promotion exhibits with their satisfied verdicts.
    expect(screen.getByTestId('im-embeddings')).toHaveTextContent('taggedReader');
    expect(screen.getByTestId('im-embeddings')).toHaveTextContent('delegatingHandler');

    // Type switch: the default row without a note, and the order-lesson
    // row with it.
    const typeSwitch = screen.getByTestId('im-type-switch');
    expect(typeSwitch).toHaveTextContent('unhandled content.Topic');
    expect(typeSwitch).toHaveTextContent('first match wins');
    expect(typeSwitch).toHaveTextContent('1m30s');

    // Compile proofs: the var _ lines with the cells they pin (regex — the
    // source lines carry gofmt alignment runs the text matcher collapses).
    expect(screen.getByTestId('im-proofs')).toHaveTextContent(
      /_\s+error\s+=\s+\(\*core\.Error\)\(nil\)/,
    );
    expect(screen.getByTestId('im-proofs')).toHaveTextContent('error × *core.Error');
  });

  it('degrades to the shared error panel on a Failed Result, with recovery', async () => {
    let calls = 0;
    stubUseCase(async () => {
      calls += 1;
      return calls === 1
        ? failed(
            new ServerFailure('The demo endpoint is down.', {
              code: 'service_unavailable',
              status: 503,
              requestId: 'req-31',
            }),
          )
        : success(fixture);
    });

    render(<InterfaceMatrixDemo />);

    await waitFor(() => expect(screen.getByTestId('error-panel')).toBeInTheDocument());
    expect(screen.getByTestId('error-panel')).toHaveTextContent('service_unavailable');

    // Retry re-runs the use case and the payload renders.
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('im-interfaces')).toBeInTheDocument());
    expect(screen.queryByTestId('error-panel')).not.toBeInTheDocument();
  });
});
