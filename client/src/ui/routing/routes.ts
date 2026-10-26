/**
 * RoutePaths — the canonical route registry; the only place raw paths are
 * written (const object, not an enum — erasable syntax).
 */
export const RoutePaths = {
  /** Home: the content index (groups + topic links). */
  Home: '/',
  /** The generic topic renderer, driven by the :topicId param. */
  Topic: '/topics/:topicId',
  /** Catch-all for unknown routes. */
  NotFound: '*',
} as const;

export type RoutePath = (typeof RoutePaths)[keyof typeof RoutePaths];

/** Builds a concrete topic URL from the canonical parametrized path. */
export function topicPath(topicId: string): string {
  return RoutePaths.Topic.replace(':topicId', encodeURIComponent(topicId));
}
