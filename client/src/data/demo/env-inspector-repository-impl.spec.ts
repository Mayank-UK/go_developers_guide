import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAppConfig } from '../../core/config/app-config';
import { stubApiFetch, stubNetworkFailure } from '../../test/fetch_stub';
import { EnvInspectorApi } from './env-inspector-api';
import { EnvInspectorRepositoryImpl } from './env-inspector-repository-impl';

afterEach(() => {
  vi.unstubAllGlobals();
});

/**
 * EnvInspectorRepositoryImpl spec — fixture-driven (happy + failure):
 * the inspector payload (resolved-config variables, scorecard, live
 * flags) round-trips; malformed payloads and transport failures
 * resolve as Failed Results, never throws.
 */

const demoPayload = {
  mode: 'runtime-derived: every row below was resolved by core.LoadConfig',
  env: 'dev',
  variables: [
    {
      field: 'Port',
      envVar: 'GDG_PORT',
      value: '8100',
      source: 'default',
      sensitive: false,
      validatedBy: 'parsed by strconv.Atoi at boot; must be 1–65535',
    },
    {
      field: 'DatabaseURL',
      envVar: 'GDG_DATABASE_URL',
      value: 'postgres://postgres@postgres:5432/guide',
      source: 'environment',
      sensitive: true,
      validatedBy: 'the pool dials at boot and the boot fails fast',
    },
  ],
  twelveFactor: [
    { factor: 'I. Codebase', verdict: 'hits', how: 'one git tree serves every environment' },
    { factor: 'X. Dev/prod parity', verdict: 'differs', how: 'devcontainer vs distroless' },
  ],
  flags: { dataMode: 'fake', injectFailure: 'none' },
  redaction: 'a DSN’s password and its entire query are never echoed',
  reasoning: 'names are the contract, defaults make dev zero-config',
};

function makeRepository(): EnvInspectorRepositoryImpl {
  return new EnvInspectorRepositoryImpl(new EnvInspectorApi(createAppConfig()));
}

describe('EnvInspectorRepositoryImpl', () => {
  it('returns a Success carrying the validated domain payload', async () => {
    stubApiFetch([{ url: '/api/demo/env/inspector', body: demoPayload }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.value.env).toBe('dev');
      expect(result.value.variables).toHaveLength(2);
      expect(result.value.variables[1]?.value).toBe('postgres://postgres@postgres:5432/guide');
      expect(result.value.twelveFactor[1]?.verdict).toBe('differs');
      expect(result.value.flags.dataMode).toBe('fake');
    }
  });

  it('guards a malformed payload into a Failed Result', async () => {
    stubApiFetch([{ url: '/api/demo/env/inspector', body: { mode: 'half a payload' } }]);

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('invalid_response');
    }
  });

  it('guards a network failure into a Failed Result (network_error, status 0)', async () => {
    stubNetworkFailure();

    const result = await makeRepository().get();

    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.failure.code).toBe('network_error');
    }
  });
});
