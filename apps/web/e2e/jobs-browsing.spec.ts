import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page, type TestInfo } from '@playwright/test';
import { mkdir } from 'node:fs/promises';
import path from 'node:path';

async function capture(page: Page, info: TestInfo, surface: string) {
  for (const theme of ['light', 'dark']) {
    if ((await page.locator('html').getAttribute('data-theme')) !== theme) {
      await page
        .getByRole('button', { name: `Use ${theme} theme` })
        .filter({ visible: true })
        .first()
        .click();
    }
    await page.evaluate(async () => {
      await document.fonts.ready;
      await Promise.allSettled(document.getAnimations().map((a) => a.finished));
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
    const root = process.env.RSP_JOBS_CAPTURE_DIR;
    if (root) {
      await mkdir(root, { recursive: true });
      await page.screenshot({
        path: path.join(root, `${info.project.name}-${surface}-${theme}.png`),
        fullPage: true,
        animations: 'disabled',
        scale: 'css',
      });
    }
  }
}

test('Applied filters include rejected and offer applications and combine with other filters', async ({
  page,
}, info) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'student'),
  );
  await page.goto('/jobs');
  for (const [company, status] of [
    ['Google', 'Rejected'],
    ['Amazon', 'Offer'],
  ]) {
    await page
      .getByRole('checkbox', {
        name: new RegExp(`Applied to .* at ${company}$`),
      })
      .click();
    await expect(page).toHaveURL(/\/jobs\/applications\?application=/);
    await page
      .getByRole('combobox', {
        name: new RegExp(`Status for .* at ${company}$`),
      })
      .click();
    await page.getByRole('option', { name: status, exact: true }).click();
    await page.getByRole('tab', { name: 'Positions', exact: true }).click();
  }
  const filter = page.getByRole('combobox', {
    name: 'Application filter',
    exact: true,
  });
  await filter.click();
  await page.getByRole('option', { name: 'Not applied', exact: true }).click();
  await expect(
    page.getByRole('checkbox', { name: /Applied to .* at (Google|Amazon)$/ }),
  ).toHaveCount(0);
  await expect(
    page.getByRole('checkbox', { name: /Applied to .* at Microsoft$/ }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Filters', exact: true }).click();
  await page.getByRole('combobox', { name: 'Job Type', exact: true }).click();
  await page.getByRole('option', { name: 'Grad', exact: true }).click();
  await page.getByLabel('Location', { exact: true }).fill('Sydney');
  await page.keyboard.press('Escape');
  await expect(
    page.getByRole('checkbox', { name: /Applied to .* at Microsoft$/ }),
  ).toBeVisible();
  await expect(
    page.getByRole('checkbox', { name: /Applied to .* at Canva$/ }),
  ).toHaveCount(0);
  await page.getByRole('tab', { name: 'My Applications', exact: true }).click();
  await page.getByRole('tab', { name: 'Positions', exact: true }).click();
  await expect(filter).toHaveText('Not applied');
  await capture(page, info, 'positions');
  await filter.click();
  await page.getByRole('option', { name: 'Applied', exact: true }).click();
  await expect(
    page.getByRole('checkbox', { name: /Applied to .* at Google$/ }),
  ).toBeChecked();
  await expect(
    page.getByRole('checkbox', { name: /Applied to .* at Amazon$/ }),
  ).toBeChecked();
  await expect(
    page.getByRole('checkbox', { name: /Applied to .* at Microsoft$/ }),
  ).toHaveCount(0);
});

test('Position referrer links select the shared company and keep the conduct notice visible', async ({
  page,
}, info) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'student'),
  );
  await page.goto('/jobs');
  await page
    .getByRole('link', { name: 'View 1 referrer at Google', exact: true })
    .click();
  await expect(page).toHaveURL(/\/jobs\/referrals\?company=company-Google/);
  const company = page.getByRole('combobox', { name: 'Company', exact: true });
  await expect(company).toHaveText('Google');
  await expect(
    page.getByRole('link', { name: 'Noah Williams', exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole('link', { name: 'Zara Ahmed', exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole('region', { name: 'Read before contacting a referrer' }),
  ).toBeVisible();
  await page.reload();
  await expect(company).toHaveText('Google');
  await capture(page, info, 'company-referrals');
  await company.click();
  await page
    .getByRole('option', { name: 'All companies', exact: true })
    .click();
  await expect(
    page.getByRole('link', { name: 'Zara Ahmed', exact: true }),
  ).toBeVisible();
});

test('Added dates sort newest first and newly published positions appear first', async ({
  page,
}) => {
  await page.addInitScript(() => {
    localStorage.setItem('rsp-demo-role', 'student');
    localStorage.setItem('rsp-table-page-size', '100');
  });
  await page.goto('/jobs');
  const sort = page.getByRole('combobox', {
    name: 'Sort positions',
    exact: true,
  });
  await expect(sort).toHaveText('Newest');
  const dates = page
    .getByRole('region', { name: 'Job positions', exact: true })
    .locator('time');
  await expect(dates).toHaveCount(26);
  const timestamps = () =>
    dates.evaluateAll((elements) =>
      elements.map((e) => Date.parse(e.getAttribute('datetime')!)),
    );
  const newest = await timestamps();
  expect(newest).toEqual([...newest].sort((a, b) => b - a));
  await sort.click();
  await page.getByRole('option', { name: 'Oldest', exact: true }).click();
  await expect.poll(timestamps).toEqual([...newest].sort((a, b) => a - b));
  await sort.click();
  await page.getByRole('option', { name: 'Newest', exact: true }).click();
  await page.getByRole('button', { name: 'Add position', exact: true }).click();
  const form = page.getByRole('dialog', { name: 'Add position', exact: true });
  await form
    .getByRole('combobox', { name: 'Company', exact: true })
    .fill('Fresh Company');
  await form
    .getByLabel('Job title', { exact: true })
    .fill('Newly added engineer');
  await form
    .getByLabel('Application link', { exact: true })
    .fill('https://example.com/new-engineer');
  await form
    .getByLabel('Locations · one per line', { exact: true })
    .fill('Sydney');
  const before = Date.now();
  await form
    .getByRole('button', { name: 'Publish position', exact: true })
    .click();
  await expect(form).toBeHidden();
  await expect(dates).toHaveCount(27);
  expect((await timestamps())[0]).toBeGreaterThanOrEqual(before);
  await expect(
    page.getByRole('checkbox', { name: /Applied to/ }).first(),
  ).toHaveAccessibleName('Applied to Newly added engineer at Fresh Company');
});

test('Visitors can sort positions without seeing private application filters or referrer counts', async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-auth-state', 'signed-out'),
  );
  await page.goto('/jobs');
  await expect(
    page.getByRole('combobox', { name: 'Sort positions', exact: true }),
  ).toHaveText('Newest');
  await expect(
    page.getByRole('combobox', { name: 'Application filter', exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole('link', { name: /View \d+ referrers? at/ }),
  ).toHaveCount(0);
  await expect(
    page.getByRole('tab', { name: 'Referrals', exact: true }),
  ).toHaveCount(0);
});

test('Direct position links reveal older applied positions despite Not applied and date sorting', async ({
  page,
}) => {
  await page.addInitScript(() => {
    localStorage.setItem('rsp-demo-role', 'student');
    localStorage.setItem('rsp-table-page-size', '10');
  });
  await page.goto('/jobs?position=position-26');
  const markers = page.getByRole('checkbox', { name: /Applied to/ });
  await expect(markers.first()).toHaveAccessibleName(
    'Applied to Intern Engineer at Dolby',
  );
  const details = page.getByRole('button', {
    name: /details for Intern Engineer/i,
  });
  await expect(details).toHaveAttribute('aria-expanded', 'true');
  await markers.first().click();
  await expect(page).toHaveURL(/\/jobs\/applications\?application=/);
  await page.getByRole('tab', { name: 'Positions', exact: true }).click();
  await page
    .getByRole('combobox', { name: 'Application filter', exact: true })
    .click();
  await page.getByRole('option', { name: 'Not applied', exact: true }).click();
  await page.goBack();
  await expect(page).toHaveURL(/\/jobs\/applications\?application=/);
  await page.goBack();
  await expect(page).toHaveURL(/\/jobs\?position=position-26/);
  await expect(
    page.getByRole('combobox', { name: 'Application filter', exact: true }),
  ).toHaveText('Not applied');
  await expect(markers.first()).toHaveAccessibleName(
    'Applied to Intern Engineer at Dolby',
  );
  await expect(markers.first()).toBeChecked();
  await expect(details).toHaveAttribute('aria-expanded', 'true');
  await page
    .getByRole('combobox', { name: 'Sort positions', exact: true })
    .click();
  await page.getByRole('option', { name: 'Oldest', exact: true }).click();
  await expect(markers.first()).toHaveAccessibleName(
    'Applied to Intern Engineer at Dolby',
  );
});
