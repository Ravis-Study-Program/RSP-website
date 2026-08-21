import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import { RouterProvider } from 'react-router-dom';

import { createTestRouter } from '@/App';

const routeTimeout = { timeout: 5_000 };

function renderRoute(path: string) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const router = createTestRouter([path]);
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

describe('application routes', () => {
  it('renders the role-aware dashboard', async () => {
    renderRoute('/dashboard');
    expect(
      await screen.findByRole('heading', { name: /welcome/i }, routeTimeout),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('navigation', { name: /primary/i }),
    ).toBeInTheDocument();
    expect(screen.getAllByLabelText('Workspace')[0]).toHaveValue('director');
  });

  it('redirects former members from the site root to the dashboard', async () => {
    window.localStorage.setItem('rsp-demo-role', 'former_member');
    const router = renderRoute('/');
    expect(
      await screen.findByRole('heading', { name: /welcome/i }, routeTimeout),
    ).toBeInTheDocument();
    await waitFor(
      () => expect(router.state.location.pathname).toBe('/dashboard'),
      routeTimeout,
    );
    window.localStorage.removeItem('rsp-demo-role');
  });

  it('redirects the old leetcode route to practice', async () => {
    const router = renderRoute('/leetcode');
    expect(
      await screen.findByRole(
        'heading',
        { name: 'Problem practice' },
        routeTimeout,
      ),
    ).toBeInTheDocument();
    await waitFor(
      () => expect(router.state.location.pathname).toBe('/practice'),
      routeTimeout,
    );
  });

  it('redirects old season user routes to people', async () => {
    const router = renderRoute('/seasons/2026-semester-2/users');
    expect(
      await screen.findByRole('heading', { name: 'People' }, routeTimeout),
    ).toBeInTheDocument();
    await waitFor(
      () =>
        expect(router.state.location.pathname).toBe(
          '/seasons/2026-semester-2/people',
        ),
      routeTimeout,
    );
  });

  it.each([
    '/profile?user=amelia-chen',
    '/seasons/2026-semester-2/profile?user=amelia-chen',
  ])(
    'redirects legacy profile queries to the canonical public profile',
    async (path) => {
      const router = renderRoute(path);
      expect(
        await screen.findByRole(
          'heading',
          { name: 'Amelia Chen' },
          routeTimeout,
        ),
      ).toBeInTheDocument();
      await waitFor(
        () =>
          expect(router.state.location.pathname).toBe('/people/amelia-chen'),
        routeTimeout,
      );
    },
  );
});
