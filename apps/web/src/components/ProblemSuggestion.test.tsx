import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { apiRequest } from '@/api/client';
import { emptyActivityFilters } from '@/activityFilters';
import { ProblemSuggestion } from '@/components/ProblemSuggestion';

vi.mock('@/api/client', () => ({
  ApiError: class ApiError extends Error {},
  apiRequest: vi.fn(),
}));
vi.mock('@/api/queries', () => ({
  demoMode: false,
  suggestDemoProblem: vi.fn(),
}));

it('shows only the question name, numbered once', async () => {
  vi.mocked(apiRequest).mockResolvedValue({
    problem: {
      id: 'transformed-array',
      number: 3379,
      title: '3379. Transformed Array',
      link: 'https://leetcode.com/problems/transformed-array/',
      difficulty: 'easy',
      categories: ['array', 'simulation'],
      premium: false,
    },
  });
  const user = userEvent.setup();
  render(<ProblemSuggestion filters={emptyActivityFilters} />);
  await user.click(screen.getByRole('button', { name: 'Suggest a question' }));

  const link = await screen.findByRole('link');
  expect(link).toHaveTextContent(/^3379\. Transformed Array$/);
  expect(link.parentElement).toHaveTextContent(/^3379\. Transformed Array$/);
  expect(screen.queryByText(/discouraged/)).not.toBeInTheDocument();
});

it('warns against targeting a difficulty or topic once one is selected', () => {
  render(
    <ProblemSuggestion
      filters={{ ...emptyActivityFilters, categories: ['array'] }}
    />,
  );
  expect(screen.getByText(/generally discouraged/)).toBeVisible();
});
