import { expect, test } from '@playwright/test';

/**
 * The ScenarioPanel is the app's ONE global demo toggle (root header): the
 * data-mode select is backed by GET/PATCH /api/demo/scenario, with an
 * optimistic UI that a failed PATCH reverts. This spec drives the select
 * through the browser and proves the change round-trips — the server's
 * shared scenario endpoint confirms it, a reload re-reads it, and the
 * original mode is restored so the shared stack is left as found.
 */
test('the scenario panel round-trips dataMode through the server', async ({
  page,
  request,
}) => {
  // The mode to flip away from and back to — whatever the stack serves now,
  // so the spec is independent of the scenario's current state.
  const before = await (await request.get('/api/demo/scenario')).json();
  const other: string = before.dataMode === 'fake' ? 'live' : 'fake';

  await page.goto('/');
  const panel = page.getByTestId('scenario-panel');
  await expect(panel).toBeVisible();
  const dataMode = panel.getByLabel('Data mode');
  await expect(dataMode).toHaveValue(before.dataMode);

  // Flip through the UI. The select disables while the PATCH is in flight;
  // Playwright auto-waits for it to re-enable once the round completes.
  await dataMode.selectOption(other);
  await expect(dataMode).toBeEnabled();
  await expect(dataMode).toHaveValue(other);

  // Round-trip part one: the server confirms the patch on the shared store.
  const after = await (await request.get('/api/demo/scenario')).json();
  expect(after.dataMode).toBe(other);

  // Round-trip part two: the UI state persists — a fresh load re-reads the
  // scenario and renders the flipped mode.
  await page.reload();
  const reloaded = page.getByTestId('scenario-panel').getByLabel('Data mode');
  await expect(reloaded).toHaveValue(other);

  // Restore the original mode — never leave the shared stack mutated.
  await reloaded.selectOption(before.dataMode);
  await expect(reloaded).toBeEnabled();
  const restored = await (await request.get('/api/demo/scenario')).json();
  expect(restored.dataMode).toBe(before.dataMode);
});
