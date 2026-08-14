import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';

async function selectDemoRole(page: Page, role: string) {
  await page.addInitScript((selectedRole) => {
    localStorage.setItem('rsp-demo-role', selectedRole);
    localStorage.removeItem('rsp-demo-auth-state');
  }, role);
}

async function useSignedOutDemo(page: Page) {
  await page.addInitScript(() => {
    localStorage.removeItem('rsp-demo-role');
    localStorage.setItem('rsp-demo-auth-state', 'signed-out');
  });
}

async function expectReflowWithoutUnreachableAction(
  page: Page,
  actionName: string,
) {
  const action = page.getByRole('main').getByRole('link', { name: actionName });
  await expect(action).toBeVisible();
  const documentMetrics = await page.evaluate(() => ({
    clientWidth: document.documentElement.clientWidth,
    scrollWidth: document.documentElement.scrollWidth,
  }));
  expect(documentMetrics.scrollWidth).toBeLessThanOrEqual(
    documentMetrics.clientWidth + 1,
  );
  const actionBounds = await action.evaluate((element) => {
    const rect = element.getBoundingClientRect();
    return {
      left: rect.left,
      right: rect.right,
      viewportWidth: window.innerWidth,
    };
  });
  expect(actionBounds.left).toBeGreaterThanOrEqual(-1);
  expect(actionBounds.right).toBeLessThanOrEqual(
    actionBounds.viewportWidth + 1,
  );
}

test('@compat protected shell and settings form work across supported engines', async ({
  page,
}, testInfo) => {
  await selectDemoRole(page, 'student');
  await page.goto('/dashboard');
  await expect(page.getByRole('heading', { name: /Welcome/i })).toBeVisible();
  const navigationName = testInfo.project.name.includes('mobile')
    ? 'Mobile navigation'
    : 'Primary navigation';
  await expect(
    page.getByRole('navigation', { name: navigationName }),
  ).toBeVisible();
  await page.goto('/settings');
  await expect(page.getByRole('heading', { name: 'Settings' })).toBeVisible();
  await expect(page.getByLabel('Timezone')).toHaveValue('Australia/Adelaide');
});

test('desktop workspace navigation and canonical routes', async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name.includes('mobile'), 'Desktop-only assertion');
  await selectDemoRole(page, 'student');
  await page.goto('/dashboard');
  await expect(page.getByRole('heading', { name: /Welcome/i })).toBeVisible();
  await expect(
    page.getByRole('navigation', { name: 'Primary navigation' }),
  ).toBeVisible();
  await page.getByRole('link', { name: 'People' }).first().click();
  await expect(page.getByRole('heading', { name: 'People' })).toBeVisible();
});

test('old practice route redirects without losing the app shell', async ({
  page,
}) => {
  await page.goto('/leetcode');
  await expect(page).toHaveURL(/\/practice$/);
  await expect(
    page.getByRole('heading', { name: 'Problem practice' }),
  ).toBeVisible();
  await expect(
    page.locator('#workspace-select:visible, #workspace-select-mobile:visible'),
  ).toBeVisible();
});

test('mobile tables use readable cards', async ({ page }, testInfo) => {
  test.skip(!testInfo.project.name.includes('mobile'), 'Mobile-only assertion');
  await page.goto('/practice');
  await expect(
    page.getByRole('heading', { name: 'Problem practice' }),
  ).toBeVisible();
  await expect(
    page
      .locator('article')
      .filter({ hasText: 'Binary Tree Level Order Traversal' }),
  ).toBeVisible();
  await expect(
    page.getByRole('navigation', { name: 'Mobile navigation' }),
  ).toBeVisible();
});

test('dashboard has no detectable WCAG A or AA violations', async ({
  page,
}) => {
  await page.goto('/dashboard');
  await expect(page.getByRole('heading', { name: /Welcome/i })).toBeVisible();
  const results = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
    .analyze();
  expect(results.violations).toEqual([]);
});

// These automated proxies complement rather than replace manual screen-reader checks.
test('320px and 200%-equivalent reflow keep primary actions reachable and reduced motion suppresses transitions', async ({
  browser,
}, testInfo) => {
  test.skip(
    testInfo.project.name !== 'desktop-chromium',
    'Dedicated Chromium accessibility proxies',
  );

  for (const width of [320, 640]) {
    const context = await browser.newContext({
      viewport: { width, height: 800 },
    });
    const page = await context.newPage();
    await selectDemoRole(page, 'student');
    await page.goto('/dashboard');
    await expect(page.getByRole('heading', { name: /Welcome/i })).toBeVisible();
    await expectReflowWithoutUnreachableAction(page, 'Continue practice');
    await context.close();
  }

  const reducedMotionContext = await browser.newContext({
    viewport: { width: 1280, height: 800 },
    reducedMotion: 'reduce',
  });
  const reducedMotionPage = await reducedMotionContext.newPage();
  await selectDemoRole(reducedMotionPage, 'student');
  await reducedMotionPage.goto('/dashboard');
  const primaryAction = reducedMotionPage
    .getByRole('main')
    .getByRole('link', { name: 'Continue practice' });
  await expect(primaryAction).toBeVisible();
  const motion = await primaryAction.evaluate((element) => ({
    mediaMatches: window.matchMedia('(prefers-reduced-motion: reduce)').matches,
    scrollBehavior: getComputedStyle(document.documentElement).scrollBehavior,
    transitionSeconds: Math.max(
      ...getComputedStyle(element)
        .transitionDuration.split(',')
        .map((duration) => Number.parseFloat(duration) || 0),
    ),
  }));
  expect(motion.mediaMatches).toBe(true);
  expect(motion.scrollBehavior).toBe('auto');
  expect(motion.transitionSeconds).toBeLessThanOrEqual(0.001);
  await reducedMotionContext.close();
});

test('keyboard users can skip navigation and open quick navigation', async ({
  page,
}, testInfo) => {
  test.skip(
    testInfo.project.name.includes('mobile'),
    'Desktop keyboard assertion',
  );
  await page.goto('/dashboard');
  await expect(page.getByRole('heading', { name: /Welcome/i })).toBeVisible();
  await page.evaluate(() =>
    (document.activeElement as HTMLElement | null)?.blur(),
  );
  await page.keyboard.press('Tab');
  await expect(
    page.getByRole('link', { name: 'Skip to main content' }),
  ).toBeFocused();
  await page.keyboard.press('Enter');
  await expect(page.locator('#main-content')).toBeFocused();
  await page.keyboard.press('Meta+k');
  await expect(
    page.getByRole('dialog', { name: 'Quick navigation' }),
  ).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(
    page.getByRole('dialog', { name: 'Quick navigation' }),
  ).toBeHidden();
});

test('role-aware navigation covers every major programme role', async ({
  browser,
}, testInfo) => {
  test.skip(testInfo.project.name.includes('mobile'), 'Desktop role matrix');
  const cases = [
    ['student', 'Season'],
    ['mentor', 'My mentees'],
    ['coordinator', 'Mentor teams'],
    ['graduate', 'Directory'],
    ['director', 'Administration'],
    ['system_admin', 'Administration'],
  ] as const;
  for (const [role, navigationLabel] of cases) {
    const context = await browser.newContext();
    const page = await context.newPage();
    await selectDemoRole(page, role);
    await page.goto('/dashboard');
    await expect(page.getByRole('heading', { name: /Welcome/i })).toBeVisible();
    await expect(
      page
        .getByRole('navigation', { name: 'Primary navigation' })
        .getByRole('link', { name: navigationLabel }),
    ).toBeVisible();
    await context.close();
  }
});

test('completed non-student members keep global practice without directory or admin access', async ({
  page,
}, testInfo) => {
  test.skip(
    testInfo.project.name.includes('mobile'),
    'Desktop former-member route gate',
  );
  await selectDemoRole(page, 'former_member');
  await page.goto('/dashboard');
  const navigation = page.getByRole('navigation', {
    name: 'Primary navigation',
  });
  await expect(
    navigation.getByRole('link', { name: 'Practice' }),
  ).toBeVisible();
  await expect(
    navigation.getByRole('link', { name: 'Mock interviews' }),
  ).toBeVisible();
  await expect(navigation.getByRole('link', { name: 'Directory' })).toHaveCount(
    0,
  );
  await page.goto('/graduates');
  await expect(page).toHaveURL(/\/forbidden$/);
  await page.goto('/admin/enrollments');
  await expect(page).toHaveURL(/\/forbidden$/);
});

test('signed-out, unverified, unavailable, nonmember, forbidden and not-found states are distinct', async ({
  browser,
  page,
}, testInfo) => {
  test.skip(
    testInfo.project.name.includes('mobile'),
    'Desktop route-state matrix',
  );
  const signedOutContext = await browser.newContext();
  const signedOutPage = await signedOutContext.newPage();
  await useSignedOutDemo(signedOutPage);
  await signedOutPage.goto('/dashboard');
  await expect(signedOutPage).toHaveURL(/\/sign-in$/);
  await expect(
    signedOutPage.getByRole('heading', { name: 'Welcome to RSP' }),
  ).toBeVisible();
  await signedOutContext.close();

  const unverifiedContext = await browser.newContext();
  const unverifiedPage = await unverifiedContext.newPage();
  await selectDemoRole(unverifiedPage, 'unverified');
  await unverifiedPage.goto('/dashboard');
  await expect(unverifiedPage).toHaveURL(/\/verify-email$/);
  await expect(
    unverifiedPage.getByRole('heading', { name: 'Check your email' }),
  ).toBeVisible();
  await unverifiedContext.close();

  const nonmemberContext = await browser.newContext();
  const nonmemberPage = await nonmemberContext.newPage();
  await selectDemoRole(nonmemberPage, 'nonmember');
  await nonmemberPage.goto('/practice');
  await expect(nonmemberPage).toHaveURL(/\/no-season$/);
  await expect(
    nonmemberPage.getByRole('heading', { name: 'No season access yet' }),
  ).toBeVisible();
  await nonmemberContext.close();

  for (const [role, message] of [
    ['suspended', 'This account is suspended.'],
    ['deletion_pending', '30-day recovery period'],
  ] as const) {
    const unavailableContext = await browser.newContext();
    const unavailablePage = await unavailableContext.newPage();
    await selectDemoRole(unavailablePage, role);
    await unavailablePage.goto('/dashboard');
    await expect(unavailablePage).toHaveURL(
      new RegExp(`/account-unavailable\\?state=${role}$`),
    );
    await expect(
      unavailablePage.getByRole('heading', { name: 'Account unavailable' }),
    ).toBeVisible();
    await expect(
      unavailablePage.getByText(message, { exact: false }),
    ).toBeVisible();
    await unavailableContext.close();
  }

  await selectDemoRole(page, 'student');
  await page.goto('/forbidden');
  await expect(
    page.getByRole('heading', { name: 'You do not have access' }),
  ).toBeVisible();
  await page.goto('/definitely-missing');
  await expect(
    page.getByRole('heading', { name: 'Page not found' }),
  ).toBeVisible();
});

test('coordinator can reach season operations and create, edit and delete a week', async ({
  page,
}, testInfo) => {
  test.skip(
    testInfo.project.name.includes('mobile'),
    'Desktop operational CRUD coverage',
  );
  await selectDemoRole(page, 'coordinator');
  await page.goto('/admin/weeks');
  await expect(page.getByRole('heading', { name: 'Weeks' })).toBeVisible();

  await page.getByRole('button', { name: 'Create week' }).click();
  const create = page.getByRole('dialog', { name: 'Create week' });
  await create.getByLabel('Week number').fill('15');
  await create
    .getByLabel('Resource URL (optional)')
    .fill('https://example.test/resources/week-15');
  page.once('dialog', async (confirmation) => confirmation.dismiss());
  await page.keyboard.press('Escape');
  await expect(create).toBeVisible();
  await create.getByRole('button', { name: 'Create week' }).click();
  await expect(
    page.getByRole('row').filter({ hasText: 'Week 15' }),
  ).toBeVisible();

  await page
    .getByRole('row')
    .filter({ hasText: 'Week 15' })
    .getByRole('button', { name: 'Edit' })
    .click();
  const edit = page.getByRole('dialog', { name: 'Edit week 15' });
  await edit.getByLabel('Week number').fill('16');
  await edit.getByRole('button', { name: 'Save week' }).click();
  await expect(
    page.getByRole('row').filter({ hasText: 'Week 16' }),
  ).toBeVisible();

  await page
    .getByRole('row')
    .filter({ hasText: 'Week 16' })
    .getByRole('button', { name: 'Delete week' })
    .click();
  const confirmation = page.getByRole('alertdialog', {
    name: 'Delete week Week 16?',
  });
  await confirmation.getByLabel(/Type Week 16/).fill('Week 16');
  await confirmation.getByRole('button', { name: 'Delete week' }).click();
  await expect(
    page.getByRole('row').filter({ hasText: 'Week 16' }),
  ).toHaveCount(0);
});

test('coordinator can enroll a verified nonmember without granting coordinator', async ({
  page,
}, testInfo) => {
  test.skip(
    testInfo.project.name.includes('mobile'),
    'Desktop enrollment operation',
  );
  await selectDemoRole(page, 'coordinator');
  await page.goto('/admin/enrollments');
  await page.getByRole('button', { name: 'Add member' }).click();
  const dialog = page.getByRole('dialog', { name: 'Add season member' });
  await expect(dialog.getByLabel('Account')).toContainText('Priya Singh');
  await expect(
    dialog.getByLabel('Season role').locator('option[value="coordinator"]'),
  ).toHaveCount(0);
  await dialog.getByLabel('Account').selectOption('person_candidate');
  page.once('dialog', async (confirmation) => confirmation.dismiss());
  await page.keyboard.press('Escape');
  await expect(dialog).toBeVisible();
  await dialog.getByRole('button', { name: 'Add member' }).click();
  await expect(
    page.getByRole('row').filter({ hasText: 'Priya Singh' }),
  ).toBeVisible();
});

test('coordinator edits only season resources and mentorship forms protect dirty input', async ({
  page,
}, testInfo) => {
  test.skip(
    testInfo.project.name.includes('mobile'),
    'Desktop scoped-season operations',
  );
  await selectDemoRole(page, 'coordinator');
  await page.goto('/seasons/2026-semester-2');
  await expect(page.getByRole('button', { name: 'Edit season' })).toHaveCount(
    0,
  );
  await page.getByRole('button', { name: 'Edit resources' }).click();
  const resources = page.getByRole('dialog', {
    name: /Edit resources for Semester 2, 2026/,
  });
  await resources
    .getByLabel('Season resources URL')
    .fill('https://rsp.org.au/resources/semester-2');
  page.once('dialog', async (confirmation) => confirmation.dismiss());
  await page.keyboard.press('Escape');
  await expect(resources).toBeVisible();
  await resources.getByRole('button', { name: 'Save resources' }).click();
  await expect(
    page.getByRole('link', { name: 'Season resources' }),
  ).toHaveAttribute('href', 'https://rsp.org.au/resources/semester-2');

  await page.goto('/admin/mentorships');
  await page.getByRole('button', { name: 'Create mentorship' }).click();
  const mentorship = page.getByRole('dialog', { name: 'Create mentorship' });
  await mentorship.getByLabel('Mentor').selectOption('person_noah');
  page.once('dialog', async (confirmation) => confirmation.dismiss());
  await page.keyboard.press('Escape');
  await expect(mentorship).toBeVisible();
  page.once('dialog', async (confirmation) => confirmation.accept());
  await page.keyboard.press('Escape');
  await expect(mentorship).toBeHidden();
});

test('received mock interviews start with their detail rows expanded', async ({
  page,
}, testInfo) => {
  test.skip(
    testInfo.project.name.includes('mobile'),
    'Desktop expanded-row presentation',
  );
  await selectDemoRole(page, 'director');
  await page.goto('/mock-interviews');
  await expect(
    page.getByRole('heading', { name: 'Mock interviews' }),
  ).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Hide details for Avery Example' }),
  ).toHaveAttribute('aria-expanded', 'true');
  await expect(
    page.getByText('Data structure implementation').first(),
  ).toBeVisible();
  await expect(
    page.getByText('Excellent testing discipline.').first(),
  ).toBeVisible();
});

test('system admin can grant a pending-MFA role and suspend and reactivate an account', async ({
  page,
}, testInfo) => {
  test.skip(
    testInfo.project.name.includes('mobile'),
    'Desktop account lifecycle operation',
  );
  await selectDemoRole(page, 'system_admin');
  await page.goto('/admin/users');
  const row = page.getByRole('row').filter({ hasText: 'Amelia Chen' });
  await row.getByRole('button', { name: 'Manage account' }).click();
  const dialog = page.getByRole('dialog', { name: 'Administer Amelia Chen' });
  await expect(dialog.getByText('amelia@example.test')).toBeVisible();
  await dialog
    .getByLabel('Reason for role grant')
    .fill('Acceptance test role assignment');
  await dialog.getByRole('button', { name: 'Grant role' }).click();
  await expect(dialog.getByRole('status')).toContainText('pending MFA');
  await dialog
    .getByLabel('Audited reason')
    .fill('Security policy acceptance test');
  await dialog
    .getByLabel(/Type Amelia Chen to confirm suspension/)
    .fill('Amelia Chen');
  await dialog.getByRole('button', { name: 'Suspend account' }).click();
  await expect(dialog.getByRole('status')).toContainText('Account suspended');
  await dialog.getByLabel('Audited reason').fill('Acceptance test completed');
  await dialog.getByRole('button', { name: 'Reactivate account' }).click();
  await expect(dialog.getByRole('status')).toContainText('Account reactivated');
});

test('attempt create, edit, delete and dirty-close confirmation work', async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name.includes('mobile'), 'Desktop CRUD coverage');
  await selectDemoRole(page, 'student');
  await page.goto('/practice');
  await page.getByRole('button', { name: 'Log attempt' }).click();
  const dialog = page.getByRole('dialog', { name: 'Log problem attempt' });
  await dialog.getByLabel('Problem').selectOption({ index: 1 });
  await dialog.getByLabel('Time taken (minutes)').fill('24');
  page.once('dialog', async (confirmation) => confirmation.dismiss());
  await dialog.getByRole('button', { name: 'Close dialog' }).click();
  await expect(dialog).toBeVisible();
  await dialog.getByRole('button', { name: 'Save attempt' }).click();
  await expect(
    page.getByRole('status').filter({ hasText: 'was added.' }),
  ).toBeVisible();

  const localRow = page
    .getByRole('row')
    .filter({ hasText: 'Binary Tree Level Order Traversal' })
    .first();
  await localRow.getByRole('button', { name: 'Edit' }).click();
  const edit = page.getByRole('dialog', {
    name: /Edit Binary Tree Level Order Traversal/,
  });
  await edit.getByLabel('Confidence (optional, 1–5)').fill('5');
  await edit.getByRole('button', { name: 'Update attempt' }).click();
  await expect(
    page.getByText('Binary Tree Level Order Traversal was saved.'),
  ).toBeVisible();

  await localRow.getByRole('button', { name: 'Delete attempt' }).click();
  const confirmation = page.getByRole('alertdialog', {
    name: /Delete attempt Binary Tree Level Order Traversal/,
  });
  await confirmation
    .getByLabel(/Type Binary Tree Level Order Traversal/)
    .fill('Binary Tree Level Order Traversal');
  await confirmation.getByRole('button', { name: 'Delete attempt' }).click();
  await expect(
    page.getByText('Binary Tree Level Order Traversal was deleted.'),
  ).toBeVisible();
});

test('director can create, edit and close a season', async ({
  page,
}, testInfo) => {
  test.skip(
    testInfo.project.name.includes('mobile'),
    'Desktop privileged workflow',
  );
  await selectDemoRole(page, 'director');
  await page.goto('/seasons');
  await page.getByRole('button', { name: 'Create season' }).click();
  const create = page.getByRole('dialog', { name: 'Create season' });
  await create.getByLabel('Name').fill('Summer 2027');
  await create.getByLabel('URL slug').fill('summer-2027');
  await create.getByLabel('Location').fill('Adelaide University');
  await create.getByRole('button', { name: 'Create season' }).click();
  await expect(page.getByRole('link', { name: 'Summer 2027' })).toBeVisible();

  await page
    .getByRole('row')
    .filter({ hasText: 'Semester 2, 2026' })
    .getByRole('link', { name: 'Semester 2, 2026' })
    .click();
  await expect(
    page.getByRole('heading', { name: 'Semester 2, 2026' }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Edit season' }).click();
  const edit = page.getByRole('dialog', { name: /Edit Semester 2, 2026/ });
  await edit.getByLabel('Location').fill('Adelaide City Campus');
  await edit.getByRole('button', { name: 'Save season' }).click();
  await expect(
    page.getByText(/Current programme at Adelaide City Campus/),
  ).toBeVisible();

  await page.getByRole('button', { name: 'Close season' }).click();
  const close = page.getByRole('dialog', {
    name: /Close season: Semester 2, 2026/,
  });
  await close.getByLabel('Reason').fill('Programme completed successfully');
  await close.getByLabel(/Type Semester 2, 2026/).fill('Semester 2, 2026');
  await close.getByRole('button', { name: 'Close season' }).click();
  await expect(page.getByText('This season is read-only.')).toBeVisible();
});

test('mock interview create, round edit and named deletion work', async ({
  page,
}, testInfo) => {
  test.skip(
    testInfo.project.name.includes('mobile'),
    'Desktop mock CRUD coverage',
  );
  await selectDemoRole(page, 'director');
  await page.goto('/mock-interviews');
  await page.getByRole('button', { name: 'New interview' }).click();
  const create = page.getByRole('dialog', { name: 'Record mock interview' });
  await create.getByLabel('Interviewee').selectOption('person_amelia');
  await create.getByRole('button', { name: 'Add LeetCode round' }).click();
  await create.getByLabel('LeetCode problem').selectOption({ index: 1 });
  await create.getByRole('button', { name: 'Save interview' }).click();
  await page.getByRole('tab', { name: 'Given' }).click();
  await expect(page.getByText('Amelia Chen').first()).toBeVisible();

  await page.getByRole('button', { name: 'Edit details' }).first().click();
  const edit = page.getByRole('dialog', { name: 'Edit interview details' });
  await edit.getByLabel('Duration (minutes)').fill('65');
  await edit
    .getByLabel(/custom score/i)
    .first()
    .fill('8');
  await edit.getByRole('button', { name: 'Save interview' }).click();
  await expect(page.getByText('65 min').first()).toBeVisible();

  await page.getByRole('button', { name: 'Delete interview' }).first().click();
  const confirmation = page.getByRole('alertdialog', {
    name: /Delete interview/,
  });
  const interviewee = await confirmation.getByText(/Type/).textContent();
  const name = interviewee?.match(/Type (.+) to confirm/)?.[1] ?? 'Amelia Chen';
  await confirmation.getByRole('textbox').fill(name);
  await confirmation.getByRole('button', { name: 'Delete interview' }).click();
});
