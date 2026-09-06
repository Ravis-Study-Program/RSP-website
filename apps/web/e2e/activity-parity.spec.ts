import { expect, test } from '@playwright/test';

test('practice charts follow filters and the picker accepts keyboard search', async ({
  page,
}, testInfo) => {
  await page.goto('/practice');
  await expect(
    page.getByRole('heading', { name: 'Problem practice' }),
  ).toBeVisible();
  await expect(
    page.getByRole('heading', { name: 'Difficulty breakdown' }),
  ).toBeVisible();
  await expect(
    page.getByRole('heading', { name: 'Time by difficulty' }),
  ).toBeVisible();
  await page.getByLabel('Show practice goals (20 / 35 / 50 minutes)').uncheck();
  await page
    .getByLabel('Search problem attempts')
    .fill('no matching question xyz');
  await expect(
    page.getByText('No attempts match these filters.'),
  ).toBeVisible();
  await page.getByLabel('Search problem attempts').clear();
  await page.getByRole('button', { name: 'Log attempt' }).first().click();
  await page.getByRole('combobox', { name: 'Problem' }).fill('Two Sum');
  await page.getByRole('combobox', { name: 'Problem' }).press('ArrowDown');
  await page.getByRole('combobox', { name: 'Problem' }).press('Enter');
  await expect(page.getByRole('combobox', { name: 'Problem' })).toHaveValue(
    /Two Sum/,
  );
  await page.screenshot({
    path: testInfo.outputPath('practice-question-picker.png'),
    fullPage: true,
  });
});

test('suggesting a question leaves practice counts unchanged', async ({
  page,
}, testInfo) => {
  await page.goto('/practice');
  const count = page.getByText(/\d+ recorded$/);
  await expect(count).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Suggest a question' }),
  ).toBeEnabled();
  const before = await count.textContent();
  await page.getByRole('button', { name: 'Suggest a question' }).click();
  await expect(
    page.getByRole('button', { name: 'Suggest a question' }),
  ).toBeEnabled();
  await expect(count).toHaveText(before!);
  await page.screenshot({
    path: testInfo.outputPath('practice-charts.png'),
    fullPage: true,
  });
});

test('administrators create, resend and cancel season invitations', async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'director'),
  );
  await page.goto('/admin/enrollments');
  await page.getByRole('button', { name: 'Invite a member' }).click();
  const dialog = page.getByRole('dialog', { name: 'Invite a member' });
  await dialog.getByLabel('Name', { exact: true }).fill('Invited member');
  await dialog
    .getByLabel('Email', { exact: true })
    .fill('invited@example.test');
  await dialog.getByRole('button', { name: 'Send invitation' }).click();
  await expect(
    page.getByText(
      'Invitation email queued. The recipient has 7 days to accept.',
    ),
  ).toBeVisible();
  const invitations = page.getByRole('region', {
    name: 'Invitations',
    exact: true,
  });
  await invitations
    .getByRole('button', { name: 'Resend', exact: true })
    .filter({ visible: true })
    .click();
  await invitations
    .getByRole('button', { name: 'Cancel invitation', exact: true })
    .filter({ visible: true })
    .click();
  await expect(page.getByText('Invitation cancelled.')).toBeVisible();
});
