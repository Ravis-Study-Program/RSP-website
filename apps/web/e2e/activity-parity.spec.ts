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

test('system admin edits profile details and requests email verification separately', async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'system_admin'),
  );
  await page.goto('/admin/users');
  await page
    .getByRole('button', { name: 'Manage account' })
    .filter({ visible: true })
    .first()
    .click();
  const editor = page.getByRole('region', {
    name: 'Edit profile and contact details',
  });
  await editor.getByLabel('Name', { exact: true }).fill('Corrected member');
  await editor.getByLabel('Discord', { exact: true }).fill('member.discord');
  await editor.getByRole('button', { name: 'Save profile' }).click();
  await expect(editor.getByText('Profile updated.')).toBeVisible();
  const currentEmail = await editor
    .getByText('Current email:', { exact: false })
    .textContent();
  await editor.getByLabel('New email address').fill('changed@example.test');
  await editor.getByRole('button', { name: 'Send email verification' }).click();
  await expect(
    editor.getByText(
      'Verification sent to the new address. The current email remains until it is verified.',
    ),
  ).toBeVisible();
  await expect(editor.getByText('Current email:', { exact: false })).toHaveText(
    currentEmail!,
  );
});

test('administrators can correct the person attached to an enrollment', async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'director'),
  );
  await page.goto('/admin/enrollments');
  await page
    .getByRole('button', { name: 'Correct person or season' })
    .filter({ visible: true })
    .first()
    .click();
  const dialog = page.getByRole('dialog', { name: /Correct enrollment for/ });
  await expect(dialog.getByLabel('Correct person')).toBeEnabled();
  await dialog.getByLabel('Correct person').selectOption({ index: 1 });
  await dialog.getByRole('button', { name: 'Save correction' }).click();
  await expect(dialog).not.toBeVisible();
});
