import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { apiRequest } from '@/api/client';
import { ProblemSuggestion } from '@/components/ProblemSuggestion';

vi.mock('@/api/client', () => ({
  ApiError: class ApiError extends Error {},
  apiRequest: vi.fn(),
}));
vi.mock('@/api/queries', () => ({
  demoMode: false,
  suggestDemoProblem: vi.fn(),
  useLeetcodeCategories: () => ({ data: ['array', 'simulation'] }),
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
  render(<ProblemSuggestion />);
  await user.click(screen.getByRole('button', { name: 'Suggest a question' }));

  const link = await screen.findByRole('link');
  expect(link).toHaveTextContent(/^3379\. Transformed Array$/);
  expect(link.parentElement).toHaveTextContent(/^3379\. Transformed Array$/);
  expect(screen.queryByText(/discouraged/)).not.toBeInTheDocument();
});

it('warns against targeting a difficulty or topic once one is selected', async () => {
  const user = userEvent.setup();
  render(<ProblemSuggestion />);
  expect(screen.queryByText(/generally discouraged/)).not.toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: /^Topics/ }));
  await user.click(await screen.findByRole('checkbox', { name: 'array' }));
  expect(screen.getByText(/generally discouraged/)).toBeVisible();
});

it('sends only the chosen difficulty and topics with the request', async () => {
  vi.mocked(apiRequest).mockResolvedValue({ problem: null });
  const user = userEvent.setup();
  render(<ProblemSuggestion />);
  await user.click(screen.getByRole('button', { name: /^Difficulty/ }));
  await user.click(await screen.findByRole('checkbox', { name: 'Hard' }));
  await user.keyboard('{Escape}');
  await user.click(screen.getByRole('button', { name: 'Suggest a question' }));
  expect(apiRequest).toHaveBeenLastCalledWith(
    '/me/problem-suggestion?difficulty=hard',
  );
});
