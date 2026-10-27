import { describe, expect, it } from 'vitest';
import { envInspectorDtoFromJson } from './env-inspector-dto';

/**
 * EnvInspectorDTO spec — the fromJson whitelist: a full payload builds
 * the domain shape (the resolved-config variables with their source
 * attribution and sensitivity flags, the twelve-factor scorecard, the
 * live flag panel); every violation (missing panels, wrong types, a
 * variable without its validatedBy note) throws so parseDto can
 * upgrade it to an InvalidResponseException.
 */

const fullPayload = {
  mode: 'runtime-derived: every row below was resolved by core.LoadConfig',
  env: 'dev',
  variables: [
    {
      field: 'Port',
      envVar: 'GDG_PORT',
      value: '8100',
      source: 'default',
      sensitive: false,
      validatedBy: 'parsed by strconv.Atoi at boot; must be 1–65535 or the boot fails',
    },
    {
      field: 'DatabaseURL',
      envVar: 'GDG_DATABASE_URL',
      value: 'postgres://postgres@postgres:5432/guide',
      source: 'environment',
      sensitive: true,
      validatedBy: 'DSN shape is the driver’s to validate: the pool dials at boot',
    },
  ],
  twelveFactor: [
    {
      factor: 'I. Codebase',
      verdict: 'hits',
      how: 'one git tree serves every environment; the compose files ARE the environments',
    },
    {
      factor: 'VIII. Concurrency',
      verdict: 'differs',
      how: 'the guide hosts the jobs worker IN the API process',
    },
  ],
  flags: { dataMode: 'fake', injectFailure: 'none' },
  redaction: 'sensitive rows are redacted by policy: a DSN’s password is never echoed',
  reasoning: 'names are the contract, defaults make dev zero-config',
};

describe('envInspectorDtoFromJson', () => {
  it('builds the domain payload from a valid wire body', () => {
    const dto = envInspectorDtoFromJson(fullPayload);

    expect(dto.env).toBe('dev');
    expect(dto.variables).toHaveLength(2);
    expect(dto.variables[1]?.envVar).toBe('GDG_DATABASE_URL');
    expect(dto.variables[1]?.sensitive).toBe(true);
    expect(dto.variables[1]?.source).toBe('environment');
    expect(dto.variables[1]?.value).not.toMatch(/:\/\/[^/@]+:[^/@]+@/);
    expect(dto.twelveFactor[1]?.verdict).toBe('differs');
    expect(dto.flags.dataMode).toBe('fake');
    expect(dto.flags.injectFailure).toBe('none');
  });

  it('throws when the body is not an object', () => {
    expect(() => envInspectorDtoFromJson(null)).toThrow();
    expect(() => envInspectorDtoFromJson('env inspector')).toThrow();
  });

  it('throws when a top-level panel is missing', () => {
    const { variables, ...withoutVariables } = fullPayload;
    expect(() => envInspectorDtoFromJson(withoutVariables)).toThrow(/variables/i);

    const { twelveFactor, ...withoutScorecard } = fullPayload;
    expect(() => envInspectorDtoFromJson(withoutScorecard)).toThrow(/twelveFactor/i);

    const { flags, ...withoutFlags } = fullPayload;
    expect(() => envInspectorDtoFromJson(withoutFlags)).toThrow(/flags/i);
  });

  it('throws when a variable row carries the wrong types', () => {
    expect(() =>
      envInspectorDtoFromJson({
        ...fullPayload,
        variables: [{ ...fullPayload.variables[0], sensitive: 'yes' }],
      }),
    ).toThrow();
    expect(() =>
      envInspectorDtoFromJson({
        ...fullPayload,
        variables: [{ ...fullPayload.variables[0], source: 42 }],
      }),
    ).toThrow();
  });

  it('throws when the flag panel is not an object of strings', () => {
    expect(() =>
      envInspectorDtoFromJson({ ...fullPayload, flags: { dataMode: 'fake' } }),
    ).toThrow(/injectFailure/);
  });
});
