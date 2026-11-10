import { chooseOption } from './select';
import { expect, test } from '@playwright/test';

test('practice charts follow filters and the picker accepts keyboard search', async ({
  page,
}, testInfo) => {
  await page.goto('/leetcode');
  await expect(
    page.getByRole('heading', { name: 'LeetCode Practice' }),
  ).toBeVisible();
  await expect(
    page.getByRole('heading', { name: 'Difficulty Breakdown' }),
  ).toBeVisible();
  await expect(
    page.getByRole('heading', { name: 'Time by Difficulty' }),
  ).toBeVisible();
  await page
    .getByRole('checkbox', {
      name: 'Show practice goals (10 / 25 / 40 minutes)',
    })
    .uncheck();
  await page
    .getByLabel('Search problem attempts')
    .fill('no matching question xyz');
  await expect(
    page.getByText('No attempts match these filters.'),
  ).toBeVisible();
  await page.getByLabel('Search problem attempts').clear();
  await page.getByRole('button', { name: 'Log Attempt' }).first().click();
  await page.getByRole('combobox', { name: 'Problem' }).fill('Two Sum');
  // Results load from the API as the user types.
  await expect(page.getByRole('option', { name: /Two Sum/ })).toBeVisible();
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

test('profiles show all-time activity in the same tables as the dedicated pages', async ({
  page,
}, testInfo) => {
  // A fellow student: staff can open someone else's notes and details.
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'student'),
  );
  await page.goto('/people/amelia-chen');
  // Profiles are all time only, with seasons as badges instead of a picker.
  await expect(page.getByLabel('View activity')).toHaveCount(0);
  await expect(page.getByLabel(/^Season 25\/26/)).toBeVisible();
  await expect(
    page.getByRole('heading', { name: 'Program participation' }),
  ).toHaveCount(0);
  // The LeetCode table is the LeetCode page's, read-only, and someone
  // else's notes stay private.
  const practice = page.getByRole('region', {
    name: 'Amelia Chen practice history',
  });
  await expect(
    practice
      .getByText('Binary Tree Level Order Traversal')
      .filter({ visible: true }),
  ).toBeVisible();
  await expect(practice.getByRole('button', { name: /notes for/ })).toHaveCount(
    0,
  );
  await expect(
    practice.getByRole('button', { name: /More actions for/ }),
  ).toHaveCount(0);
  // The table tabs sit in the toolbar, beside the search.
  await practice.getByRole('tab', { name: 'Mock Interviews' }).click();
  const mocks = page.getByRole('region', { name: 'Amelia Chen mock history' });
  await expect(
    mocks.getByRole('tablist', { name: 'Mock interview view' }),
  ).toBeVisible();
  // You gave Amelia's interview, so you can open its details.
  await expect(
    mocks
      .getByRole('button', { name: 'Show details for Amelia Chen' })
      .or(mocks.getByRole('button', { name: 'View Details' })),
  ).toBeVisible();
  await mocks.getByRole('button', { name: /^Filters/ }).click();
  await expect(page.getByRole('combobox', { name: 'Type' })).toBeVisible();
  await expect(
    page.getByRole('combobox', { name: 'Result', exact: true }),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath('profile-history.png'),
    fullPage: true,
  });
});

test('profiles hide details of interviews you were not part of', async ({
  page,
}) => {
  // Zara's only interview was between her and Liam; the viewer is a fellow
  // student, since staff can open the details.
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'student'),
  );
  await page.goto('/people/zara-ahmed');
  await page.getByRole('tab', { name: 'Mock Interviews' }).click();
  const mocks = page.getByRole('region', { name: 'Zara Ahmed mock history' });
  await expect(
    mocks.getByText('Liam Foster').filter({ visible: true }).first(),
  ).toBeVisible();
  await expect(mocks.getByRole('button', { name: /details/i })).toHaveCount(0);
});

test('the sidebar account card opens your profile', async ({
  page,
  isMobile,
}) => {
  test.skip(isMobile, 'The sidebar is shown on wide screens only.');
  await page.goto('/dashboard');
  await page.getByRole('link', { name: /^Your profile/ }).click();
  await expect(page).toHaveURL(/\/profile$/);
});

test('suggesting a question leaves practice counts unchanged', async ({
  page,
}, testInfo) => {
  await page.goto('/leetcode');
  // One per attempt: a row menu on desktop, an Edit button on mobile cards.
  const attempts = page
    .getByRole('button', { name: /More actions for/ })
    .or(page.getByRole('button', { name: 'Edit', exact: true }));
  // Capture the count only after the attempt history has loaded.
  await expect(attempts.first()).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Suggest a Question' }),
  ).toBeEnabled();
  const before = await attempts.count();
  await page.getByRole('button', { name: 'Suggest a Question' }).click();
  await expect(
    page.getByRole('button', { name: 'Suggest a Question' }),
  ).toBeEnabled();
  await expect(attempts).toHaveCount(before);
  await page.screenshot({
    path: testInfo.outputPath('practice-charts.png'),
    fullPage: true,
  });
});

test('administrators invite a student with their mentor, then resend and cancel', async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'director'),
  );
  await page.goto('/admin/enrollments');
  await page.getByRole('button', { name: 'Invite a Member' }).click();
  const dialog = page.getByRole('dialog', { name: 'Invite a Member' });
  await dialog.getByLabel('Name', { exact: true }).fill('Invited member');
  await dialog
    .getByLabel('Email', { exact: true })
    .fill('invited@example.test');
  // The form starts on the page's season.
  await expect(dialog.getByRole('combobox', { name: 'Season' })).toHaveText(
    /Summer 2025\/26/,
  );
  // A student can be invited together with their mentor.
  await chooseOption(dialog, 'Mentor', 'Noah Williams');
  await dialog.getByRole('button', { name: 'Send Invitation' }).click();
  await expect(
    page.getByText(
      /^Invitation email queued for .+\. The recipient has 7 days to accept\.$/,
    ),
  ).toBeVisible();
  // The invitation is a row of the enrollments table, with its mentor.
  const invited = page
    .locator('tr:visible, article:visible')
    .filter({ hasText: 'Invited member' });
  await expect(invited).toContainText('Noah Williams');
  await expect(invited).toContainText('Invited');
  await expect(invited).toContainText('invited@example.test');
  const invitationMenu = invited.getByRole('button', {
    name: 'More actions for Invited member',
  });
  await invitationMenu.click();
  await page.getByRole('menuitem', { name: 'Resend', exact: true }).click();
  await invitationMenu.click();
  await page
    .getByRole('menuitem', { name: 'Cancel Invitation', exact: true })
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
    .getByRole('button', { name: /^More actions for / })
    .filter({ visible: true })
    .first()
    .click();
  await page.getByRole('menuitem', { name: 'Manage Account' }).click();
  const editor = page.getByRole('region', {
    name: 'Edit profile and contact details',
  });
  await editor.getByLabel('Name', { exact: true }).fill('Corrected member');
  // There is no Discord field: a person's Discord link is their own.
  await expect(editor.getByLabel('Discord')).toHaveCount(0);
  await editor.getByLabel('Email Address').fill('changed@example.test');
  await page
    .getByRole('dialog')
    .getByRole('button', { name: 'Confirm' })
    .click();
  await expect(
    page
      .getByRole('dialog')
      .getByText(
        'Verification sent to the new address. The current email stays until it is verified.',
      ),
  ).toBeVisible();
});

test('administrators edit an enrollment from Manage Account', async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'director'),
  );
  await page.goto('/admin/enrollments');
  await page
    .getByRole('button', { name: /^More actions for / })
    .filter({ visible: true })
    .first()
    .click();
  await page.getByRole('menuitem', { name: 'Manage Account' }).click();
  const dialog = page.getByRole('dialog', { name: /^Manage / });
  await expect(dialog.getByLabel('Edit Season', { exact: true })).toBeEnabled();
  await expect(dialog.getByLabel('Edit Role', { exact: true })).toBeEnabled();
  // The person an enrollment belongs to is not editable here.
  await expect(dialog.getByLabel('Correct Person')).toHaveCount(0);
  // Nothing to confirm until something changes.
  await expect(dialog.getByRole('button', { name: 'Confirm' })).toBeDisabled();
  await dialog.getByRole('button', { name: 'Cancel' }).click();
  await expect(dialog).not.toBeVisible();
});

test('removing a mentor takes them off the season’s enrollments', async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'director'),
  );
  await page.goto('/admin/enrollments');
  const row = page
    .locator('tr:visible, article:visible')
    .filter({ hasText: 'Noah Williams' });
  await row
    .getByRole('button', { name: 'More actions for Noah Williams' })
    .click();
  await page.getByRole('menuitem', { name: 'Remove', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Remove Noah Williams?' });
  await dialog
    .getByLabel('Reason', { exact: true })
    .fill('Finished volunteering');
  await dialog
    .getByLabel('Type Noah Williams to confirm')
    .fill('Noah Williams');
  await dialog.getByRole('button', { name: 'Remove Member' }).click();
  // Enrollments lists current members; their history stays on People. His
  // name can still appear as a student's mentor, so check his own row.
  await expect(
    page.getByRole('button', { name: 'More actions for Noah Williams' }),
  ).toHaveCount(0);
});

test('administrators delete empty seasons and retain populated seasons', async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'director'),
  );
  await page.goto('/admin/seasons');
  const seasonMenu = /^More actions for /;
  const populated = page
    .locator('tr:visible, article:visible')
    .filter({ has: page.getByRole('button', { name: seasonMenu }) })
    .first();
  await populated.getByRole('button', { name: seasonMenu }).click();
  await page.getByRole('menuitem', { name: 'Delete', exact: true }).click();
  let confirmation = page.getByRole('alertdialog');
  const label = await confirmation.locator('label strong').textContent();
  await confirmation.getByRole('textbox').fill(label!);
  await confirmation
    .getByRole('button', { name: 'Delete Empty Season' })
    .click();
  await expect(confirmation.getByRole('alert')).toContainText(
    'cannot be deleted',
  );
  await confirmation.getByRole('button', { name: 'Cancel' }).click();

  await page.getByRole('button', { name: 'Create Season' }).click();
  const create = page.getByRole('dialog', { name: 'Create Season' });
  await create.getByLabel('Name').fill('Empty test season');
  await create.getByLabel('URL Slug').fill('empty-test-season');
  await create.getByLabel('Location').fill('Test campus');
  await create.getByRole('button', { name: 'Create Season' }).click();
  const row = page
    .locator('tr:visible, article:visible')
    .filter({ hasText: 'Empty test season' });
  await row.getByRole('button', { name: seasonMenu }).click();
  await page.getByRole('menuitem', { name: 'Delete', exact: true }).click();
  confirmation = page.getByRole('alertdialog');
  await confirmation.getByRole('textbox').fill('Empty test season');
  await confirmation
    .getByRole('button', { name: 'Delete Empty Season' })
    .click();
  await expect(row).toHaveCount(0);
});
