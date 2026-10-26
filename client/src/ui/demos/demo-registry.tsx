import type { ComponentType, LazyExoticComponent } from 'react';

/**
 * Demo registry — the client-side half of the content API's demo-block
 * contract. Served content names a demo by the demo block's id; this record is
 * the only place that binds an id to an implementation. Demos are lazy-loaded
 * so a topic's chunk only pays for the demos it actually renders, and an
 * unknown id degrades to a "not wired yet" panel instead of crashing the
 * screen (see DemoHost).
 *
 * The registry is EMPTY until the first demos land with their topics — every
 * demo block id served before that renders the friendly not-wired-yet card.
 *
 * ── HOW A DEMO LANDS ────────────────────────────────────────────────────────
 * 1. Server side: a pure payload builder in `internal/domain/demos/` + a thin
 *    handler in `internal/presentation/demo/<area>.go` (area module), so the
 *    demo endpoint is `GET /api/demo/<area>/<name>` with a typed response.
 * 2. Client data path: `src/domain/demo/<name>-repository.ts` (port) +
 *    `src/data/demo/<name>-{dto,api,repository-impl}.ts` + a use case in
 *    `src/domain/demo/`, wired in the container under its own tokens.
 * 3. Create `src/ui/demos/<area>/<name>_demo.tsx` — a DEFAULT-exported
 *    component taking no props (it resolves its own dependencies through the
 *    container, like every view) — with loading + error states and a
 *    fixture-driven spec (happy + failure).
 * 4. Add one entry here:
 *
 *      const MyDemo = lazy(() => import('./<area>/<name>_demo'));
 *      export const demoRegistry = { ..., '<area>:<name>': MyDemo };
 *
 * Nothing else changes: the content's demo-block id starts resolving through
 * DemoHost automatically. Demo ids follow the server's `<area>:<name>`
 * convention (e.g. `go:app-info`, `runtime:goroutines`).
 */

export type DemoComponentType = ComponentType;

export type DemoRegistry = Readonly<
  Record<string, LazyExoticComponent<DemoComponentType>>
>;

export const demoRegistry: DemoRegistry = {};
