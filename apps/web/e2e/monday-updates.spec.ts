import { expect, test } from '@playwright/test';
import { chooseOption } from './select';

const monday = new Date('2025-12-15T00:00:00+10:30');

test.beforeEach(async ({ page }) => {
  await page.clock.setFixedTime(monday);
  await page.addInitScript(() => {
    localStorage.setItem('rsp-demo-role', 'student');
    localStorage.setItem('rsp-theme', 'dark');
  });
});

test('Monday reminder clears after the fixed weekly report is submitted; missed weeks remain available', async ({
  page,
}, testInfo) => {
  await page.goto('/dashboard');
  await expect(page.getByRole('alert')).toContainText(
    'Your Week 2 Monday update is due',
  );
  await page.screenshot({
    path: testInfo.outputPath('monday-reminder.png'),
    fullPage: true,
  });
  await page.getByRole('link', { name: 'Write your update' }).click();
  await expect(
    page.getByRole('combobox', { name: 'Program week' }),
  ).toContainText('Week 2');
  await expect(page.getByLabel('Dates', { exact: true })).toHaveCount(0);
  await page
    .getByRole('textbox', { name: 'Your update', exact: true })
    .fill(
      'Completed trees, learned traversal patterns, no blockers. Next week: graphs.',
    );
  await page
    .getByRole('spinbutton', { name: 'Hours spent (Expected: ~30h/week)' })
    .fill('30');
  await page.getByRole('button', { name: 'Post update', exact: true }).click();
  await expect(
    page.getByRole('status').filter({ hasText: 'Week 2 submitted.' }),
  ).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Post update', exact: true }),
  ).toHaveCount(0);
  await page.goto('/dashboard');
  await expect(
    page.getByRole('heading', { name: 'Welcome, Avery.' }),
  ).toBeVisible();
  await expect(page.getByRole('alert')).toHaveCount(0);
  await page.goto('/seasons/summer-2025-26/monday-updates');
  await chooseOption(page, 'Program week', /Week 1/);
  await expect(
    page.getByRole('heading', { name: 'Week 1 update', exact: true }),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath('late-week-form.png'),
    fullPage: true,
  });
  await page
    .getByRole('textbox', { name: 'Your update', exact: true })
    .fill('Submitting my missed first week.');
  await page.getByRole('button', { name: 'Post update', exact: true }).click();
  await expect(
    page.getByRole('status').filter({ hasText: 'Week 1 submitted.' }),
  ).toBeVisible();
  const submissions = await page.evaluate(
    () =>
      JSON.parse(localStorage.getItem('rsp-demo-monday-updates') ?? '{}').added,
  );
  expect(
    submissions.map((item: { startDate: string; endDate: string }) => [
      item.startDate,
      item.endDate,
    ]),
  ).toEqual([
    ['2025-12-01', '2025-12-07'],
    ['2025-12-08', '2025-12-14'],
  ]);
});

test('no reminder on Tuesday, Week 1 Monday or after the program', async ({
  page,
}) => {
  for (const date of [
    '2025-12-16T00:00:00+10:30',
    '2025-12-01T00:00:00+10:30',
    '2026-03-02T00:00:00+10:30',
  ]) {
    await page.clock.setFixedTime(new Date(date));
    await page.goto('/dashboard');
    await expect(
      page.getByRole('heading', { name: 'Welcome, Avery.' }),
    ).toBeVisible();
    await expect(page.getByRole('alert')).toHaveCount(0);
  }
});

test('staff review is organized by program week', async ({ page }) => {
  for (const role of ['mentor', 'coordinator']) {
    await page.addInitScript(
      (value) => localStorage.setItem('rsp-demo-role', value),
      role,
    );
    await page.goto('/seasons/summer-2025-26/monday-updates');
    await expect(
      page.getByRole('combobox', { name: 'Program week' }),
    ).toContainText('Week 2');
    await chooseOption(page, 'Program week', /Week 1/);
    await expect(page).toHaveURL(/week=week_season_2026_s2_1/);
    await chooseOption(page, 'Program week', 'All updates');
    await expect(page).toHaveURL(/week=all/);
    await expect(page.getByLabel('Dates', { exact: true })).toHaveCount(0);
  }
});
