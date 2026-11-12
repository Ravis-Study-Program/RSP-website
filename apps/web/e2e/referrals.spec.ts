import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import { mkdir } from 'node:fs/promises';
import path from 'node:path';

const captureRoot = process.env.RSP_JOBS_CAPTURE_DIR;

test('Referrals require sign-in and preserve the return path', async ({
  page,
}, testInfo) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-auth-state', 'signed-out'),
  );
  await page.goto('/jobs');
  await expect(page.getByRole('tab', { name: 'Positions' })).toBeVisible();
  await expect(page.getByRole('tab', { name: 'Referrals' })).toHaveCount(0);
  if (captureRoot) {
    await mkdir(captureRoot, { recursive: true });
    await page.screenshot({
      fullPage: true,
      animations: 'disabled',
      scale: 'css',
      path: path.join(
        captureRoot,
        `${testInfo.project.name}-referrals-signed-out.png`,
      ),
    });
  }
  await page.goto('/jobs/referrals');
  await expect(page).toHaveURL(/\/sign-in\?returnTo=%2Fjobs%2Freferrals/);
  await expect(
    page.getByText('Software engineering internships only.', { exact: false }),
  ).toHaveCount(0);
});

test('Members can filter, offer, edit and stop referring', async ({
  page,
}, testInfo) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'student'),
  );
  await page.goto('/jobs/referrals');
  await expect(page.getByRole('tab', { name: 'Referrals' })).toHaveAttribute(
    'aria-selected',
    'true',
  );
  const rules = page.getByRole('region', {
    name: 'Read before contacting a referrer',
  });
  await expect(rules).toContainText('every requirement in their notes');
  await expect(rules).toContainText('Do not harass');
  await expect(rules).toContainText('unable or unwilling');
  await expect(rules).toContainText('removed from the program');
  await expect(
    page.getByText('Graduate software roles with a distinction average.', {
      exact: false,
    }),
  ).toBeVisible();
  await expect(
    page.getByRole('button', { name: /Manage your referral listing/ }),
  ).toHaveCount(0);

  if (captureRoot) await mkdir(captureRoot, { recursive: true });
  for (const theme of ['light', 'dark']) {
    if ((await page.locator('html').getAttribute('data-theme')) !== theme)
      await page
        .getByRole('button', { name: `Use ${theme} theme` })
        .filter({ visible: true })
        .first()
        .click();
    await page.evaluate(async () => {
      await document.fonts.ready;
      await Promise.all(document.getAnimations().map((a) => a.finished));
    });
    expect(
      await page.evaluate(
        () =>
          document.documentElement.scrollWidth <=
          document.documentElement.clientWidth + 1,
      ),
    ).toBe(true);
    expect(
      (
        await new AxeBuilder({ page })
          .withTags(['wcag2a', 'wcag2aa', 'wcag21aa'])
          .analyze()
      ).violations,
    ).toEqual([]);
    if (captureRoot)
      await page.screenshot({
        fullPage: true,
        animations: 'disabled',
        scale: 'css',
        path: path.join(
          captureRoot,
          `${testInfo.project.name}-referrals-${theme}.png`,
        ),
      });
  }

  await page.getByRole('combobox', { name: 'Company', exact: true }).click();
  await page.getByRole('option', { name: 'IMC', exact: true }).click();
  await expect(
    page.getByText('Graduate software roles with a distinction average.', {
      exact: false,
    }),
  ).toBeVisible();
  await expect(
    page.getByText('Software engineering internships only.', { exact: false }),
  ).toHaveCount(0);
  await page.getByRole('combobox', { name: 'Company', exact: true }).click();
  await page
    .getByRole('option', { name: 'All companies', exact: true })
    .click();

  await page
    .getByRole('button', { name: 'Open to referring', exact: true })
    .click();
  const form = page.getByRole('dialog', {
    name: 'Open to referring',
    exact: true,
  });
  await form.getByLabel('Company', { exact: true }).fill('Local Test Company');
  await page
    .getByRole('option', { name: 'Add “Local Test Company”', exact: true })
    .click();
  const help = form.getByRole('button', {
    name: 'What to include in referral notes',
  });
  await help.click();
  await expect(page.getByRole('tooltip')).toContainText('GPA thresholds');
  await expect(page.getByRole('tooltip')).toContainText(
    'Message me your resume',
  );
  if (captureRoot)
    await page.screenshot({
      animations: 'disabled',
      scale: 'css',
      path: path.join(
        captureRoot,
        `${testInfo.project.name}-referral-form.png`,
      ),
    });
  await page.keyboard.press('Escape');
  await form
    .getByLabel('Notes', { exact: true })
    .fill('Backend roles. GPA 6+. Message me your resume on Discord.');
  await form
    .getByRole('button', { name: 'Open to referring', exact: true })
    .click();
  await expect(form).toBeHidden();

  const search = page.getByPlaceholder('Search companies, people or notes…');
  await search.fill('Local Test Company');
  await expect(
    page.getByRole('link', { name: 'Avery Example', exact: true }),
  ).toHaveAttribute('href', '/people/avery-example');
  await expect(
    page.getByText(
      'Backend roles. GPA 6+. Message me your resume on Discord.',
      { exact: true },
    ),
  ).toBeVisible();
  const manage = page.getByRole('button', {
    name: 'Manage your referral listing at Local Test Company',
    exact: true,
  });
  await manage.click();
  await page.getByRole('menuitem', { name: 'Edit listing' }).click();
  const edit = page.getByRole('dialog', { name: 'Edit referral listing' });
  await edit
    .getByLabel('Notes', { exact: true })
    .fill('Graduate roles only. Please send your resume.');
  await edit.getByRole('button', { name: 'Save changes' }).click();
  await expect(edit).toBeHidden();
  await expect(
    page.getByText('Graduate roles only. Please send your resume.', {
      exact: true,
    }),
  ).toBeVisible();

  // Duplicate offers keep the form open with a useful error and do not create a second row.
  await page
    .getByRole('button', { name: 'Open to referring', exact: true })
    .click();
  await form.getByLabel('Company', { exact: true }).fill('local test company');
  await page
    .getByRole('option', { name: 'Local Test Company', exact: true })
    .click();
  await form
    .getByRole('button', { name: 'Open to referring', exact: true })
    .click();
  await expect(form).toContainText(
    'You already offer referrals at this company.',
  );
  await form.getByRole('button', { name: 'Cancel' }).click();
  await manage.click();
  await page.getByRole('menuitem', { name: 'Stop referring' }).click();
  const confirm = page.getByRole('alertdialog');
  await confirm.getByRole('textbox').fill('Local Test Company');
  await confirm
    .getByRole('button', { name: 'Stop referring', exact: true })
    .click();
  await expect(confirm).toBeHidden();
  await expect(manage).toHaveCount(0);
});
