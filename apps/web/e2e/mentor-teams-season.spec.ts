import { expect, test } from '@playwright/test';

test('Mentor Teams opens on the season last chosen there', async ({ page }) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'director'),
  );
  // Not the default (open) season, so the link can only have remembered it.
  await page.goto('/seasons/summer-2024-25/mentor-teams');
  await expect(
    page.getByRole('heading', { name: /Mentor Teams/ }),
  ).toBeVisible();
  await page.goto('/admin');
  const link = page.getByRole('link', { name: /Mentor Teams/ }).first();
  await expect(link).toHaveAttribute('href', /\/seasons\/summer-2024-25\//);
});
