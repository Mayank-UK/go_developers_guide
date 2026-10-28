import { expect, test } from '@playwright/test';

/**
 * The theme toggle (root header) flips the design system between its dark
 * default and light mode. The choice must persist: the data-theme attribute
 * on <html>, the localStorage key 'gdg-theme', and the re-applied mode after
 * a reload — light must not flash back to dark after hydration. Each Playwright
 * test runs in a fresh browser context, so the stored preference never leaks
 * between specs.
 */
test('the theme toggle persists the choice across reloads', async ({ page }) => {
  await page.goto('/');

  // Dark is the design default; the toggle offers the other direction.
  const toggle = page.getByRole('button', { name: 'Switch to light theme' });
  await expect(toggle).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');

  // Flip to light: the button inverts, the attribute flips, and the choice
  // is written to the localStorage key.
  await toggle.click();
  await expect(
    page.getByRole('button', { name: 'Switch to dark theme' }),
  ).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
  expect(
    await page.evaluate(() => window.localStorage.getItem('gdg-theme')),
  ).toBe('light');

  // Reload: getInitialMode re-reads the stored preference before the first
  // render, so light persists — no dark flash, no reset.
  await page.reload();
  await expect(
    page.getByRole('button', { name: 'Switch to dark theme' }),
  ).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
});
