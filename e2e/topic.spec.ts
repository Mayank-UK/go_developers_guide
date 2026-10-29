import { expect, test } from '@playwright/test';

/**
 * One topic page, rendered end-to-end: the client's generic TopicView turns
 * the served JSON (topic → sections → blocks) into the real DOM through the
 * Vite /api proxy. Three specs pin the anatomy facts every topic page
 * promises — sections render in order and close with the terminal
 * companion, code blocks start COLLAPSED and expand to the real served
 * source, and the inline live-demo panel mounts with its payload.
 */
test('a topic page renders its sections and terminal companion', async ({
  page,
}) => {
  await page.goto('/topics/go-language');

  // The served topic title is the page h1, scoped to the article.
  await expect(
    page
      .getByRole('article')
      .getByRole('heading', { level: 1, name: 'The Go language' }),
  ).toBeVisible();

  // The header mirrors the same served title: the route handle's generic
  // 'Topic' covered the load, and the screen's served title then took over
  // the banner h1.
  await expect(
    page
      .getByRole('banner')
      .getByRole('heading', { level: 1, name: 'The Go language' }),
  ).toBeVisible();

  // Sections render from the content API — the opening pitch, the
  // self-contained basics sections, and the terminal companion that closes
  // every topic.
  await expect(
    page.getByRole('heading', { level: 2, name: 'Why Go on the server' }),
  ).toBeVisible();
  await expect(
    page.getByRole('heading', {
      level: 2,
      name: 'Go basics: variables, types, control flow, loops, functions',
    }),
  ).toBeVisible();
  await expect(
    page.getByRole('heading', {
      level: 2,
      name: 'Data structures and strings',
    }),
  ).toBeVisible();
  await expect(
    page
      .getByRole('article')
      .getByRole('heading', { level: 2, name: 'Terminal companion: the curl tour' }),
  ).toBeVisible();
});

test('code blocks start collapsed and expand to the real served source', async ({
  page,
}) => {
  await page.goto('/topics/go-language');

  // Collapsed by default: the module manifest ("mod") toggle is
  // aria-expanded=false and its source is not in the DOM until clicked.
  const modToggle = page.getByRole('button', { name: 'mod', exact: true }).first();
  await expect(modToggle).toHaveAttribute('aria-expanded', 'false');
  const modFigure = page.locator('figure', { has: modToggle });
  await expect(modFigure.locator('pre')).toHaveCount(0);

  // Expanding reveals the real served code — the server module manifest.
  await modToggle.click();
  await expect(modToggle).toHaveAttribute('aria-expanded', 'true');
  await expect(modFigure.locator('pre')).toBeVisible();
  await expect(modFigure.locator('pre')).toContainText('module guide');
});

test('the inline live-demo panel mounts with its payload', async ({ page }) => {
  await page.goto('/topics/go-language');

  // The demo panel mounts (the demo body lazy-loads; the panel heading
  // comes from the served content, so it is the stable mount signal).
  await expect(
    page.getByRole('heading', { level: 3, name: 'Types at runtime — the proof' }),
  ).toBeVisible();

  // The lazy chunk loads and renders the demo's live payload — the reflect
  // walk computed server-side from this repo's real objects.
  await expect(page.getByTestId('rt-inspections')).toBeVisible();
  await expect(page.getByTestId('rt-inspections')).toContainText(
    'reflect.TypeOf — what the runtime knows',
  );
});
