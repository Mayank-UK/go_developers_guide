import { expect, test } from '@playwright/test';

/** The ten group titles the content API serves — the home map shows them all. */
const GROUP_TITLES = [
  'Foundations',
  'Framework',
  'Request handling',
  'APIs',
  'Data',
  'State & auth',
  'Media & realtime',
  'Platform',
  'Ship',
  'Capstones',
] as const;

/**
 * Curriculum-completeness floor: all 43 topics are authored (orders 1–43,
 * zero coming-soon cards). The rendered-map assertions below are pinned to
 * what the live API serves rather than a hardcoded count, so content growth
 * never breaks the spec — but a stack serving fewer than the complete
 * curriculum fails here loudly.
 */
const MIN_AUTHORED_TOPICS = 43;

/**
 * Home is the guide's front door: a hero with the live health chip and the
 * curriculum index (topic map) fetched live from GET /api/content. This spec
 * pins that the map actually renders in a real browser — every served group
 * and every authored topic as a populated link card, exactly as many as the
 * API serves (a dropped or doubled card fails), with no coming-soon cards.
 */
test('the home hub renders the full topic map', async ({ page, request }) => {
  const content = await (await request.get('/api/content')).json();
  const servedTopics: Array<{ id: string; sectionCount: number }> =
    content.topics;
  const authored = servedTopics.filter((topic) => topic.sectionCount > 0);
  expect(authored.length).toBeGreaterThanOrEqual(MIN_AUTHORED_TOPICS);

  await page.goto('/');

  // Scoped to the main region: the page-level h1 is the hero inside <main>.
  await expect(
    page
      .getByRole('main')
      .getByRole('heading', { level: 1, name: 'Go Developers Guide' }),
  ).toBeVisible();

  // The map arrives from the content API — wait for the real section to
  // replace the loading skeleton.
  const topicMap = page.locator('#topic-map');
  await expect(topicMap).toBeVisible();

  for (const title of GROUP_TITLES) {
    await expect(topicMap.getByRole('heading', { name: title, exact: true })).toBeVisible();
  }

  // Every authored topic is a populated link; the count mirrors the served
  // index exactly — a dropped or doubled topic fails here.
  await expect(topicMap.getByRole('link')).toHaveCount(authored.length);

  // Spot check: the first curriculum topic renders as its populated card.
  await expect(topicMap.getByTestId(`topic-card-${authored[0].id}`)).toBeVisible();

  // The whole curriculum is authored, so no disabled coming-soon cards may
  // render anywhere (the served-but-empty defensive branch stays dormant).
  await expect(
    topicMap.locator('[data-testid^="topic-card-"][aria-disabled="true"]'),
  ).toHaveCount(0);
});

/**
 * The hero's health chip is the one live-API signal on the shell itself: it
 * polls GET /api/health and reports the served status and environment, so a
 * dead server shows on the front door instead of a silently empty page.
 */
test('the hero health chip reports the live API', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByTestId('health-chip')).toHaveText(/^api: ok · \S+/);
});
