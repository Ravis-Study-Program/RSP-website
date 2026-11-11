import { expect, test } from '@playwright/test';

async function openFirstWeek(
  page: import('@playwright/test').Page,
  role: string,
) {
  await page.addInitScript(
    (selected) => localStorage.setItem('rsp-demo-role', selected),
    role,
  );
  await page.goto('/seasons/summer-2025-26');
  await page.getByRole('link', { name: 'Week Resources' }).first().click();
  await expect(
    page.getByRole('heading', { name: 'Week 1 Resources', level: 1 }),
  ).toBeVisible();
}

test('students read a week resources page but cannot edit it', async ({
  page,
}) => {
  await openFirstWeek(page, 'student');
  await expect(
    page.getByText('Reading and practice for the week.'),
  ).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Edit Resources' }),
  ).toHaveCount(0);
  await page.getByRole('link', { name: 'Back to Season' }).click();
  await expect(page).toHaveURL(/\/seasons\/summer-2025-26$/);
});

test('coordinators edit a week resources page', async ({ page }) => {
  await openFirstWeek(page, 'coordinator');
  await page.getByRole('button', { name: 'Edit Resources' }).click();
  await page
    .getByRole('textbox', { name: 'Resources', exact: true })
    .fill('Start with the graph traversal notes.');
  await page.getByRole('button', { name: 'Save Resources' }).click();
  await expect(
    page.getByText('Start with the graph traversal notes.'),
  ).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Edit Resources' }),
  ).toBeVisible();
});
