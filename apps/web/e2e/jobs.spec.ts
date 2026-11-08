import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import { mkdir } from 'node:fs/promises';
import path from 'node:path';

const captureRoot = process.env.RSP_JOBS_CAPTURE_DIR;

test('Jobs is public and readable in both themes', async ({
  page,
}, testInfo) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-auth-state', 'signed-out'),
  );
  await page.goto('/jobs');
  await expect(page.getByRole('heading', { name: 'Jobs' })).toBeVisible();
  await expect(
    page.getByRole('link', {
      name: 'Apply for Software Engineering Intern, Summer 2026 at Google',
    }),
  ).toBeVisible();
  await expect(page.getByRole('checkbox', { name: /Applied to/ })).toHaveCount(
    0,
  );
  for (const theme of ['light', 'dark']) {
    if ((await page.locator('html').getAttribute('data-theme')) !== theme)
      await page
        .getByRole('button', { name: `Use ${theme} theme` })
        .filter({ visible: true })
        .first()
        .click();
    await page.evaluate(async () => {
      document.body.getBoundingClientRect();
      await Promise.all(document.getAnimations().map((a) => a.finished));
    });
    const metrics = await page.evaluate(() => ({
      width: document.documentElement.clientWidth,
      scroll: document.documentElement.scrollWidth,
    }));
    expect(metrics.scroll).toBeLessThanOrEqual(metrics.width + 1);
    expect(
      (
        await new AxeBuilder({ page })
          .withTags(['wcag2a', 'wcag2aa', 'wcag21aa'])
          .analyze()
      ).violations,
    ).toEqual([]);
    if (captureRoot) {
      await mkdir(captureRoot, { recursive: true });
      await page.screenshot({
        fullPage: true,
        scale: 'css',
        path: path.join(captureRoot, `${testInfo.project.name}-${theme}.png`),
      });
    }
  }
  await page.getByRole('tab', { name: 'My Applications' }).click();
  await expect(page).toHaveURL(/\/sign-in\?returnTo=/);
});

test('Applied opens a private named stage history', async ({
  page,
}, testInfo) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'student'),
  );
  await page.goto('/jobs');
  const applied = page.getByRole('checkbox', {
    name: 'Applied to Software Engineering Intern, Summer 2026 at Google',
  });
  await expect(applied).toBeEnabled();
  await applied.click();
  await expect(page).toHaveURL(/\/jobs\/applications\?application=/);
  await page.getByRole('button', { name: 'Add stage', exact: true }).click();
  const form = page.getByRole('dialog', { name: 'Add stage' });
  await form.getByLabel('Stage name').fill('Technical interview');
  await form
    .getByLabel('Notes · private, optional')
    .fill('Prepare graph traversal examples and questions about the team.');
  await form.getByRole('button', { name: 'Choose date' }).click();
  await expect(page.getByRole('button', { name: 'Next month' })).toBeVisible();
  if (captureRoot)
    await page.screenshot({
      fullPage: true,
      scale: 'css',
      path: path.join(captureRoot, `${testInfo.project.name}-stage-form.png`),
    });
  await page.keyboard.press('Escape');
  await form.getByRole('button', { name: 'Add stage', exact: true }).click();
  await expect(form).not.toBeVisible();
  await expect(
    page.getByText(
      'Prepare graph traversal examples and questions about the team.',
    ),
  ).toBeVisible();
  for (const theme of ['light', 'dark']) {
    if ((await page.locator('html').getAttribute('data-theme')) !== theme)
      await page
        .getByRole('button', { name: `Use ${theme} theme` })
        .filter({ visible: true })
        .first()
        .click();
    await page.evaluate(async () => {
      document.body.getBoundingClientRect();
      await Promise.all(document.getAnimations().map((a) => a.finished));
    });
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
        scale: 'css',
        path: path.join(
          captureRoot,
          `${testInfo.project.name}-applications-${theme}.png`,
        ),
      });
  }
  await page.getByRole('tab', { name: 'Positions' }).click();
  await expect(applied).toBeChecked();
  await expect(
    page.getByText(
      'Prepare graph traversal examples and questions about the team.',
    ),
  ).toHaveCount(0);
});

test('Members can publish a position with the shared form', async ({
  page,
}, testInfo) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'student'),
  );
  await page.goto('/jobs');
  await page.getByRole('button', { name: 'Add position', exact: true }).click();
  const form = page.getByRole('dialog', { name: 'Add position' });
  await form.getByLabel('Company', { exact: true }).fill('Example Robotics');
  await form.getByLabel('Job title').fill('Graduate Software Engineer');
  await form
    .getByLabel('Application link')
    .fill('https://example.com/careers/graduate');
  await form.getByLabel('Locations · one per line').fill('Adelaide, Australia');
  if (captureRoot)
    await page.screenshot({
      fullPage: true,
      scale: 'css',
      path: path.join(
        captureRoot,
        `${testInfo.project.name}-position-form.png`,
      ),
    });
  await form
    .getByRole('button', { name: 'Publish position' })
    .scrollIntoViewIfNeeded();
  if (captureRoot && testInfo.project.name === 'mobile')
    await page.screenshot({
      scale: 'css',
      path: path.join(captureRoot, 'mobile-position-form-bottom.png'),
    });
  await form.getByRole('button', { name: 'Publish position' }).click();
  await expect(form).not.toBeVisible();
  await page.getByPlaceholder('Search positions…').fill('Example Robotics');
  await expect(
    page.getByText('Graduate Software Engineer', { exact: true }),
  ).toBeVisible();
  await expect(page.getByRole('button', { name: /Manage .* at/ })).toHaveCount(
    0,
  );
});
