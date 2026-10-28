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
