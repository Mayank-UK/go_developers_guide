# Browser E2E (host-side Playwright)

Seven real-browser specs, small on purpose — each pins one wiring fact no
in-process test can see (the rendered page and the Vite → Go /api proxy hop):

- `home.spec.ts` — the topic map renders from the content API: all 10 groups,
  one populated link per served topic (pinned to the live response, floor =
  the complete 43-topic curriculum), zero coming-soon cards; the hero health
  chip reports the live API.
- `topic.spec.ts` — a topic page renders: sections in order, the terminal
  companion, code blocks that start collapsed and expand to the real served
  source, and the inline live-demo panel with its payload.
- `scenario_panel.spec.ts` — the header's ScenarioPanel flips dataMode and
  the change round-trips through PATCH /api/demo/scenario (server-confirmed,
  reload-persistent, restored to the mode it found).
- `demo.spec.ts` — the perf bench demo renders its measured payload
  (provenance, gzip curve, marshal A/B, escape verdicts) through the UI.
- `theme.spec.ts` — the theme toggle flips dark→light and the choice
  persists (data-theme, `gdg-theme` localStorage, reload).
- `not_found.spec.ts` — an unknown topic id renders the ErrorPanel with the
  server's error envelope (code, status, requestId); an unknown route
  renders the friendly 404 view.
- `console.spec.ts` — home + a topic page open with zero console errors.

## Why this runs on the host

The toolchain containers run unprivileged, so Chromium's system libraries
cannot be installed in them. The suite therefore runs on the HOST against the
externally managed dev stack — never a `webServer` block. The default base
URL is the client dev server (Vite inside `go_developers_guide_client_dev`,
published on :5175), which proxies /api to the Go server inside the dev
container (:8100, compose network).

## Run it

Prerequisites: host Node 22+, and the dev stack up (it runs continuously in
active dev; from the repo root if needed):

```bash
docker compose -f .docker/docker-compose.yml up -d
```

The server half must be listening inside the dev container (the stack's
smoke: `curl http://localhost:8100/api/health` from inside it; the server
process is started there, it is not a compose service).

```bash
cd e2e
npm install
npx playwright install chromium
npm test
```

`E2E_BASE_URL` overrides the default `http://localhost:5175` — point it at
any front door that serves the client and proxies /api to the server.
