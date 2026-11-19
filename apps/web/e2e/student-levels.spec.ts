import { expect, test } from '@playwright/test';
import { expectSelected } from './select';

test("coordinators edit a student's details from People", async ({
  page,
}, testInfo) => {
  await page.addInitScript(() => {
    localStorage.setItem('rsp-demo-role', 'coordinator');
    localStorage.removeItem('rsp-demo-auth-state');
  });
  // The old All Students address now opens People, where students are managed.
  await page.goto('/seasons/summer-2025-26/mentees');
  await expect(page).toHaveURL(/\/seasons\/summer-2025-26\/people$/);
  await expect(page.getByRole('heading', { name: 'People' })).toBeVisible();
  await page
    .getByRole('button', { name: 'More actions for Amelia Chen' })
    .filter({ visible: true })
    .first()
    .click();
  await page.getByRole('menuitem', { name: 'Edit Details' }).click();
  const dialog = page.getByRole('dialog');
  await expectSelected(dialog, 'Student Level', 'Beginner');
  await dialog.getByRole('combobox', { name: 'Student Level' }).click();
  await expect(page.getByRole('option')).toHaveText([
    'Novice',
    'Beginner',
    'Intermediate',
    'Advanced',
  ]);
  await page.getByRole('option', { name: 'Advanced', exact: true }).click();
  // The student's own goals and weekly hours; blank follows the defaults.
  await expect(dialog.getByLabel('Easy', { exact: true })).toHaveAttribute(
    'placeholder',
    '10',
  );
  await dialog.getByLabel('Weekly hours').fill('20');
  await dialog.getByRole('button', { name: 'Save Details' }).click();
  await expect(dialog).toBeHidden();
  await expect(
    page
      .getByText('Advanced', { exact: true })
      .filter({ visible: true })
      .first(),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath('student-level.png'),
    fullPage: true,
  });
});

test('mentors see the practice goals read-only', async ({ page }, testInfo) => {
  await page.addInitScript(() => {
    localStorage.setItem('rsp-demo-role', 'mentor');
    localStorage.removeItem('rsp-demo-auth-state');
  });
  await page.goto('/settings');
  const goals = page.getByRole('region', { name: 'Practice Goals' });
  for (const [difficulty, minutes] of [
    ['Easy', '10'],
    ['Medium', '25'],
    ['Hard', '40'],
  ]) {
    const field = goals.getByLabel(`${difficulty} (minutes)`);
    await expect(field).toHaveValue(minutes);
    await expect(field).toBeDisabled();
    await expect(field).toHaveClass(/disabled:cursor-not-allowed/);
    await expect(field).toHaveClass(/disabled:select-none/);
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
  const save = page.getByRole('button', { name: 'Save Settings' });
  await expect(save).toBeDisabled();
  await easy.fill('15');
  await save.click();
  await expect(page.getByText('Settings saved.')).toBeVisible();
  await expect(easy).toHaveValue('15');
});
