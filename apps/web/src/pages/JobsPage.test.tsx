import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { RouterProvider } from 'react-router-dom';
import { createTestRouter } from '@/App';
import { localJobDate } from '@/jobs';

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

  it('defaults to active applications and can mark Rejected and reactivate without losing history', async () => {
    localStorage.setItem('rsp-demo-role', 'student');
    renderJobs();
    const checkbox = await screen.findByRole('checkbox', {
      name: 'Applied to 2026 Software Dev Engineer Intern (Location : Sydney) at Amazon',
    });
    await waitFor(() => expect(checkbox).toBeEnabled());
    await userEvent.click(checkbox);
    const filter = () =>
      screen.getByRole('combobox', { name: 'Application status filter' });
    expect(
      await screen.findByRole('combobox', {
        name: 'Application status filter',
      }),
    ).toHaveTextContent('Active');
    const status = () =>
      screen.getByRole('combobox', {
        name: 'Status for 2026 Software Dev Engineer Intern (Location : Sydney) at Amazon',
      });
    await userEvent.click(status());
    await userEvent.click(
      await screen.findByRole('option', { name: 'Rejected' }),
    );
    await waitFor(() =>
      expect(
        screen.queryByRole('combobox', { name: /Status for .* at Amazon/ }),
      ).not.toBeInTheDocument(),
    );
    await userEvent.click(filter());
    await userEvent.click(
      await screen.findByRole('option', {
        name: 'Rejected',
      }),
    );
    expect(
      await screen.findByRole('combobox', { name: /Status for .* at Amazon/ }),
    ).toHaveTextContent('Rejected');
    await userEvent.click(filter());
    await userEvent.click(await screen.findByRole('option', { name: 'All' }));
    expect(
      await screen.findByRole('combobox', { name: /Status for .* at Amazon/ }),
    ).toHaveTextContent('Rejected');
    await userEvent.click(status());
    await userEvent.click(await screen.findByRole('option', { name: 'Offer' }));
    await waitFor(() => expect(status()).toHaveTextContent('Offer'));
    await userEvent.click(filter());
    await userEvent.click(await screen.findByRole('option', { name: 'Offer' }));
    expect(status()).toHaveTextContent('Offer');
    await userEvent.click(status());
    await userEvent.click(
      await screen.findByRole('option', { name: 'Active' }),
    );
    await waitFor(() =>
      expect(
        screen.queryByRole('combobox', { name: /Status for .* at Amazon/ }),
      ).not.toBeInTheDocument(),
    );
    await userEvent.click(filter());
    await userEvent.click(
      await screen.findByRole('option', { name: 'Active' }),
    );
    expect(status()).toHaveTextContent('Active');
    await userEvent.click(screen.getByRole('tab', { name: 'Positions' }));
    expect(
      await screen.findByRole('checkbox', { name: /Applied to .* at Amazon/ }),
    ).toBeChecked();
    await userEvent.click(screen.getByRole('tab', { name: 'My Applications' }));
    expect(
      await screen.findByRole('combobox', {
        name: 'Application status filter',
      }),
    ).toHaveTextContent('Active');
    await userEvent.click(
      screen.getByRole('button', {
        name: /Show history for 2026 Software Dev Engineer Intern/,
      }),
    );
    const history = await screen.findByRole('region', {
      name: 'Application history for 2026 Software Dev Engineer Intern (Location : Sydney)',
    });
    expect(
      within(history)
        .getAllByRole('heading')
        .map((heading) => heading.textContent),
    ).toEqual(['Applied', 'Rejected', 'Offer', 'Active']);
    for (const date of history.querySelectorAll('time')) {
      expect(date).toHaveAttribute('datetime', localJobDate());
    }
  });
});
