import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import { mkdir } from 'node:fs/promises';
import path from 'node:path';

const captureRoot = process.env.RSP_JOBS_CAPTURE_DIR;

test('Referrals require sign-in and an eligible RSP enrollment', async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-auth-state', 'signed-out'),
  );
  await page.goto('/jobs/referrals?company=company-Google');
  await expect(page).toHaveURL(
    /\/sign-in\?returnTo=%2Fjobs%2Freferrals%3Fcompany%3Dcompany-Google/,
  );
  await page.goto('/jobs/refer-candidates');
  await expect(page).toHaveURL(
    /\/sign-in\?returnTo=%2Fjobs%2Frefer-candidates/,
  );
});

test('Verified accounts without an RSP enrollment cannot open Referrals', async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'nonmember'),
  );
  for (const route of ['/jobs/referrals', '/jobs/refer-candidates']) {
    await page.goto(route);
    await expect(page).toHaveURL(/\/jobs$/);
    await expect(
      page.getByRole('tab', { name: 'Request Referrals', exact: true }),
    ).toHaveCount(0);
    await expect(
      page.getByRole('tab', { name: 'Refer Candidates' }),
    ).toHaveCount(0);
  }
  await expect(
    page.getByRole('link', { name: /Browse referral positions/ }),
  ).toHaveCount(0);
});

test('Other RSP members can open Refer Candidates from the old referrals path', async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'coordinator'),
  );
  await page.goto('/jobs/referrals');
  await expect(page).toHaveURL(/\/jobs\/refer-candidates$/);
  await expect(
    page.getByRole('tab', { name: 'Refer Candidates' }),
  ).toHaveAttribute('aria-selected', 'true');
  await expect(
    page.getByRole('tab', { name: 'Request Referrals', exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole('button', { name: 'Open to Referring' }),
  ).toHaveCount(0);
});

test('Referral pages show and remember their own guidelines for the current account', async ({
  page,
}) => {
  await page.addInitScript(() => {
    localStorage.setItem('rsp-demo-role', 'student');
    localStorage.setItem(
      'rsp-referral-guidelines-ack:v3:other-account:referrals',
      '1',
    );
  });
  await page.goto('/jobs/referrals');
  const rules = page.getByRole('region', {
    name: 'Request Referral Guidelines',
  });
  await expect(rules).toBeVisible();
  await rules
    .getByRole('checkbox', { name: 'I acknowledge the referral guidelines' })
    .press('Space');
  const headingRow = page
    .getByRole('heading', { name: 'Jobs', exact: true })
    .locator('..');
  const guidelines = headingRow.getByRole('button', {
    name: 'Referral Guidelines',
    exact: true,
  });
  await expect(guidelines).toBeFocused();
  await page.reload();
  await expect(guidelines).toHaveAttribute('aria-expanded', 'false');
  await expect(rules).toBeHidden();
  const tabs = page.getByRole('tablist', { name: 'Jobs' });
  const tabsPosition = await tabs.boundingBox();
  await guidelines.click();
  const popover = page.getByRole('dialog', {
    name: 'Request Referral Guidelines',
  });
  await expect(popover).toContainText(
    'Your résumé link and reason are shared with members who opt in to refer',
  );
  expect(await tabs.boundingBox()).toEqual(tabsPosition);
  await page.keyboard.press('Escape');
  await expect(popover).toBeHidden();
  await expect(guidelines).toBeFocused();
  await tabs.getByRole('tab', { name: 'Refer Candidates' }).click();
  await expect(page).toHaveURL(/\/jobs\/refer-candidates$/);
  await expect(
    page.getByText('Referring Is Available to RSP Graduates'),
  ).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Open to Referring' }),
  ).toHaveCount(0);
  await expect(page.getByRole('button', { name: /^View Profile/ })).toHaveCount(
    0,
  );
  const referrerRules = page.getByRole('region', {
    name: 'Referrer Guidelines',
  });
  await expect(referrerRules).toContainText(
    'Use candidates’ information only for the referrals they requested.',
  );
  await referrerRules
    .getByRole('checkbox', { name: 'I acknowledge the referral guidelines' })
    .press('Space');
  await expect(guidelines).toBeFocused();
  await page.reload();
  await expect(referrerRules).toBeHidden();
  await guidelines.click();
  await expect(
    page.getByRole('dialog', { name: 'Referrer Guidelines' }),
  ).toBeVisible();
});

test('Students reuse a profile, request up to five open roles and withdraw requests', async ({
  page,
}, testInfo) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'student'),
  );
  await page.goto('/jobs/referrals');
  await expect(
    page.getByRole('tab', { name: 'Request Referrals', exact: true }),
  ).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByText('0 of 5 requested')).toBeVisible();
  const controls = page.getByRole('checkbox', {
    name: /^Request referral for /,
  });
  const google = page.getByRole('checkbox', {
    name: /Request referral for .* at Google$/,
  });
  const notifications = page.getByRole('region', {
    name: 'Referral notifications',
  });
  await google.click();
  const missingProfileNotice = notifications.getByText(
    'Create a referral profile first',
    { exact: true },
  );
  const profile = page.getByRole('dialog', { name: 'Create Referral Profile' });
  await expect(missingProfileNotice).toBeVisible();
  await expect(google).not.toBeChecked();
  await expect(profile).toBeHidden();
  const header = page.locator('header').filter({
    has: page.getByRole('heading', { name: 'Jobs', exact: true }),
  });
  await expect(
    header.getByRole('button', { name: 'My Requests' }),
  ).toBeVisible();
  await header.getByRole('button', { name: 'Create Referral Profile' }).click();
  await expect(profile).toBeVisible();
  await expect(missingProfileNotice).toBeHidden();
  await profile
    .getByRole('textbox', { name: /Why I’d Be a Good Referral/ })
    .fill('I build and maintain software for my university club.');
  const resume = profile.getByLabel('Résumé Link');
  await expect(resume).toHaveAccessibleDescription(
    /Provide a public link to your résumé \(e.g. Google Drive\)/,
  );
  await resume.fill('http://localhost/resume.pdf');
  await profile.getByRole('button', { name: 'Save Profile' }).click();
  await expect(
    profile.getByText('Enter a public link to your résumé.'),
  ).toBeVisible();
  const resumeUrl =
    'https://portfolio.example.org/resume.pdf?download=1#page=2';
  await resume.fill(resumeUrl);
  await expect(
    profile.getByRole('link', { name: 'View Résumé' }),
  ).toHaveAttribute('href', resumeUrl);
  await profile.getByRole('button', { name: 'Save Profile' }).click();
  await expect(profile).toBeHidden();
  await page.getByRole('button', { name: 'Edit Referral Profile' }).click();
  const editProfile = page.getByRole('dialog', {
    name: 'Edit Referral Profile',
  });
  await expect(editProfile.getByLabel('Résumé Link')).toHaveValue(resumeUrl);
  await editProfile.getByRole('button', { name: 'Close dialog' }).click();
  await google.click();
  await expect(google).toBeChecked();
  for (let i = 0; i < 4; ++i) {
    await page
      .getByRole('checkbox', { checked: false, name: /^Request referral for / })
      .first()
      .click();
    await expect(page.getByText(`${i + 2} of 5 requested`)).toBeVisible();
  }
  const extraRequest = page
    .getByRole('checkbox', { checked: false, name: /^Request referral for / })
    .first();
  await extraRequest.click();
  const limitNotice = notifications.getByText(
    'Referral request limit reached',
    {
      exact: true,
    },
  );
  await expect(limitNotice).toBeVisible();
  await expect(extraRequest).not.toBeChecked();
  await expect(page.getByText('5 of 5 requested')).toBeVisible();
  await page.mouse.move(0, 0);
  if (captureRoot) {
    await mkdir(captureRoot, { recursive: true });
    await page.screenshot({
      path: path.join(
        captureRoot,
        `${testInfo.project.name}-referral-limit.png`,
      ),
      animations: 'disabled',
    });
  }
  await expect(limitNotice).toBeHidden({ timeout: 6500 });
  const myRequests = page.getByRole('button', { name: 'My Requests' });
  await myRequests.click();
  await expect(controls).toHaveCount(5);
  await google.click();
  await expect(page.getByText('4 of 5 requested')).toBeVisible();
  await expect(controls).toHaveCount(4);
  await page.getByRole('button', { name: 'Edit Referral Profile' }).click();
  const edit = page.getByRole('dialog', { name: 'Edit Referral Profile' });
  await expect(
    edit.getByRole('textbox', { name: /Why I’d Be a Good Referral/ }),
  ).toHaveValue('I build and maintain software for my university club.');
  await edit.getByRole('button', { name: 'Cancel' }).click();
  for (const theme of ['light', 'dark']) {
    if ((await page.locator('html').getAttribute('data-theme')) !== theme)
      await page
        .getByRole('button', { name: `Use ${theme} theme` })
        .filter({ visible: true })
        .first()
        .click();
    await page.evaluate(async () => {
      await document.fonts.ready;
      await Promise.all(
        document.getAnimations().map((animation) => animation.finished),
      );
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
    if (captureRoot) {
      await mkdir(captureRoot, { recursive: true });
      await page.screenshot({
        path: path.join(
          captureRoot,
          `${testInfo.project.name}-referral-requests-${theme}.png`,
        ),
        animations: 'disabled',
      });
    }
  }
});

test('Referrers opt in, review matching candidate profiles and stop referring', async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'graduate'),
  );
  await page.goto('/jobs/referrals');
  await page.getByRole('tab', { name: 'Refer Candidates' }).click();
  await expect(page).toHaveURL(/\/jobs\/refer-candidates$/);
  await expect(
    page.getByRole('tab', { name: 'Refer Candidates' }),
  ).toHaveAttribute('aria-selected', 'true');
  await expect(
    page.getByText('Choose a Company to Start Referring'),
  ).toBeVisible();
  await page
    .locator('header')
    .filter({
      has: page.getByRole('heading', { name: 'Jobs', exact: true }),
    })
    .getByRole('button', { name: 'Open to Referring' })
    .click();
  const offer = page.getByRole('dialog', { name: 'Open to Referring' });
  await offer.getByRole('combobox', { name: 'Company' }).click();
  await page.getByRole('option', { name: 'Google', exact: true }).click();
  await offer.getByRole('button', { name: 'Open to Referring' }).click();
  await page
    .getByRole('button', { name: 'View Profile for Zara Ahmed' })
    .click();
  const profile = page.getByRole('dialog', { name: 'Zara Ahmed' });
  await expect(profile).toContainText(
    'Software Engineering Intern, Summer 2026',
  );
  await expect(profile).not.toContainText('Frontend/Backend');
  await expect(profile).toContainText('Search for Zara Ahmed on Discord');
  const resume = profile.getByRole('link', {
    name: 'View Résumé',
  });
  const resumeUrl = 'https://drive.google.com/file/d/rsp-demo-zara-ahmed/view';
  await expect(resume).toHaveAttribute('href', resumeUrl);
  await expect(resume).toHaveAttribute('target', '_blank');
  await page
    .context()
    .route(resumeUrl, (route) =>
      route.fulfill({ contentType: 'text/plain', body: 'Demo résumé link' }),
    );
  const [resumeTab] = await Promise.all([
    page.waitForEvent('popup'),
    resume.click(),
  ]);
  await expect(resumeTab).toHaveURL(resumeUrl);
  await resumeTab.close();
  await profile.getByRole('button', { name: 'Close dialog' }).click();
  await page.getByRole('button', { name: 'Stop Referring' }).click();
  await page
    .getByRole('alertdialog')
    .getByRole('button', { name: 'Stop Referring' })
    .click();
  await expect(
    page.getByRole('button', { name: 'View Profile for Zara Ahmed' }),
  ).toHaveCount(0);
});
