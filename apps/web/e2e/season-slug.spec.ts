import { expect, test } from '@playwright/test';

test('changing a season’s slug keeps the page at the new address', async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'director'),
  );
  await page.goto('/seasons/summer-2025-26');
  await page.getByRole('button', { name: 'Edit Season' }).click();
  await page.getByRole('menuitem', { name: 'Edit Details' }).click();
  const dialog = page.getByRole('dialog', { name: /Edit Summer 2025\/26/ });
  await dialog.getByLabel('URL Slug').fill('summer-renamed');
  await dialog.getByRole('button', { name: 'Save Season' }).click();
  await expect(page).toHaveURL(/\/seasons\/summer-renamed$/);
  await expect(
    page.getByRole('heading', { name: 'Summer 2025/26' }),
  ).toBeVisible();
});
