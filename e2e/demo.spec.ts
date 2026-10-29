import { expect, test } from '@playwright/test';

/**
 * One deterministic demo observed through the real UI: the performance
 * topic's bench demo (demo block id `perf:bench`) fetches
 * GET /api/demo/perf/bench and renders the boot-cached run of the repo's
 * REAL benchmarks — provenance (exact command), the measured gzip curve,
 * the marshal-vs-stream A/B, and the compiler's escape-analysis verdicts.
 * Every number comes from a real run, so the spec asserts the payload's
 * structure and provenance, never a measured value.
 */
test('the perf bench demo renders its measured payload', async ({ page }) => {
  // Image-stack caveat, probed up front: this demo's numbers come from a
  // REAL bounded `go test -bench` run executed beside the process at boot,
  // so it needs the Go toolchain AND the module's source tree. The
  // dev-stack server runs inside the golang devcontainer with both beside
  // it (the payload is always live there). A production-image front door
  // (E2E_BASE_URL → the nginx/CI stack) runs the DISTROLESS server — no
  // shell, no toolchain, no sources — and the endpoint then answers its
  // documented, server-suite-pinned typed 503 instead of rounding
  // anything into anecdotes. Skip exactly that case; any other failure
  // shape still fails the spec below.
  const probe = await page.request.get('/api/demo/perf/bench');
  if (probe.status() === 503) {
    const envelope = (await probe.json()) as { error?: { code?: string } };
    test.skip(
      envelope.error?.code === 'service_unavailable',
      'perf:bench needs the Go toolchain + source tree beside the server; the distroless image stack answers with its documented typed 503',
    );
  }

  await page.goto('/topics/performance');

  // The lazy demo chunk loads (the loading note gives way to the payload).
  const ready = page.getByTestId('perf-bench-ready');
  await expect(ready).toBeVisible();

  // Provenance: the run's exact command and configuration are rendered.
  const run = page.getByTestId('perf-bench-run');
  await expect(run).toBeVisible();
  await expect(run).toContainText('go test -bench');
  await expect(run).toContainText('benchtime');

  // The measured sections render: the gzip curve (levels × sizes) …
  const compression = page.getByTestId('perf-bench-compression');
  await expect(compression).toBeVisible();
  await expect(compression).toContainText('gzip');
  // … the marshal-vs-stream A/B with the hot path's allocation tax …
  await expect(page.getByTestId('perf-bench-encoding')).toBeVisible();
  // … and the compiler's escape-analysis verdicts verbatim.
  const escape = page.getByTestId('perf-bench-escape');
  await expect(escape).toBeVisible();
  await expect(escape).toContainText('escape analysis');
});
