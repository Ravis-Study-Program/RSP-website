import { chooseOption } from './select';
import { expect, test } from '@playwright/test';

test('former students can record mocks on past days and filter their personal history', async ({
  page,
}, testInfo) => {
  await page.clock.setFixedTime(new Date('2026-09-06T07:30:00Z'));
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'kicked'),
  );
  await page.goto('/mock-interviews');
  await expect(
    page.getByRole('heading', { name: 'Mock Interviews', exact: true }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'New interview' }).click();
  const create = page.getByRole('dialog', { name: 'New Mock Interview' });
  // Mocks are dated, not timed; the date defaults to today in Adelaide and
  // reads day, month, year.
  const when = create.getByRole('group', { name: 'Date' });
  await expect(when).toHaveText('06/09/2026');
  const setDate = async (digits: string) => {
    await when.getByRole('spinbutton', { name: /^day/ }).click();
    await page.keyboard.type(digits);
  };
  await expect(create.getByLabel('Type', { exact: true })).toHaveAttribute(
    'id',
    /^round-type-[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/,
  );
  await chooseOption(create, 'Interviewee', 'Amelia Chen · student');
  await setDate('07092026');
  await create.getByRole('button', { name: 'Create interview' }).click();
  await expect(
    create.getByText('Choose a date that has already happened.'),
  ).toBeVisible();
  // Interviews from earlier days can still be recorded.
  await setDate('05092026');
  await create.screenshot({
    path: testInfo.outputPath('adelaide-recording.png'),
  });
  await create.getByRole('button', { name: 'Create interview' }).click();
  await expect(create).toBeHidden();
  // All, the default view, already includes the interview just given.
  await expect(page.getByRole('tab', { name: 'All' })).toHaveAttribute(
    'aria-selected',
    'true',
  );
  await expect(
    page
      .getByRole('link', { name: 'Amelia Chen', exact: true })
      .filter({ visible: true })
      .first(),
  ).toBeVisible();
  await page
    .getByRole('button', {
      name: /More actions for the interview with Amelia Chen/,
    })
    .filter({ visible: true })
    .first()
    .click();
  await page.getByRole('menuitem', { name: 'Edit notes and scores' }).click();
  const edit = page.getByRole('dialog', { name: 'Edit Notes and Scores' });
  // The date is shown but cannot be changed.
  await expect(edit.getByLabel('Date', { exact: true })).toHaveAttribute(
    'readonly',
  );
  await expect(
    edit.locator('input[type="date"], input[type="time"]'),
  ).toHaveCount(0);
  await edit.getByLabel('Duration (minutes)').fill('65');
  await edit.getByRole('button', { name: 'Save changes' }).click();
  await expect(edit).toBeHidden();
  await page.getByRole('button', { name: /^Filters/ }).click();
  await chooseOption(page, /^Season$/, 'Summer 2024/25');
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL(/\/mock-interviews\?seasonId=season_2026_s1/);
  await expect(
    page.getByRole('button', { name: /More actions for the interview/ }),
  ).toHaveCount(0);
  await page.getByRole('button', { name: /^Filters/ }).click();
  await chooseOption(page, /^Season$/, 'All time');
  await page.keyboard.press('Escape');
  await expect(page).not.toHaveURL(/seasonId=/);
  await expect(
    page
      .getByText(/65 min/)
      .filter({ visible: true })
      .first(),
  ).toBeVisible();
  await page.goto('/leetcode');
  await expect(
    page.getByRole('heading', { name: 'LeetCode Practice', exact: true }),
  ).toBeVisible();
  await page.getByRole('button', { name: /^Filters/ }).click();
  await chooseOption(page, /^Season$/, 'Summer 2024/25');
  await page.keyboard.press('Escape');
  await expect(
    page.getByText('Binary Tree Level Order Traversal', { exact: true }),
  ).toHaveCount(0);
  await page.getByRole('button', { name: /^Filters/ }).click();
  await chooseOption(page, /^Season$/, 'All time');
  await page.keyboard.press('Escape');
  await expect(
    page
      .getByText('Binary Tree Level Order Traversal', { exact: true })
      .filter({ visible: true }),
  ).toBeVisible();
  await page.goto('/graduates');
  await expect(page).toHaveURL(/\/forbidden$/);
});
