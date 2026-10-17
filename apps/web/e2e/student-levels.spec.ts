import { expect, test } from '@playwright/test';
import { expectSelected } from './select';

test('season mentors can set student levels and see fixed practice goals', async ({
  page,
}, testInfo) => {
  await page.addInitScript(() => {
    localStorage.setItem('rsp-demo-role', 'mentor');
    localStorage.removeItem('rsp-demo-auth-state');
  });
  await page.goto('/seasons/summer-2025-26/people');
  await expect(page.getByRole('heading', { name: 'People' })).toBeVisible();
  await page.getByRole('button', { name: 'Set level' }).first().click();
  const dialog = page.getByRole('dialog');
  await expectSelected(dialog, 'Student level', 'Beginner');
  await dialog.getByRole('combobox', { name: 'Student level' }).click();
  await expect(page.getByRole('option')).toHaveText([
    'Novice',
    'Beginner',
    'Intermediate',
    'Advance',
  ]);
  await page.getByRole('option', { name: 'Advance', exact: true }).click();
  await dialog.getByRole('button', { name: 'Save level' }).click();
  await expect(dialog).toBeHidden();
  await expect(
    page
      .getByText(
        testInfo.project.name.includes('mobile') ? 'Level: Advance' : 'Advance',
        { exact: true },
      )
      .first(),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath('student-level.png'),
    fullPage: true,
  });

  await page.goto('/settings');
  const goals = page.getByRole('region', { name: 'Practice goals' });
  for (const minutes of [20, 35, 50])
    await expect(
      goals.getByText(`${minutes} minutes`, { exact: true }),
    ).toBeVisible();
  await expect(goals.getByRole('spinbutton')).toHaveCount(0);
  await expect(goals.getByRole('switch')).toHaveCount(0);
  await page.screenshot({
    path: testInfo.outputPath('fixed-goals.png'),
    fullPage: true,
  });
});
