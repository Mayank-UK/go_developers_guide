import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { RuntimeTypesDemo } from './runtime_types_demo';
import { demoRegistry } from '../../demos/demo-registry';
import { container, ContainerTokens } from '../../../core/container';
import { createAppConfig } from '../../../core/config/app-config';
import { ServerFailure } from '../../../core/error-handling/failures';
import { failed, success, type Result } from '../../../core/result';
import type { GetRuntimeTypesDemoUseCase } from '../../../domain/demo/get-runtime-types-demo-usecase';
import type { RuntimeTypesDemoData } from '../../../domain/demo/runtime-types';

/**
 * RuntimeTypesDemo spec — fixture-driven (happy + failure): the component
 * resolves its use case through the container, so the spec swaps the token
 * registration (the container's documented replace-a-singleton seam) instead
 * of stubbing fetch. Behavior, not implementation: the six payload faces
 * render, the compiler's verbatim verdict is displayed as the proof, empty
 * data fields (a predeclared type's pkgPath, a missed assertion's block id)
 * render honestly, and failures degrade to the shared error panel with
 * recovery.
 */

const fixture: RuntimeTypesDemoData = {
  inspections: [
    {
      expression: 'content.Topic{}',
      type: 'guide/content.Topic',
      kind: 'struct',
      pkgPath: 'guide/content',
      sizeBytes: 96,
    },
    {
      expression: 'content.NewTextBlock',
      type: 'func(string, string, string, string) (content.TextBlock, error)',
      kind: 'func',
      pkgPath: 'guide/content',
      sizeBytes: 8,
    },
    { expression: '43', type: 'int', kind: 'int', pkgPath: '', sizeBytes: 8 },
  ],
  fields: [
    { name: 'ID', type: 'string', jsonTag: 'id' },
    { name: 'Title', type: 'string', jsonTag: 'title' },
    { name: 'Lead', type: 'string', jsonTag: 'lead,omitempty' },
    { name: 'Blocks', type: '[]content.Block', jsonTag: 'blocks' },
  ],
  assertions: [
    { expression: 'blocks[0].(type)', dynamicType: 'content.TextBlock', ok: true, blockID: 'demo:text' },
    { expression: 'blocks[3].(type)', dynamicType: 'content.ListBlock', ok: true, blockID: 'demo:list' },
    { expression: 'blocks[0].(content.CodeBlock)', dynamicType: 'content.CodeBlock', ok: false, blockID: '' },
  ],
  zeroValues: [
    { declaration: 'var id string', type: 'string', display: '""', isZero: true },
    { declaration: 'var present bool', type: 'bool', display: 'false', isZero: true },
    { declaration: 'var sections []content.Section', type: '[]content.Section', display: 'nil or 0', isZero: true },
  ],
  enforcement: {
    function: 'repeat',
    signature: 'func repeat(text string, times int) string',
    call: 'repeat(3.0, 2)',
    compilerVerdict:
      'cannot use 3.0 (untyped float constant 3) as string value in argument to repeat',
    runtimeOutcome:
      'none — the program containing the call never builds; the typed call repeat("go", 3) returns "gogogo"',
  },
  boundary: {
    decodeTarget: 'content.TextBlock',
    badInput: '{"kind":"text","id":42,"text":"Contraband"}',
    outcome: 'rejected',
    errorType: '*json.UnmarshalTypeError',
    message: 'json: cannot unmarshal number into Go struct field TextBlock.id of type string',
  },
  // The tables' teaching sentences ride the payload — the client renders
  // them, the server authors them.
  fieldsNote:
    "The tag column is read at runtime by reflect — it is where this API's camelCase wire names live, and how encoding/json finds them.",
  assertionsNote:
    'The comma-ok row reports false instead of panicking — the dynamic type is data the runtime carries, not a guess it commits to.',
};

function stubUseCase(execute: () => Promise<Result<RuntimeTypesDemoData>>): void {
  container.initialize(createAppConfig());
  container.register(
    ContainerTokens.GetRuntimeTypesDemoUseCase,
    { execute } as unknown as GetRuntimeTypesDemoUseCase,
  );
}

describe('RuntimeTypesDemo', () => {
  it('is bound in the demo registry under its demo-block id', () => {
    expect(demoRegistry['go:runtime-types']).toBeDefined();
  });

  it('renders loading first, then all six payload faces', async () => {
    stubUseCase(async () => success(fixture));

    render(<RuntimeTypesDemo />);

    expect(screen.getByTestId('runtime-types-loading')).toBeInTheDocument();

    await waitFor(() =>
      expect(screen.getByTestId('rt-inspections')).toBeInTheDocument(),
    );

    // Inspections: real guide values, kinds included; a predeclared type's
    // empty pkgPath renders as a dash, not as a blank guess.
    expect(screen.getByTestId('rt-inspections')).toHaveTextContent('guide/content.Topic');
    expect(screen.getByTestId('rt-inspections')).toHaveTextContent('func');
    expect(screen.getByTestId('rt-inspections')).toHaveTextContent('96');

    // Fields: the walked struct with its runtime-readable json tags, plus
    // the server-taught sentence that reads the table (rendered from the
    // payload, never hardcoded here).
    expect(screen.getByTestId('rt-fields')).toHaveTextContent('lead,omitempty');
    expect(screen.getByTestId('rt-fields')).toHaveTextContent('[]content.Block');
    expect(screen.getByTestId('rt-fields')).toHaveTextContent(
      'read at runtime by reflect',
    );
    expect(screen.getByTestId('rt-fields')).toHaveTextContent(
      "how encoding/json finds them",
    );

    // Assertions: matches resolve ids; the comma-ok miss reports false with
    // no block id, and never crashes the render. Its teaching sentence is
    // the payload's own (fieldsNote's sibling), served by the server.
    expect(screen.getByTestId('rt-assertions')).toHaveTextContent('matched');
    expect(screen.getByTestId('rt-assertions')).toHaveTextContent('ok=false');
    expect(screen.getByTestId('rt-assertions')).toHaveTextContent(
      'reports false instead of panicking',
    );

    // Zero values: every declaration already at its zero.
    expect(screen.getByTestId('rt-zero-values')).toHaveTextContent('var present bool');
    expect(screen.getByTestId('rt-zero-values')).toHaveTextContent('IsZero() = true');

    // The enforcement proof is the compiler's own verdict, verbatim.
    expect(screen.getByTestId('rt-verdict')).toHaveTextContent('build failed');
    expect(screen.getByTestId('rt-compiler-verdict')).toHaveTextContent('cannot use 3.0');

    // The boundary check names the error type that did reject.
    expect(screen.getByTestId('rt-boundary')).toHaveTextContent('*json.UnmarshalTypeError');
    expect(screen.getByTestId('rt-boundary')).toHaveTextContent('rejected');
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

    render(<RuntimeTypesDemo />);

    await waitFor(() => expect(screen.getByTestId('error-panel')).toBeInTheDocument());
    expect(screen.getByTestId('error-panel')).toHaveTextContent('service_unavailable');

    // Retry re-runs the use case and the payload renders.
    screen.getByRole('button', { name: 'Try again' }).click();
    await waitFor(() => expect(screen.getByTestId('rt-inspections')).toBeInTheDocument());
    expect(screen.queryByTestId('error-panel')).not.toBeInTheDocument();
  });
});
