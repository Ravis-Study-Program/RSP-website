import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { RouterProvider } from 'react-router-dom';
import { createTestRouter } from '@/App';

function renderJobs(path = '/jobs') {
  const router = createTestRouter([path]);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

afterEach(() => {
  localStorage.removeItem('rsp-demo-auth-state');
  localStorage.removeItem('rsp-demo-role');
  sessionStorage.removeItem('rsp-auth-return-to');
});

describe('Jobs', () => {
  it('lets visitors browse positions, but sends My Applications to sign-in', async () => {
    localStorage.setItem('rsp-demo-auth-state', 'signed-out');
    const router = renderJobs();
    expect(
      await screen.findByRole('heading', { name: 'Jobs' }, { timeout: 5000 }),
    ).toBeVisible();
    expect(
      await screen.findByRole('table', { name: /Job positions/ }),
    ).toBeVisible();
    expect(
      screen.getByRole('link', { name: 'Sign In to Add a Position' }),
    ).toBeVisible();
    expect(
      screen.queryByRole('checkbox', { name: /Applied to/ }),
    ).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('tab', { name: 'My Applications' }));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe('/sign-in'),
    );
    expect(router.state.location.search).toContain(
      'returnTo=%2Fjobs%2Fapplications',
    );
  });

  it('edits stages together and keeps direct history actions', async () => {
    localStorage.setItem('rsp-demo-role', 'student');
    const router = renderJobs();
    const title = 'Software Engineering Intern, Summer 2026';
    const checkbox = await screen.findByRole('checkbox', {
      name: `Applied to ${title} at Google`,
    });
    await waitFor(() => expect(checkbox).toBeEnabled());
    await userEvent.click(checkbox);
    await waitFor(() => expect(checkbox).toBeChecked());
    expect(router.state.location.pathname).toBe('/jobs');
    await userEvent.click(screen.getByRole('tab', { name: 'My Applications' }));
    await waitFor(() =>
      expect(router.state.location.pathname).toBe('/jobs/applications'),
    );
    const showHistory = async () =>
      userEvent.click(
        await screen.findByRole('button', {
          name: /^Show history for Software Engineering Intern, Summer 2026/,
        }),
      );
    await showHistory();
    const openEditor = async () => {
      await userEvent.click(
        await screen.findByRole('button', { name: 'Edit Application' }),
      );
      return screen.findByRole('dialog', { name: 'Edit Application' });
    };
    let dialog = await openEditor();
    expect(
      within(dialog).getByRole('button', { name: /choose applied date/i }),
    ).toBeVisible();
    expect(
      within(dialog).queryByLabelText('Stage Name'),
    ).not.toBeInTheDocument();
    expect(
      within(dialog).queryByLabelText('Notes (Private, Optional)'),
    ).not.toBeInTheDocument();
    await userEvent.click(
      within(dialog).getByRole('button', { name: 'Add Stage' }),
    );
    await userEvent.type(
      within(dialog).getByLabelText('Stage Name'),
      'Phone Screen',
    );
    await userEvent.type(
      within(dialog).getByLabelText('Notes (Private, Optional)'),
      'Private preparation',
    );
    await userEvent.click(
      within(dialog).getByRole('button', { name: 'Add Stage' }),
    );
    await userEvent.type(
      within(dialog).getAllByLabelText('Stage Name')[1],
      'Technical Interview',
    );
    await userEvent.click(
      within(dialog).getByRole('button', { name: 'Move Stage 2 Up' }),
    );
    await userEvent.click(
      within(dialog).getByRole('button', { name: 'Save Changes' }),
    );
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
    );
    const history = screen.getByRole('region', {
      name: `Application history for ${title}`,
    });
    expect(
      within(within(history).getByRole('list', { name: 'Application stages' }))
        .getAllByRole('heading')
        .map((h) => h.textContent),
    ).toEqual(['Applied', 'Technical Interview', 'Phone Screen']);
    expect(
      within(history).getByRole('button', { name: 'Add Stage' }),
    ).toBeVisible();
    dialog = await openEditor();
    expect(
      within(dialog)
        .getAllByLabelText('Stage Name')
        .map((input) => (input as HTMLInputElement).value),
    ).toEqual(['Technical Interview', 'Phone Screen']);
    await userEvent.type(
      within(dialog).getAllByLabelText('Stage Name')[0],
      ' Unsaved',
    );
    await userEvent.click(
      within(dialog).getByRole('button', { name: 'Cancel' }),
    );
    expect(
      within(history).queryByText('Technical Interview Unsaved'),
    ).not.toBeInTheDocument();
    dialog = await openEditor();
    await userEvent.click(
      within(dialog).getByRole('combobox', { name: 'Status' }),
    );
    await userEvent.click(
      await screen.findByRole('option', { name: 'Rejected' }),
    );
    await userEvent.click(
      within(dialog).getByRole('button', { name: 'Save Changes' }),
    );
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
    );
    expect(
      screen.queryByRole('status', {
        name: `Status for ${title} at Google`,
      }),
    ).not.toBeInTheDocument();
    await userEvent.click(
      screen.getByRole('combobox', { name: 'Application Status Filter' }),
    );
    await userEvent.click(
      await screen.findByRole('option', { name: 'Rejected' }),
    );
    expect(
      await screen.findByRole('status', {
        name: `Status for ${title} at Google`,
      }),
    ).toBeVisible();
    expect(
      screen.queryByRole('combobox', { name: /^Status for/ }),
    ).not.toBeInTheDocument();
    if (
      !screen.queryByRole('region', {
        name: `Application history for ${title}`,
      })
    )
      await showHistory();
    expect(
      screen.getByRole('region', { name: `Application history for ${title}` }),
    ).toHaveTextContent('Rejected');
  });

  it('lets staff cancel or confirm closing a position without typing', async () => {
    localStorage.setItem('rsp-demo-role', 'coordinator');
    renderJobs();
    const title = 'Software Engineering Intern, Summer 2026';
    const manageName = `Manage ${title} at Google`;
    await userEvent.click(
      await screen.findByRole('button', { name: manageName }),
    );
    await userEvent.click(
      await screen.findByRole('menuitem', { name: 'Close Position' }),
    );
    let dialog = await screen.findByRole('alertdialog');
    expect(dialog).toHaveAccessibleName(`Close position at Google (${title})?`);
    expect(within(dialog).queryByRole('textbox')).not.toBeInTheDocument();
    expect(
      within(dialog).getByRole('button', { name: 'Confirm' }),
    ).toBeEnabled();
    await userEvent.click(
      within(dialog).getByRole('button', { name: 'Cancel' }),
    );
    expect(screen.getByRole('button', { name: manageName })).toBeVisible();

    await userEvent.click(screen.getByRole('button', { name: manageName }));
    await userEvent.click(
      await screen.findByRole('menuitem', { name: 'Close Position' }),
    );
    dialog = await screen.findByRole('alertdialog');
    await userEvent.click(
      within(dialog).getByRole('button', { name: 'Confirm' }),
    );
    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument(),
    );
    await waitFor(() =>
      expect(
        screen.queryByRole('button', { name: manageName }),
      ).not.toBeInTheDocument(),
    );
    await userEvent.click(screen.getByRole('combobox', { name: 'Listings' }));
    await userEvent.click(
      await screen.findByRole('option', { name: 'Closed Positions' }),
    );
    await userEvent.click(
      await screen.findByRole('button', { name: manageName }),
    );
    await userEvent.click(
      await screen.findByRole('menuitem', { name: 'Reopen Position' }),
    );
    await waitFor(() =>
      expect(
        screen.queryByRole('button', { name: manageName }),
      ).not.toBeInTheDocument(),
    );
  });
});
