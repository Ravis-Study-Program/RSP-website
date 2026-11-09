import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }, testInfo) => {
  test.skip(testInfo.project.name.includes('mobile'), 'Desktop table rows');
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'director'),
  );
});

test('clicking a season row opens the season', async ({ page }) => {
  await page.goto('/seasons');
  await page
    .getByRole('row')
    .filter({ hasText: 'Summer 2025/26' })
    .getByRole('cell')
    .nth(1)
    .click();
  await expect(page).toHaveURL(/\/seasons\/summer-2025-26$/);
});

test('clicking a person row opens their profile', async ({ page }) => {
  await page.goto('/people');
  await page
    .getByRole('row')
    .filter({ hasText: 'Amelia Chen' })
    .getByRole('cell')
    .nth(1)
    .click();
  await expect(page).toHaveURL(/\/people\/amelia-chen/);
});

test('clicking a LeetCode row with notes shows and hides them', async ({
  page,
}) => {
  await page.goto('/leetcode');
  const label = await page
    .getByRole('button', { name: /^Show notes for / })
    .first()
    .getAttribute('aria-label');
  const problem = label!.replace('Show notes for ', '');
  const row = page.getByRole('row').filter({ hasText: problem }).first();
  await row.getByRole('cell').nth(2).click();
  const notes = page.getByRole('region', { name: `Notes for ${problem}` });
  await expect(notes).toBeVisible();
  await row.getByRole('cell').nth(2).click();
  await expect(notes).toHaveCount(0);
});

test('a row menu does not also open the row', async ({ page }) => {
  await page.goto('/seasons');
  await page
    .getByRole('button', { name: 'More actions for Summer 2025/26' })
    .click();
  await expect(
    page.getByRole('menuitem', { name: 'Edit season' }),
  ).toBeVisible();
  await expect(page).toHaveURL(/\/seasons$/);
});
