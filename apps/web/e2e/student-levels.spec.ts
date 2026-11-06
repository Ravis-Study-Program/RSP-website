import { expect, test } from '@playwright/test';
import { expectSelected } from './select';

test('season mentors can set student levels and see the practice goals', async ({
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
    'Advanced',
  ]);
  await page.getByRole('option', { name: 'Advanced', exact: true }).click();
  await dialog.getByRole('button', { name: 'Save level' }).click();
  await expect(dialog).toBeHidden();
  await expect(
    page
      .getByText(
        testInfo.project.name.includes('mobile')
          ? 'Level: Advanced'
          : 'Advanced',
        { exact: true },
      )
      .first(),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath('student-level.png'),
    fullPage: true,
  });

  await page.goto('/settings');
  const goals = page.getByRole('region', { name: 'Practice Goals' });
  // Mentors see the program's goals read-only.
  for (const [difficulty, minutes] of [
    ['Easy', '10'],
    ['Medium', '25'],
    ['Hard', '40'],
  ]) {
    const field = goals.getByLabel(`${difficulty} (minutes)`);
    await expect(field).toHaveValue(minutes);
    await expect(field).toHaveAttribute('readonly');
  }
  await page.screenshot({
    path: testInfo.outputPath('practice-goals.png'),
    fullPage: true,
  });
});

test('coordinators change the practice goals for everyone from settings', async ({
  page,
}) => {
  await page.addInitScript(() => {
    localStorage.setItem('rsp-demo-role', 'coordinator');
    localStorage.removeItem('rsp-demo-auth-state');
  });
  await page.goto('/settings');
  const goals = page.getByRole('region', { name: 'Practice Goals' });
  const easy = goals.getByLabel('Easy (minutes)');
  await expect(easy).toHaveValue('10');
  await easy.fill('0');
  await expect(goals.getByText('Enter 1 to 180 minutes.')).toBeVisible();
  const save = page.getByRole('button', { name: 'Save settings' });
  await expect(save).toBeDisabled();
  await easy.fill('15');
  await save.click();
  await expect(page.getByText('Settings saved.')).toBeVisible();
  await expect(easy).toHaveValue('15');
});
