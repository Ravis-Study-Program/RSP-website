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
  // The reminder's link opens that week's update straight away.
  await page.getByRole('link', { name: 'Write your update' }).click();
  let dialog = page.getByRole('dialog', { name: 'Week 2 Update' });
  await expect(dialog).toBeVisible();
  await dialog
    .getByRole('textbox', { name: 'Your Update', exact: true })
    .fill(
      'Completed trees, learned traversal patterns, no blockers. Next week: graphs.',
    );
  await dialog
    .getByRole('spinbutton', { name: 'Hours Spent (Expected: ~30h/week)' })
    .fill('30');
  await dialog
    .getByRole('button', { name: 'Post Update', exact: true })
    .click();
  await expect(dialog).toHaveCount(0);
  // Week 2 is done and the missed Week 1 is locked, so the header offers the
  // current week, which can be written as it goes.
  await expect(
    page.getByRole('button', { name: 'Write Week 3 Update' }),
  ).toBeVisible();
  await page.goto('/dashboard');
  await expect(
    page.getByRole('heading', { name: 'Welcome, Avery.' }),
  ).toBeVisible();
  await expect(page.getByRole('alert')).toHaveCount(0);
  // The student asks their mentor to unlock it, with a reason.
  await page.goto('/seasons/summer-2025-26/monday-updates');
  await page.getByRole('button', { name: 'More actions for Week 1' }).click();
  await expect(
    page.getByRole('menuitem', { name: 'Write Update' }),
  ).toHaveCount(0);
  await page.getByRole('menuitem', { name: 'Request Unlock' }).click();
  dialog = page.getByRole('dialog', { name: 'Request Unlock · Week 1' });
  await dialog.getByRole('textbox', { name: 'Reason' }).fill('I was sick.');
  await dialog.getByRole('button', { name: 'Send Request' }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByText('Unlock requested')).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath('late-week-requested.png'),
    fullPage: true,
  });
  // The week due now can still be edited.
  await page.getByRole('button', { name: 'More actions for Week 2' }).click();
  await page.getByRole('menuitem', { name: 'Edit Update' }).click();
  dialog = page.getByRole('dialog', { name: 'Edit Week 2 Update' });
  await dialog
    .getByRole('textbox', { name: 'Your Update', exact: true })
    .fill('Completed trees and graphs, now with detail.');
  await dialog
    .getByRole('button', { name: 'Save Update', exact: true })
    .click();
  await expect(dialog).toHaveCount(0);
  await page.getByRole('button', { name: 'More actions for Week 2' }).click();
  await page.getByRole('menuitem', { name: 'Show Details' }).click();
  await expect(
    page.getByText('Completed trees and graphs, now with detail.'),
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
  ).toEqual([['2025-12-08', '2025-12-14']]);
  await page.screenshot({
    path: testInfo.outputPath('monday-updates-table.png'),
    fullPage: true,
  });
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
    // Mentors can also see every week; coordinators read one week at a time.
    if (role === 'mentor') {
      await chooseOption(page, 'Program week', 'All updates');
      await expect(page).toHaveURL(/week=all/);
    } else
      await expect(page.getByText(/of \d+( marked)?$/).first()).toBeVisible();
  }
});

test('mentors, coordinators and directors choose the season in the filters', async ({
  page,
}) => {
  for (const role of ['mentor', 'coordinator']) {
    await page.addInitScript(
      (value) => localStorage.setItem('rsp-demo-role', value),
      role,
    );
    await page.goto('/seasons/summer-2025-26/monday-updates');
    // The toolbar keeps only the week; the season lives in Filters.
    await expect(
      page.getByRole('combobox', { name: 'Program week' }),
    ).toBeVisible();
    await expect(page.getByRole('combobox', { name: 'Season' })).toHaveCount(0);
    await page.getByRole('button', { name: 'Filters' }).click();
    await expect(page.getByRole('combobox', { name: 'Season' })).toContainText(
      'Summer 2025/26',
    );
    await page.keyboard.press('Escape');
  }

  // A Director can open any season from the same filter.
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'director'),
  );
  await page.goto('/seasons/summer-2025-26/monday-updates');
  await page.getByRole('button', { name: 'Filters' }).click();
  await chooseOption(page, 'Season', 'Summer 2024/25');
  await expect(page).toHaveURL(/\/seasons\/summer-2024-25\/monday-updates/);
});
