import { expect, test } from '@playwright/test';

/**
 * Unknown URLs degrade gracefully — two different paths, neither a crash:
 *
 * - a well-formed topic URL whose id the content API does not know fails at
 *   the data layer and renders the ErrorPanel with the server's error
 *   envelope surfaced verbatim (machine code, HTTP status, requestId — the
 *   trace key for the server logs);
 * - any other unknown route renders the friendly client 404 view.
 */
test('an unknown topic id degrades to the error panel with the envelope', async ({
  page,
}) => {
  await page.goto('/topics/definitely-not-a-topic');

  const panel = page.getByTestId('error-panel');
  await expect(panel).toBeVisible();
  await expect(panel).toContainText('Could not load this topic');

  // The envelope facts, rendered as machine diagnostics: the not_found code
  // and the truthful 404 status …
  await expect(panel).toContainText('not_found · status 404');
  // … and the server's requestId (32 hex chars) for log correlation.
  await expect(panel).toContainText(/request [0-9a-f]{32}/);
});

test('an unknown route renders the friendly 404 view', async ({ page }) => {
  await page.goto('/nowhere/at-all');

  await expect(
    page.getByRole('heading', { level: 1, name: 'Page not found' }),
  ).toBeVisible();
  const backHome = page.getByRole('link', { name: '← back to the guide' });
  await expect(backHome).toBeVisible();

  // The way out works, not just renders.
  await backHome.click();
  await expect(
    page.getByRole('heading', { level: 1, name: 'Go Developers Guide' }),
  ).toBeVisible();
});
