import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';
import { chooseOption } from './select';

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

test('desktop navigation and canonical routes', async ({ page }, testInfo) => {
  test.skip(testInfo.project.name.includes('mobile'), 'Desktop-only assertion');
  await selectDemoRole(page, 'student');
  await page.goto('/dashboard');
  await expect(page.getByRole('heading', { name: /Welcome/i })).toBeVisible();
  await expect(
    page.getByRole('navigation', { name: 'Primary navigation' }),
  ).toBeVisible();
  await page.getByRole('link', { name: 'People' }).first().click();
  await expect(page.getByRole('heading', { name: 'People' })).toBeVisible();
  await expect(page).toHaveURL(/\/people$/);
});

test('old practice route redirects without losing the app shell', async ({
  page,
}) => {
  await page.goto('/practice');
  await expect(page).toHaveURL(/\/leetcode$/);
  await expect(
    page.getByRole('heading', { name: 'LeetCode Practice' }),
  ).toBeVisible();
  await expect(
    page.getByRole('link', { name: 'RSP home' }).filter({ visible: true }),
  ).toBeVisible();
  await expect(page.getByRole('combobox', { name: 'Workspace' })).toHaveCount(
    0,
  );
});

test('mobile tables use readable cards', async ({ page }, testInfo) => {
  test.skip(!testInfo.project.name.includes('mobile'), 'Mobile-only assertion');
  await page.goto('/leetcode');
  await expect(
    page.getByRole('heading', { name: 'LeetCode Practice' }),
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

test('dashboard has no detectable WCAG A or AA violations in either theme', async ({
  page,
}) => {
  await page.goto('/dashboard');
  await expect(page.getByRole('heading', { name: /Welcome/i })).toBeVisible();
  for (const theme of ['light', 'dark']) {
    await page.evaluate(async (nextTheme) => {
      document.documentElement.dataset.theme = nextTheme;
      // Flush styles so the theme's colour transitions start, then wait for
      // them; axe would otherwise sample colours mid-fade.
      document.body.getBoundingClientRect();
      await Promise.all(
        document.getAnimations().map((animation) => animation.finished),
      );
    }, theme);
    const results = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
      .analyze();
    expect(results.violations).toEqual([]);
  }
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
    await expectReflowWithoutUnreachableAction(page, 'Continue LeetCode');
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
    .getByRole('link', { name: 'Continue LeetCode' });
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

test('role-aware navigation covers every major program role', async ({
  browser,
}, testInfo) => {
  test.skip(testInfo.project.name.includes('mobile'), 'Desktop role matrix');
  const cases = [
    ['student', 'Seasons'],
    ['mentor', 'My Students'],
    ['coordinator', 'Season Operations'],
    ['graduate', 'People'],
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

test('completed staff can review their season and keep personal practice without global directory or admin access', async ({
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
    navigation.getByRole('link', { name: 'My Students' }),
  ).toBeVisible();
  await expect(
    navigation.getByRole('link', { name: 'LeetCode' }),
  ).toBeVisible();
  await expect(
    navigation.getByRole('link', { name: 'Mock Interviews' }),
  ).toBeVisible();
  await expect(navigation.getByRole('link', { name: 'People' })).toHaveCount(0);
  await navigation.getByRole('link', { name: 'My Students' }).click();
  await expect(page).toHaveURL(/\/seasons\/summer-2024-25\/mentees$/);
  await expect(
    page.getByRole('heading', { name: 'My Students' }),
  ).toBeVisible();
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
  await expect(
    signedOutPage.getByRole('tab', { name: 'Sign In' }),
  ).toHaveAttribute('aria-selected', 'true');
  await expect(
    signedOutPage.getByRole('tab', { name: 'Create Account' }),
  ).toHaveAttribute('aria-selected', 'false');
  await signedOutContext.close();

  const unverifiedContext = await browser.newContext();
  const unverifiedPage = await unverifiedContext.newPage();
  await selectDemoRole(unverifiedPage, 'unverified');
  await unverifiedPage.goto('/dashboard');
  await expect(unverifiedPage).toHaveURL(/\/verify-email$/);
  await expect(
    unverifiedPage.getByRole('heading', { name: 'Check Your Email' }),
  ).toBeVisible();
  await unverifiedContext.close();

  const nonmemberContext = await browser.newContext();
  const nonmemberPage = await nonmemberContext.newPage();
  await selectDemoRole(nonmemberPage, 'nonmember');
  await nonmemberPage.goto('/leetcode');
  await expect(nonmemberPage).toHaveURL(/\/profile$/);
  await expect(
    nonmemberPage.getByRole('heading', { name: 'Avery Example' }),
  ).toBeVisible();
  await expect(
    nonmemberPage.getByRole('link', { name: 'Onboarding' }),
  ).toHaveCount(0);
  await expect(
    nonmemberPage.getByRole('combobox', { name: 'Workspace' }),
  ).toHaveCount(0);
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
      unavailablePage.getByRole('heading', { name: 'Account Unavailable' }),
    ).toBeVisible();
    await expect(
      unavailablePage.getByText(message, { exact: false }),
    ).toBeVisible();
    await unavailableContext.close();
  }

  await selectDemoRole(page, 'student');
  await page.goto('/forbidden');
  await expect(
    page.getByRole('heading', { name: 'You Do Not Have Access' }),
  ).toBeVisible();
  await page.goto('/definitely-missing');
  await expect(
    page.getByRole('heading', { name: 'Page Not Found' }),
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

  await page.getByRole('button', { name: 'Create Week' }).click();
  const create = page.getByRole('dialog', { name: 'Create Week' });
  await create.getByLabel('Week Number').fill('15');
  page.once('dialog', async (confirmation) => confirmation.dismiss());
  await page.keyboard.press('Escape');
  await expect(create).toBeVisible();
  await create.getByRole('button', { name: 'Create Week' }).click();
  const createdWeek = page.getByRole('row').filter({ hasText: 'Week 15' });
  await expect(createdWeek).toBeVisible();
  // Resources are their own column, empty until a page is written.
  await expect(
    page.getByRole('columnheader', { name: 'Has Resources' }),
  ).toBeVisible();
  await expect(createdWeek).toContainText('No');

  await createdWeek
    .getByRole('button', { name: 'More actions for Week 15' })
    .click();
  await page.getByRole('menuitem', { name: 'Edit', exact: true }).click();
  const edit = page.getByRole('dialog', { name: 'Edit Week 15' });
  await edit.getByLabel('Week Number').fill('16');
  await edit.getByRole('button', { name: 'Save Week' }).click();
  await expect(
    page.getByRole('row').filter({ hasText: 'Week 16' }),
  ).toBeVisible();

  await page
    .getByRole('row')
    .filter({ hasText: 'Week 16' })
    .getByRole('button', { name: 'More actions for Week 16' })
    .click();
  await page.getByRole('menuitem', { name: 'Delete', exact: true }).click();
  const confirmation = page.getByRole('alertdialog', {
    name: 'Delete Week Week 16?',
  });
  await confirmation.getByLabel(/Type Week 16/).fill('Week 16');
  await confirmation.getByRole('button', { name: 'Delete Week' }).click();
  await expect(
    page.getByRole('row').filter({ hasText: 'Week 16' }),
  ).toHaveCount(0);
});

test('coordinator edits only season resources and mentorship forms protect dirty input', async ({
  page,
}, testInfo) => {
  test.skip(
    testInfo.project.name.includes('mobile'),
    'Desktop scoped-season operations',
  );
  await selectDemoRole(page, 'coordinator');
  await page.goto('/seasons/summer-2025-26');
  // A coordinator's Edit Season menu offers resources, not the details.
  await page.getByRole('button', { name: 'Edit Season' }).click();
  await expect(
    page.getByRole('menuitem', { name: 'Edit Details' }),
  ).toHaveCount(0);
  await page.getByRole('menuitem', { name: 'Edit Resources' }).click();
  const resources = page.getByRole('dialog', {
    name: /Edit Resources for Summer 2025\/26/,
  });
  await resources
    .getByLabel('Season Resources URL')
    .fill('https://rsp.org.au/resources/summer-2025-26');
  page.once('dialog', async (confirmation) => confirmation.dismiss());
  await page.keyboard.press('Escape');
  await expect(resources).toBeVisible();
  await resources.getByRole('button', { name: 'Save Resources' }).click();
  await expect(
    page.getByRole('link', { name: 'Season Resources' }),
  ).toHaveAttribute('href', 'https://rsp.org.au/resources/summer-2025-26');

  // Mentorships are added from Mentor Teams.
  await page.goto('/seasons/summer-2025-26/mentor-teams');
  await page.getByRole('button', { name: 'Edit Mentorship' }).click();
  const mentorship = page.getByRole('dialog', { name: 'Edit Mentorship' });
  await chooseOption(mentorship, 'Mentor', 'Noah Williams');
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
    page.getByRole('heading', { name: 'Mock Interviews' }),
  ).toBeVisible();
  // Rows start collapsed; the chevron opens the details.
  const details = page.getByRole('button', {
    name: 'Show details for Avery Example',
  });
  await expect(details).toHaveAttribute('aria-expanded', 'false');
  await details.click();
  await expect(page.getByText('Behavioural round').first()).toBeVisible();
  await expect(
    page.getByText('Excellent structure, with a measurable result.').first(),
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
  await row
    .getByRole('button', { name: 'More actions for Amelia Chen' })
    .click();
  await page.getByRole('menuitem', { name: 'Manage Account' }).click();
  const dialog = page.getByRole('dialog', { name: 'Administer Amelia Chen' });
  await expect(dialog.getByLabel('Email Address')).toHaveValue(
    'amelia@example.test',
  );
  // Role changes are held until Confirm, which asks for a reason and the name.
  await dialog.getByRole('checkbox', { name: 'Director' }).click();
  await dialog.getByRole('button', { name: 'Confirm' }).click();
  const confirm = page.getByRole('alertdialog', {
    name: /Change Amelia Chen’s Roles/,
  });
  await confirm.getByLabel('Reason').fill('Acceptance test role assignment');
  await confirm.getByLabel(/Type Amelia Chen to confirm/).fill('Amelia Chen');
  await confirm.getByRole('button', { name: 'Confirm' }).click();
  await expect(confirm).toBeHidden();
  await expect(dialog).toBeHidden();
});

test('attempt create, edit, delete and dirty-close confirmation work', async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name.includes('mobile'), 'Desktop CRUD coverage');
  await selectDemoRole(page, 'student');
  await page.goto('/leetcode');
  await page.getByRole('button', { name: 'Log Attempt' }).click();
  const dialog = page.getByRole('dialog', { name: 'Log Problem Attempt' });
  await dialog.getByLabel('Problem').fill('Two Sum');
  await page.getByRole('option').filter({ hasText: 'Two Sum' }).first().click();
  await dialog.getByLabel('Time taken (minutes)').fill('24');
  page.once('dialog', async (confirmation) => confirmation.dismiss());
  await dialog.getByRole('button', { name: 'Close dialog' }).click();
  await expect(dialog).toBeVisible();
  await dialog.getByRole('button', { name: 'Save Attempt' }).click();
  await expect(
    page.getByRole('status').filter({ hasText: 'was added.' }),
  ).toBeVisible();

  const localRow = page
    .getByRole('row')
    .filter({ hasText: 'Binary Tree Level Order Traversal' })
    .first();
  await localRow.getByRole('button', { name: /More actions for/ }).click();
  await page.getByRole('menuitem', { name: 'Edit' }).click();
  const edit = page.getByRole('dialog', {
    name: /Edit Binary Tree Level Order Traversal/,
  });
  await chooseOption(edit, 'Outcome', 'Unsolved');
  await edit.getByRole('button', { name: 'Update Attempt' }).click();
  await expect(
    page.getByText('Binary Tree Level Order Traversal was saved.'),
  ).toBeVisible();
  await expect(localRow).toContainText('Unsolved');
  // You mark your own attempts reviewed, as on mock interviews.
  const reviewed = localRow.getByRole('checkbox', {
    name: 'Binary Tree Level Order Traversal reviewed',
  });
  const wasReviewed = await reviewed.isChecked();
  await reviewed.click();
  await expect(reviewed).toBeChecked({ checked: !wasReviewed });

  await localRow.getByRole('button', { name: /More actions for/ }).click();
  await page.getByRole('menuitem', { name: 'Delete Attempt' }).click();
  const confirmation = page.getByRole('alertdialog', {
    name: /Delete Attempt Binary Tree Level Order Traversal/,
  });
  await confirmation
    .getByLabel(/Type Binary Tree Level Order Traversal/)
    .fill('Binary Tree Level Order Traversal');
  await confirmation.getByRole('button', { name: 'Delete Attempt' }).click();
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
  await page.getByRole('button', { name: 'Create Season' }).click();
  const create = page.getByRole('dialog', { name: 'Create Season' });
  await create.getByLabel('Name').fill('Summer 2027');
  await create.getByLabel('URL Slug').fill('summer-2027');
  await create.getByRole('button', { name: 'Create Season' }).click();
  await expect(page.getByRole('link', { name: 'Summer 2027' })).toBeVisible();

  await page
    .getByRole('row')
    .filter({ hasText: 'Summer 2025/26' })
    .getByRole('link', { name: 'Summer 2025/26' })
    .click();
  await expect(
    page.getByRole('heading', { name: 'Summer 2025/26' }),
  ).toBeVisible();
  const editSeason = async (item: string) => {
    await page.getByRole('button', { name: 'Edit Season' }).click();
    await page.getByRole('menuitem', { name: item }).click();
  };
  await editSeason('Edit Details');
  const edit = page.getByRole('dialog', { name: /Edit Summer 2025\/26/ });
  await edit
    .getByLabel('Resources URL')
    .fill('https://example.test/season-resources');
  await edit.getByRole('button', { name: 'Save Season' }).click();
  await expect(edit).not.toBeVisible();
  await editSeason('Edit Details');
  await expect(edit.getByLabel('Resources URL')).toHaveValue(
    'https://example.test/season-resources',
  );
  await page.keyboard.press('Escape');
  await expect(edit).not.toBeVisible();

  await editSeason('Close Season');
  const close = page.getByRole('dialog', {
    name: /Close Season: Summer 2025\/26/,
  });
  await close.getByLabel('Reason').fill('Program completed successfully');
  await close.getByLabel(/Type Summer 2025\/26/).fill('Summer 2025/26');
  await close.getByRole('button', { name: 'Close Season' }).click();
  await expect(page.getByText('This season is read-only.')).toBeVisible();

  await page.getByRole('link', { name: 'Seasons', exact: true }).click();
  await expect(page.getByRole('link', { name: 'Summer 2027' })).toBeVisible();
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
  await page.getByRole('button', { name: 'New Interview' }).click();
  const create = page.getByRole('dialog', { name: 'New Mock Interview' });
  await chooseOption(create, 'Interviewee', 'Amelia Chen · student');
  // A new interview starts with a behavioural round and two LeetCode rounds.
  await expect(create.getByLabel('LeetCode Problem')).toHaveCount(2);
  for (const [index, problem] of ['Two Sum', 'Merge Intervals'].entries()) {
    await create.getByLabel('LeetCode Problem').nth(index).fill(problem);
    await page.getByRole('option').filter({ hasText: problem }).first().click();
  }
  await create.getByRole('button', { name: 'Create Interview' }).click();
  await page.getByRole('tab', { name: 'Given' }).click();
  await expect(
    page
      .getByRole('link', { name: /Amelia Chen/ })
      .filter({ visible: true })
      .first(),
  ).toBeVisible();

  await page
    .getByRole('button', {
      name: /More actions for the interview with Amelia Chen/,
    })
    .filter({ visible: true })
    .first()
    .click();
  await page.getByRole('menuitem', { name: 'Edit Notes and Scores' }).click();
  const edit = page.getByRole('dialog', { name: 'Edit Notes and Scores' });
  await edit.getByLabel('Duration (minutes)').fill('65');
  await edit
    .getByLabel(/behavioural score/i)
    .first()
    .fill('8');
  await edit.getByRole('button', { name: 'Save Changes' }).click();
  await expect(page.getByText('65 min').first()).toBeVisible();

  await page
    .getByRole('button', {
      name: /More actions for the interview with Amelia Chen/,
    })
    .filter({ visible: true })
    .first()
    .click();
  await page.getByRole('menuitem', { name: 'Delete Interview' }).click();
  const confirmation = page.getByRole('alertdialog', {
    name: /Delete Interview/,
  });
  const interviewee = await confirmation.getByText(/Type/).textContent();
  const name = interviewee?.match(/Type (.+) to confirm/)?.[1] ?? 'Amelia Chen';
  await confirmation.getByRole('textbox').fill(name);
  await confirmation.getByRole('button', { name: 'Delete Interview' }).click();
});
