import { expect, test } from '@playwright/test';

// The demo mentor has no assigned students, so this uses a season Coordinator,
// who may manage every student exactly as an assigned mentor manages theirs.
test('season staff set and clear a student’s time goals', async ({
  page,
}, testInfo) => {
  await page.addInitScript(() => {
    localStorage.setItem('rsp-demo-role', 'coordinator');
    localStorage.removeItem('rsp-demo-auth-state');
  });
  await page.goto('/seasons/summer-2025-26/mentees');
  await page.getByRole('button', { name: 'Set goals' }).first().click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByLabel('Easy (minutes)')).toHaveValue('10');
  await expect(dialog.getByLabel('Medium (minutes)')).toHaveValue('25');
  await expect(dialog.getByLabel('Hard (minutes)')).toHaveValue('40');
  await dialog.getByLabel('Hard (minutes)').fill('55');
  await page.screenshot({
    path: testInfo.outputPath('student-goals.png'),
    fullPage: true,
  });
  await dialog.getByRole('button', { name: 'Save goals' }).click();
  await expect(dialog).toBeHidden();

  await page.getByRole('button', { name: 'Custom goals' }).first().click();
  await expect(dialog.getByLabel('Hard (minutes)')).toHaveValue('55');
  await dialog.getByRole('button', { name: 'Use programme goals' }).click();
  await expect(dialog).toBeHidden();
  await expect(page.getByRole('button', { name: 'Custom goals' })).toHaveCount(
    0,
  );
});

test('Directors edit the programme time goals', async ({ page }, testInfo) => {
  await page.addInitScript(() => {
    localStorage.setItem('rsp-demo-role', 'director');
    localStorage.removeItem('rsp-demo-auth-state');
  });
  await page.goto('/settings');
  const goals = page.getByRole('region', { name: 'Practice goals' });
  for (const minutes of [10, 25, 40])
    await expect(
      goals.getByText(`${minutes} minutes`, { exact: true }),
    ).toBeVisible();
  await goals.getByRole('button', { name: 'Edit programme goals' }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByLabel('Easy (minutes)').fill('15');
  await dialog.getByRole('button', { name: 'Save goals' }).click();
  await expect(dialog).toBeHidden();
  await expect(
    goals.getByText(/Programme goals: 15 \/ 25 \/ 40/),
  ).toBeVisible();
  await expect(goals.getByText('15 minutes', { exact: true })).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath('programme-goals.png'),
    fullPage: true,
  });
});
