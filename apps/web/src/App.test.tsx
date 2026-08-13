import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import { RouterProvider } from 'react-router-dom';

import { createTestRouter } from '@/App';

function renderRoute(path: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createTestRouter([path]);
  render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>);
  return router;
}

describe('application routes', () => {
  it('renders the role-aware dashboard', async () => {
    renderRoute('/dashboard');
    expect(await screen.findByRole('heading', { name: /good evening/i })).toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: /primary/i })).toBeInTheDocument();
    expect(screen.getAllByLabelText('Workspace')[0]).toHaveValue('season_2026_s2');
  });

  it('redirects the old leetcode route to practice', async () => {
    const router = renderRoute('/leetcode');
    expect(await screen.findByRole('heading', { name: 'Problem practice' })).toBeInTheDocument();
    await waitFor(() => expect(router.state.location.pathname).toBe('/practice'));
  });

  it('redirects old season user routes to people', async () => {
    const router = renderRoute('/seasons/2026-semester-2/users');
    expect(await screen.findByRole('heading', { name: 'People' })).toBeInTheDocument();
    await waitFor(() => expect(router.state.location.pathname).toBe('/seasons/2026-semester-2/people'));
  });
});
