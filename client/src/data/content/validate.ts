/**
 * Shared payload validators for the content DTO `fromJson` functions: they
 * throw on shape violations (rather than coercing) so bad server data surfaces
 * loudly as an InvalidResponseException via the guards' parseDto — hostile
 * input never reaches the domain as NaN orders or "[object Object]" titles.
 *
 * The DTO constructors copy only the fields listed here, so unknown/extra wire
 * keys are dropped (whitelist, not pass-through).
 */

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

export function requiredString(source: Record<string, unknown>, key: string): string {
  const value = source[key];
  if (typeof value !== 'string' || value === '') {
    throw new Error(`expected string at "${key}"`);
  }
  return value;
}

/**
 * A string field whose empty value is DATA, not absence — e.g. an unset
 * filter or an OK round's empty status detail. Present-but-empty passes;
 * missing or non-string still throws.
 */
export function requiredText(source: Record<string, unknown>, key: string): string {
  const value = source[key];
  if (typeof value !== 'string') {
    throw new Error(`expected string at "${key}"`);
  }
  return value;
}

export function requiredNumber(source: Record<string, unknown>, key: string): number {
  const value = source[key];
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    throw new Error(`expected number at "${key}"`);
  }
  return value;
}

export function optionalNumber(source: Record<string, unknown>, key: string): number | undefined {
  const value = source[key];
  if (value === undefined) return undefined;
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    throw new Error(`expected number at "${key}"`);
  }
  return value;
}

export function optionalString(source: Record<string, unknown>, key: string): string | undefined {
  const value = source[key];
  if (value === undefined) return undefined;
  if (typeof value !== 'string') throw new Error(`expected string at "${key}"`);
  return value;
}

export function requiredArray(source: Record<string, unknown>, key: string): unknown[] {
  const value = source[key];
  if (!Array.isArray(value)) throw new Error(`expected array at "${key}"`);
  return value;
}

export function requiredRecord(source: Record<string, unknown>, key: string): Record<string, unknown> {
  const value = source[key];
  if (!isRecord(value)) throw new Error(`expected object at "${key}"`);
  return value;
}

export function optionalRecord(
  source: Record<string, unknown>,
  key: string,
): Record<string, unknown> | undefined {
  const value = source[key];
  if (value === undefined) return undefined;
  if (!isRecord(value)) throw new Error(`expected object at "${key}"`);
  return value;
}

export function optionalArray(source: Record<string, unknown>, key: string): unknown[] | undefined {
  const value = source[key];
  if (value === undefined) return undefined;
  if (!Array.isArray(value)) throw new Error(`expected array at "${key}"`);
  return value;
}

export function requiredBoolean(source: Record<string, unknown>, key: string): boolean {
  const value = source[key];
  if (typeof value !== 'boolean') {
    throw new Error(`expected boolean at "${key}"`);
  }
  return value;
}
