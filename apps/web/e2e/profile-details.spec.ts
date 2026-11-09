import { expect, test } from '@playwright/test';

async function openAmeliaLeetCode(
  page: import('@playwright/test').Page,
  role: string,
) {
  await page.addInitScript(
    (selected) => localStorage.setItem('rsp-demo-role', selected),
    role,
  );
  await page.goto('/people/amelia-chen');
  await page.getByRole('tab', { name: 'LeetCode' }).click();
  await expect(
    page.getByRole('region', { name: 'Amelia Chen practice history' }),
  ).toBeVisible();
}

test('directors can open a student’s LeetCode notes on their profile', async ({
  page,
  isMobile,
}) => {
  test.skip(isMobile, 'Notes open from table rows on wide screens.');
  await openAmeliaLeetCode(page, 'director');
  await expect(
    page.getByRole('button', { name: /^Show notes for / }).first(),
  ).toBeVisible();
});

test('other students cannot open someone else’s LeetCode notes', async ({
  page,
}) => {
  await openAmeliaLeetCode(page, 'student');
  await expect(
    page.getByRole('button', { name: /^Show notes for / }),
  ).toHaveCount(0);
});
