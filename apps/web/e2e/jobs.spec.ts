import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';
import { mkdir } from 'node:fs/promises';
import path from 'node:path';

const captureRoot = process.env.RSP_JOBS_CAPTURE_DIR;

async function openApplicationHistory(page: Page, title: string) {
  const history = page.getByRole('region', {
    name: `Application history for ${title}`,
    exact: true,
  });
  if (!(await history.isVisible()))
    await page
      .getByRole('button', { name: `Show history for ${title} ·` })
      .click();
  return history;
}

async function editApplicationStatus(
  page: Page,
  title: string,
  company: string,
  status: string,
) {
  await expect(
    page.getByRole('status', {
      name: `Current stage for ${title} at ${company}`,
      exact: true,
    }),
  ).toBeVisible();
  await openApplicationHistory(page, title);
  await page
    .getByRole('button', {
      name: `Edit application for ${title} at ${company}`,
      exact: true,
    })
    .click();
  const dialog = page.getByRole('dialog', { name: 'Edit Application' });
  await dialog.getByRole('combobox', { name: 'Status', exact: true }).click();
  await page.getByRole('option', { name: status, exact: true }).click();
  await dialog.getByRole('button', { name: 'Save Changes' }).click();
  await expect(dialog).not.toBeVisible();
}

test('Application statuses keep inactive histories out of the default list', async ({
  page,
}, testInfo) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'student'),
  );
  await page.goto('/jobs');
  const titles = [
    ['Software Engineering Intern, Summer 2026', 'Google', 'Rejected'],
    [
      '2026 Software Dev Engineer Intern (Location : Sydney)',
      'Amazon',
      'Rejected',
    ],
    ['Software Engineer', 'Microsoft', 'Offer'],
    ['Software Engineer Intern Frontend/Backend 2026/27', 'Canva', 'Active'],
  ];
  for (const [title, company, status] of titles) {
    const applied = page.getByRole('checkbox', {
      name: `Applied to ${title} at ${company}`,
      exact: true,
    });
    await applied.click();
    await expect(applied).toBeChecked();
    await page
      .getByRole('tab', { name: 'My Applications', exact: true })
      .click();
    await expect(
      page.getByRole('combobox', { name: 'Application Status Filter' }),
    ).toHaveText('Active');
    if (status !== 'Active') {
      await editApplicationStatus(page, title, company, status);
      await expect(
        page.getByRole('status', {
          name: `Current stage for ${title} at ${company}`,
          exact: true,
        }),
      ).toHaveCount(0);
    }
    await page.getByRole('tab', { name: 'Positions', exact: true }).click();
    await expect(
      page.getByRole('checkbox', {
        name: `Applied to ${title} at ${company}`,
        exact: true,
      }),
    ).toBeChecked();
  }
  await page.getByRole('tab', { name: 'My Applications' }).click();
  const filter = page.getByRole('combobox', {
    name: 'Application Status Filter',
  });
  await expect(filter).toHaveText('Active');
  await expect(
    page.getByRole('status', { name: /Current stage for .* at Canva/ }),
  ).toBeVisible();
  await expect(
    page.getByRole('status', {
      name: /Current stage for .* at (Google|Amazon|Microsoft)/,
    }),
  ).toHaveCount(0);
  if (captureRoot) {
    await mkdir(captureRoot, { recursive: true });
    await page.screenshot({
      fullPage: true,
      scale: 'css',
      path: path.join(
        captureRoot,
        `${testInfo.project.name}-status-active.png`,
      ),
    });
  }
  await filter.click();
  const rejectedOption = page.getByRole('option', {
    name: 'Rejected',
    exact: true,
  });
  await expect(rejectedOption).toBeVisible();
  expect(
    await rejectedOption.evaluate((element) => {
      const text = element.firstElementChild!;
      const popup = element.closest('[data-slot="select-content"]')!;
      return (
        text.getBoundingClientRect().right <=
        popup.getBoundingClientRect().right
      );
    }),
  ).toBe(true);
  if (captureRoot)
    await page.screenshot({
      fullPage: false,
      animations: 'disabled',
      scale: 'css',
      path: path.join(
        captureRoot,
        `${testInfo.project.name}-status-filter.png`,
      ),
    });
  await page.getByRole('option', { name: 'All', exact: true }).click();
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
    const width = await page.evaluate(() => ({
      viewport: document.documentElement.clientWidth,
      content: document.documentElement.scrollWidth,
    }));
    expect(width.content).toBeLessThanOrEqual(width.viewport + 1);
    expect(
      (
        await new AxeBuilder({ page })
          .withTags(['wcag2a', 'wcag2aa', 'wcag21aa'])
          .analyze()
      ).violations,
    ).toEqual([]);
    for (const [title, company, status] of titles)
      await expect(
        page.getByRole('status', {
          name: `Current stage for ${title} at ${company}`,
          exact: true,
        }),
      ).toHaveText(status === 'Active' ? 'Applied' : status);
    if (captureRoot)
      await page.screenshot({
        fullPage: true,
        scale: 'css',
        path: path.join(
          captureRoot,
          `${testInfo.project.name}-status-all-${theme}.png`,
        ),
      });
  }
  await filter.click();
  await page.getByRole('option', { name: 'Rejected', exact: true }).click();
  await expect(
    page.getByRole('status', { name: /Current stage for .* at Amazon/ }),
  ).toBeVisible();
  await expect(
    page.getByRole('status', { name: /Current stage for .* at Google/ }),
  ).toBeVisible();
  await expect(
    page.getByRole('status', { name: /Current stage for .* at Microsoft/ }),
  ).toHaveCount(0);
  await filter.click();
  await page.getByRole('option', { name: 'Offer', exact: true }).click();
  await expect(
    page.getByRole('status', { name: /Current stage for .* at Microsoft/ }),
  ).toHaveText('Offer');
  await expect(
    page.getByRole('status', {
      name: /Current stage for .* at (Google|Amazon|Canva)/,
    }),
  ).toHaveCount(0);
  await filter.click();
  await page.getByRole('option', { name: 'Rejected', exact: true }).click();
  await editApplicationStatus(
    page,
    'Software Engineering Intern, Summer 2026',
    'Google',
    'Active',
  );
  await expect(
    page.getByRole('status', { name: /Current stage for .* at Google/ }),
  ).toHaveCount(0);
  await filter.click();
  await page.getByRole('option', { name: 'Active', exact: true }).click();
  await expect(
    page.getByRole('status', { name: /Current stage for .* at Google/ }),
  ).toHaveText('Applied');
  const history = await openApplicationHistory(
    page,
    'Software Engineering Intern, Summer 2026',
  );
  await expect(
    history
      .getByRole('list', { name: 'Application stages' })
      .getByRole('heading'),
  ).toHaveText(['Applied', 'Rejected', 'Active']);
  if (captureRoot)
    await page.screenshot({
      fullPage: true,
      animations: 'disabled',
      scale: 'css',
      path: path.join(
        captureRoot,
        `${testInfo.project.name}-status-history.png`,
      ),
    });
});

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

test('Marking Applied keeps the board open and saves a private history', async ({
  page,
}, testInfo) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'student'),
  );
  await page.goto('/jobs');
  const search = page.getByPlaceholder('Search positions…');
  await search.fill('Google');
  const applied = page.getByRole('checkbox', {
    name: 'Applied to Software Engineering Intern, Summer 2026 at Google',
  });
  await expect(applied).toBeEnabled();
  await applied.click();
  await expect(applied).toBeChecked();
  await expect(page).toHaveURL(/\/jobs$/);
  await expect(search).toHaveValue('Google');
  await page.getByRole('tab', { name: 'My Applications', exact: true }).click();
  const history = await openApplicationHistory(
    page,
    'Software Engineering Intern, Summer 2026',
  );
  await expect(
    page.getByRole('heading', {
      name: 'Software Engineering Intern, Summer 2026',
      exact: true,
    }),
  ).toBeVisible();
  await page
    .getByRole('button', { name: 'Actions for Applied', exact: true })
    .click();
  await page.getByRole('menuitem', { name: 'Edit Applied' }).click();
  const appliedForm = page.getByRole('dialog', { name: 'Edit Applied' });
  await expect(
    appliedForm.getByRole('button', { name: 'Choose applied date' }),
  ).toBeVisible();
  await expect(appliedForm.getByLabel('Stage Name')).toHaveCount(0);
  const applicationNote = 'Listed Sydney first and Melbourne second.';
  await appliedForm.getByLabel('Notes', { exact: true }).fill(applicationNote);
  await appliedForm.getByRole('button', { name: 'Save Changes' }).click();
  await expect(appliedForm).not.toBeVisible();
  await expect(history.getByText(applicationNote)).toBeVisible();
  await page.getByRole('button', { name: 'Add Stage', exact: true }).click();
  const form = page.getByRole('dialog', { name: 'Add Stage' });
  await form.getByLabel('Stage Name').last().fill('Technical interview');
  await form
    .getByLabel('Notes', { exact: true })
    .fill('Prepare graph traversal examples and questions about the team.');
  await form.getByRole('button', { name: 'Choose date' }).last().click();
  await expect(page.getByRole('button', { name: 'Next month' })).toBeVisible();
  if (captureRoot)
    await page.screenshot({
      fullPage: true,
      scale: 'css',
      path: path.join(captureRoot, `${testInfo.project.name}-stage-form.png`),
    });
  await page.keyboard.press('Escape');
  await form.getByRole('button', { name: 'Add Stage', exact: true }).click();
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

test('Applications can be removed with a single confirmation from Positions or the editor', async ({
  page,
}) => {
  await page.goto('/jobs');
  const title = 'Software Engineering Intern, Summer 2026';
  const applied = page.getByRole('checkbox', {
    name: `Applied to ${title} at Google`,
    exact: true,
  });
  const confirmation = page.getByRole('alertdialog', {
    name: `Remove application for Google (${title})?`,
    exact: true,
  });
  const confirmRemoval = async () => {
    await expect(confirmation.getByRole('textbox')).toHaveCount(0);
    await confirmation
      .getByRole('button', { name: 'Confirm', exact: true })
      .click();
    await expect(confirmation).toBeHidden();
  };

  await applied.click();
  await expect(applied).toBeChecked();
  await applied.click();
  await confirmation.getByRole('button', { name: 'Cancel' }).click();
  await expect(applied).toBeChecked();
  await applied.click();
  await confirmRemoval();
  await expect(applied).not.toBeChecked();

  await applied.click();
  await expect(applied).toBeChecked();
  await page.getByRole('tab', { name: 'My Applications' }).click();
  await page
    .getByRole('button', { name: `Edit application for ${title} at Google` })
    .click();
  const editor = page.getByRole('dialog', { name: 'Edit Application' });
  await editor.getByRole('button', { name: 'Remove Application' }).click();
  await confirmRemoval();
  await expect(editor).toBeHidden();
  await expect(page.getByText('No Active Applications')).toBeVisible();
  await page.getByRole('tab', { name: 'Positions' }).click();
  await expect(applied).not.toBeChecked();
});

test('Stage suggestions and sharing by default update the application flow', async ({
  page,
}, testInfo) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'student'),
  );
  await page.goto('/jobs?position=position-1');
  const flow = page.getByRole('region', {
    name: 'Application Flow',
    exact: true,
  });
  await expect(flow.getByRole('img', { name: /Sankey diagram/ })).toBeVisible();
  await expect(
    page.getByRole('heading', {
      name: 'Software Engineering Intern, Summer 2026',
      exact: true,
    }),
  ).toBeVisible();
  await expect(
    flow.getByText('32 applicants sharing (demo data)'),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () =>
        document.documentElement.scrollWidth <=
        document.documentElement.clientWidth + 1,
    ),
  ).toBe(true);
  if (captureRoot) {
    await flow.screenshot({
      scale: 'css',
      path: path.join(
        captureRoot,
        `${testInfo.project.name}-application-flow.png`,
      ),
    });
  }
  const applied = page.getByRole('checkbox', {
    name: 'Applied to Software Engineering Intern, Summer 2026 at Google',
  });
  await applied.click();
  await expect(applied).toBeChecked();
  await page.getByRole('tab', { name: 'My Applications', exact: true }).click();
  const editor = page.getByRole('dialog', { name: 'Edit Application' });
  const quickEdit = page.getByRole('button', {
    name: 'Edit application for Software Engineering Intern, Summer 2026 at Google',
    exact: true,
  });
  await expect(quickEdit).toHaveAttribute('title', 'Edit');
  await quickEdit.click();
  await expect(
    editor.getByRole('checkbox', { name: 'Share Progress' }),
  ).toBeChecked();
  const addStage = editor.getByRole('button', {
    name: 'Add Stage',
    exact: true,
  });
  await addStage.click();
  const firstStage = editor
    .getByRole('combobox', { name: 'Stage Name' })
    .first();
  await firstStage.press('ArrowDown');
  await expect(
    page.getByRole('option', { name: 'Recruiter screen (confirm info)' }),
  ).toBeVisible();
  for (const name of [
    'Application review',
    'Background check',
    'Reference check',
  ]) {
    await expect(page.getByRole('option', { name, exact: true })).toHaveCount(
      0,
    );
  }
  await firstStage.fill('Recruiter screen');
  await expect(
    page.getByRole('option', { name: 'Recruiter screen (behavioural)' }),
  ).toBeVisible();
  if (captureRoot) {
    await page.screenshot({
      scale: 'css',
      animations: 'disabled',
      path: path.join(
        captureRoot,
        `${testInfo.project.name}-stage-suggestions.png`,
      ),
    });
  }
  await page
    .getByRole('option', { name: 'Recruiter screen (confirm info)' })
    .click();
  await editor
    .getByRole('group', { name: 'Stage 1', exact: true })
    .getByLabel('Notes', { exact: true })
    .fill('Private demo preparation note');
  await expect(
    editor.getByRole('checkbox', { name: 'Upcoming Stage' }),
  ).toHaveCount(0);
  await editor.getByRole('button', { name: 'Save Changes' }).click();
  await expect(editor).not.toBeVisible();
  await expect(quickEdit).toBeVisible();
  await page.getByRole('tab', { name: 'Positions', exact: true }).click();
  await page
    .getByRole('button', {
      name: /^Show details for Software Engineering Intern, Summer 2026/,
    })
    .click();
  await expect(
    flow.getByText('33 applicants sharing (demo data)'),
  ).toBeVisible();
  await flow.getByText('Stage Counts', { exact: true }).click();
  const screen = flow.getByRole('row', {
    name: /^Recruiter screen \(confirm info\)/,
  });
  await expect(screen.getByRole('cell').nth(1)).toHaveText('8');
  await expect(flow.getByText('Private demo preparation note')).toHaveCount(0);
});

test('Members can publish a position with the shared form', async ({
  page,
}, testInfo) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'student'),
  );
  await page.goto('/jobs');
  await page.getByRole('button', { name: 'Add Position', exact: true }).click();
  const form = page.getByRole('dialog', { name: 'Add Position' });
  await form.getByLabel('Company', { exact: true }).fill('Example Robotics');
  await form.getByLabel('Job Title').fill('Graduate Software Engineer');
  await form
    .getByLabel('Application Link')
    .fill('https://example.com/careers/graduate');
  await form.getByLabel('Locations (One per Line)').fill('Adelaide, Australia');
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
    .getByRole('button', { name: 'Publish Position' })
    .scrollIntoViewIfNeeded();
  if (captureRoot && testInfo.project.name === 'mobile')
    await page.screenshot({
      scale: 'css',
      path: path.join(captureRoot, 'mobile-position-form-bottom.png'),
    });
  await form.getByRole('button', { name: 'Publish Position' }).click();
  await expect(form).not.toBeVisible();
  await page.getByPlaceholder('Search positions…').fill('Example Robotics');
  await expect(
    page.getByText('Graduate Software Engineer', { exact: true }),
  ).toBeVisible();
  await expect(page.getByRole('button', { name: /Manage .* at/ })).toHaveCount(
    0,
  );
});
