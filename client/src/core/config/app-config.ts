/**
 * Application configuration — mirrors the react guide's `AppConfig` and the
 * express/spring guides' core/config module, folded into one file for the
 * skeleton.
 *
 * React/Vite selects `.env.*` files by `--mode` (dev → development, build →
 * production, vitest → test) and exposes them at build time via
 * `import.meta.env`. `VITE_*` prefixed vars are the only ones exposed to
 * client code.
 *
 * `Environment` is a const object + literal union (not an `enum`) to stay
 * compatible with `erasableSyntaxOnly` and modern TS conventions.
 */
export const Environment = {
  Dev: 'dev',
  Prod: 'prod',
} as const;

export type Environment = (typeof Environment)[keyof typeof Environment];

export interface AppConfig {
  environment: Environment;
  /**
   * Base URL every API request is resolved against. Defaults to '/api' so the
   * browser hits the Vite dev proxy, which forwards to the Go server.
   */
  apiBaseUrl: string;
  enableLogs: boolean;
}

export const isDev = (c: AppConfig): boolean => c.environment === Environment.Dev;
export const isProd = (c: AppConfig): boolean => c.environment === Environment.Prod;

function parseBoolean(value: string | undefined, fallback: boolean): boolean {
  if (value === undefined) return fallback;
  return value === 'true' || value === '1';
}

/**
 * Resolve the runtime environment from the Vite mode. Unknown modes throw
 * (rather than silently falling through to Dev, which would enable dev-only
 * behavior in an environment that didn't ask for it).
 */
function resolveEnvironment(mode: string): Environment {
  switch (mode) {
    case 'development':
    case 'test':
      return Environment.Dev;
    case 'production':
      return Environment.Prod;
    default:
      throw new Error(
        `Unknown Vite mode "${mode}"; expected development|test|production.`,
      );
  }
}

export function createAppConfig(env: ImportMetaEnv = import.meta.env): AppConfig {
  const environment = resolveEnvironment(env.MODE);

  return {
    environment,
    apiBaseUrl: env.VITE_API_BASE_URL ?? '/api',
    enableLogs: parseBoolean(env.VITE_ENABLE_LOGS, environment !== Environment.Prod),
  };
}
