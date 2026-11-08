import { expect, test } from '@playwright/test';

for (const [role, staffLink] of [
  ['student', null],
  ['mentor', 'My Students'],
  ['coordinator', 'All Students'],
  ['director', 'Administration'],
  ['system_admin', 'Administration'],
] as const) {
  test(`${role} keeps shared navigation alongside staff access`, async ({
    page,
  }, testInfo) => {
    await page.addInitScript(
      (role) => localStorage.setItem('rsp-demo-role', role),
      role,
    );
    await page.goto('/dashboard');
    await expect(page.getByRole('heading', { name: /Welcome/ })).toBeVisible();
    await expect(page.getByRole('combobox', { name: 'Workspace' })).toHaveCount(
      0,
    );
    const mobile = testInfo.project.name.includes('mobile');
    const navigation = page.getByRole('navigation', {
      name: mobile ? 'Mobile navigation' : 'Primary navigation',
    });
    for (const [label, path, title] of [
      ['LeetCode', '/leetcode', 'LeetCode Practice'],
      ['Mock Interviews', '/mock-interviews', 'Mock Interviews'],
    ]) {
      await navigation.getByRole('link', { name: label, exact: true }).click();
      await expect(page).toHaveURL(new RegExp(`${path}$`));
      await expect(
        page.getByRole('heading', { name: title, exact: true }),
      ).toBeVisible();
    }

    // A historical season URL must not replace personal links or admin access.
    await page.goto('/seasons/summer-2024-25/leetcode');
    await expect(
      page.getByRole('heading', { name: 'LeetCode Practice' }),
    ).toBeVisible();
    await expect(
      navigation.getByRole('link', { name: 'LeetCode', exact: true }),
    ).toHaveAttribute('href', '/leetcode');
    await expect(
      navigation.getByRole('link', { name: 'LeetCode', exact: true }),
    ).toHaveAttribute('aria-current', 'page');

    if (staffLink) {
      let staffNavigation = navigation;
      if (
        mobile &&
        !(await navigation
          .getByRole('link', { name: staffLink, exact: true })
          .isVisible())
      ) {
        await navigation
          .getByRole('button', { name: 'More', exact: true })
          .click();
        staffNavigation = page.getByRole('dialog');
      }
      await staffNavigation
        .getByRole('link', { name: staffLink, exact: true })
        .click();
      await expect(
        page.getByRole('heading', { name: staffLink, exact: true }),
      ).toBeVisible();
    }

    await page.getByRole('button', { name: 'Search pages' }).click();
    const quickNavigation = page.getByRole('navigation', {
      name: 'Quick navigation results',
    });
    await expect(
      quickNavigation.getByRole('link', {
        name: 'LeetCode Problem attempt history',
      }),
    ).toHaveAttribute('href', '/leetcode');
    await expect(
      quickNavigation.getByRole('link', { name: /Mock Interviews/ }),
    ).toHaveAttribute('href', '/mock-interviews');
    if (staffLink)
      await expect(
        quickNavigation.getByRole('link', { name: new RegExp(staffLink) }),
      ).toBeVisible();
    else
      await expect(
        quickNavigation.getByRole('link', {
          name: /My Students|All Students|Administration|Season Operations/,
        }),
      ).toHaveCount(0);
  });
}

test('Seasons keeps a mentor’s historical student list reachable', async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'former_member'),
  );
  await page.goto('/seasons');
  await page
    .getByRole('main')
    .getByRole('link', { name: /Summer 2024\/25|Open/ })
    .filter({ visible: true })
    .first()
    .click();
  await expect(
    page.getByRole('heading', { name: 'Summer 2024/25' }),
  ).toBeVisible();
  await page
    .getByRole('main')
    .getByRole('link', { name: 'My Students', exact: true })
    .click();
  await expect(
    page.getByRole('heading', { name: 'My Students' }),
  ).toBeVisible();
  await expect(page).toHaveURL(/\/seasons\/summer-2024-25\/mentees$/);
});
