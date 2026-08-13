import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';

test('desktop workspace navigation and canonical routes', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name.includes('mobile'), 'Desktop-only assertion');
  await page.goto('/dashboard');
  await expect(page.getByRole('heading', { name: /Good evening/i })).toBeVisible();
  await expect(page.getByRole('navigation', { name: 'Primary navigation' })).toBeVisible();
  await page.getByRole('link', { name: 'People' }).first().click();
  await expect(page.getByRole('heading', { name: 'People' })).toBeVisible();
});

test('old practice route redirects without losing the app shell', async ({ page }) => {
  await page.goto('/leetcode');
  await expect(page).toHaveURL(/\/practice$/);
  await expect(page.getByRole('heading', { name: 'Problem practice' })).toBeVisible();
  await expect(page.locator('#workspace-select:visible, #workspace-select-mobile:visible')).toBeVisible();
});

test('mobile tables use readable cards', async ({ page }, testInfo) => {
  test.skip(!testInfo.project.name.includes('mobile'), 'Mobile-only assertion');
  await page.goto('/practice');
  await expect(page.getByRole('heading', { name: 'Problem practice' })).toBeVisible();
  await expect(page.locator('article').filter({ hasText: 'Binary Tree Level Order Traversal' })).toBeVisible();
  await expect(page.getByRole('navigation', { name: 'Mobile navigation' })).toBeVisible();
});

test('dashboard has no detectable WCAG A or AA violations', async ({ page }) => {
  await page.goto('/dashboard');
  await expect(page.getByRole('heading', { name: /Good evening/i })).toBeVisible();
  const results = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa']).analyze();
  expect(results.violations).toEqual([]);
});

test('keyboard users can skip navigation and open quick navigation', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name.includes('mobile'), 'Desktop keyboard assertion');
  await page.goto('/dashboard');
  await expect(page.getByRole('heading', { name: /Good evening/i })).toBeVisible();
  await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
  await page.keyboard.press('Tab');
  await expect(page.getByRole('link', { name: 'Skip to main content' })).toBeFocused();
  await page.keyboard.press('Enter');
  await expect(page.locator('#main-content')).toBeFocused();
  await page.keyboard.press('Meta+k');
  await expect(page.getByRole('dialog', { name: 'Quick navigation' })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog', { name: 'Quick navigation' })).toBeHidden();
});
