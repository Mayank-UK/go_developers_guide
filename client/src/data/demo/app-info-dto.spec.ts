import { describe, expect, it } from 'vitest';
import { appInfoDtoFromJson, toAppInfoDemo } from './app-info-dto';
import type { AppInfoDemoData } from '../../domain/demo/app-info';

/**
 * AppInfoDTO spec — hostile-input discipline: only the exact wire shape
 * parses, anything else throws (and therefore surfaces upstream as an
 * invalid_response Failure, never as a guessed default). The fixture
 * mirrors the REAL builder output observed from
 * GET /api/demo/gohttp/app-info (module `guide`, `(devel)`, zero
 * dependencies, toolchain-stamped build settings), and the empty-string
 * fields that ARE data — an unset build flag's value — survive validation.
 */

const wellFormed: AppInfoDemoData = {
  module: { path: 'guide', version: '(devel)', goVersion: 'go1.27.3', dependencyCount: 0 },
  runtime: { numCPU: 8, goMaxProcs: 8 },
  environment: { port: 8100, env: 'dev', logLevel: 'INFO' },
  buildSettings: [
    { key: '-buildmode', value: 'exe' },
    { key: 'CGO_CFLAGS', value: '' },
    { key: 'GOARCH', value: 'arm64' },
  ],
};

describe('appInfoDtoFromJson', () => {
  it('parses a well-formed payload round-trip', () => {
    expect(appInfoDtoFromJson(wellFormed)).toEqual(wellFormed);
  });

  it('accepts an empty buildSettings array (the server marshals [] never null)', () => {
    const parsed = appInfoDtoFromJson({ ...wellFormed, buildSettings: [] });

    expect(parsed.buildSettings).toEqual([]);
  });

  it('keeps empty strings that are data: an unset build flag value', () => {
    const parsed = appInfoDtoFromJson(wellFormed);

    expect(parsed.buildSettings[1]?.key).toBe('CGO_CFLAGS');
    expect(parsed.buildSettings[1]?.value).toBe('');
  });

  it.each([
    ['null', null],
    ['an array', [wellFormed]],
    ['a string', 'nope'],
    ['missing module', { ...wellFormed, module: undefined }],
    ['missing runtime', { ...wellFormed, runtime: undefined }],
    ['missing environment', { ...wellFormed, environment: undefined }],
    ['missing buildSettings', { ...wellFormed, buildSettings: undefined }],
    ['buildSettings not an array', { ...wellFormed, buildSettings: 'nope' }],
    ['a non-object module', { ...wellFormed, module: 'guide' }],
    ['missing dependencyCount', { ...wellFormed, module: { ...wellFormed.module, dependencyCount: undefined } }],
    ['NaN numCPU', { ...wellFormed, runtime: { ...wellFormed.runtime, numCPU: Number.NaN } }],
    ['missing goMaxProcs', { ...wellFormed, runtime: { numCPU: 8 } }],
    ['missing env name', { ...wellFormed, environment: { ...wellFormed.environment, env: undefined } }],
    ['empty env name', { ...wellFormed, environment: { ...wellFormed.environment, env: '' } }],
    ['empty module path', { ...wellFormed, module: { ...wellFormed.module, path: '' } }],
    ['a non-object setting row', { ...wellFormed, buildSettings: ['nope'] }],
    ['missing setting key', { ...wellFormed, buildSettings: [{ key: undefined, value: 'x' }] }],
    ['missing setting value (absent is not empty)', { ...wellFormed, buildSettings: [{ key: 'GOOS', value: undefined }] }],
  ])('throws for %s', (_label, body) => {
    expect(() => appInfoDtoFromJson(body)).toThrow();
  });

  it('drops unknown wire keys (whitelist, not pass-through)', () => {
    const parsed = appInfoDtoFromJson({
      ...wellFormed,
      extra: 'ignored',
      module: { ...wellFormed.module, sneaky: true },
    });

    expect(parsed).toEqual(wellFormed);
  });
});

describe('toAppInfoDemo', () => {
  it('maps the DTO onto the domain model (identity while 1:1)', () => {
    expect(toAppInfoDemo(wellFormed)).toBe(wellFormed);
  });
});
