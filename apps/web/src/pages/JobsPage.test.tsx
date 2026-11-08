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
      screen.getByRole('link', { name: 'Sign in to add a position' }),
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

  it('opens an applied position and saves a named stage with private notes', async () => {
    localStorage.setItem('rsp-demo-role', 'student');
    const router = renderJobs();
    const checkbox = await screen.findByRole('checkbox', {
      name: 'Applied to Software Engineering Intern, Summer 2026 at Google',
    });
    await waitFor(() => expect(checkbox).toBeEnabled());
    await userEvent.click(checkbox);
    await waitFor(() =>
      expect(router.state.location.pathname).toBe('/jobs/applications'),
    );
    await userEvent.click(
      await screen.findByRole('button', { name: 'Add stage' }),
    );
    const dialog = screen.getByRole('dialog', { name: 'Add stage' });
    await userEvent.type(
      within(dialog).getByLabelText('Stage name'),
      'Technical interview with maps team',
    );
    await userEvent.type(
      within(dialog).getByLabelText('Notes · private, optional'),
      'Prepare graph traversal examples.',
    );
    await userEvent.click(
      within(dialog).getByRole('button', { name: 'Add stage' }),
    );
    expect(
      await screen.findByText('Prepare graph traversal examples.'),
    ).toBeVisible();
    expect(
      screen.getByRole('heading', {
        name: 'Technical interview with maps team',
      }),
    ).toBeVisible();
    await userEvent.click(screen.getByRole('tab', { name: 'Positions' }));
    expect(
      await screen.findByRole('checkbox', {
        name: 'Applied to Software Engineering Intern, Summer 2026 at Google',
      }),
    ).toBeChecked();
    expect(
      screen.queryByText('Prepare graph traversal examples.'),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: /Manage .* at/ }),
    ).not.toBeInTheDocument();
  });
});
