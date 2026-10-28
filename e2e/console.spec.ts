import { expect, test } from '@playwright/test';

/**
 * Zero console errors across the screens a visitor actually opens: home and
 * one full topic page (including its lazy demo chunk). Both render from the
 * live content API, so a failing fetch, a shape-validation rejection, or a
 * client crash surfaces as a console error (or page error) long before a
 * screenshot looks wrong.
 */
test('home and a topic page render with zero console errors', async ({ page }) => {
  const consoleErrors: string[] = [];
  page.on('console', (message) => {
    if (message.type() === 'error') {
      consoleErrors.push(message.text());
    }
  });
  page.on('pageerror', (error) => {
    consoleErrors.push(`pageerror: ${error.message}`);
  });

  await page.goto('/');
  await expect(
    page.getByRole('heading', { level: 1, name: 'Go Developers Guide' }),
  ).toBeVisible();
  await expect(page.locator('#topic-map')).toBeVisible();

  await page.goto('/topics/go-language');
  await expect(
    page.getByRole('article').getByRole('heading', { level: 1, name: 'The Go language' }),
  ).toBeVisible();
  await expect(
    page.getByRole('heading', { level: 2, name: 'Why Go on the server' }),
  ).toBeVisible();
  // The demo chunk mounts too — its lazy import is part of what must be
  // error-free.
  await expect(page.getByTestId('rt-inspections')).toBeVisible();

  expect(consoleErrors).toEqual([]);
});
