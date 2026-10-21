import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Spoiler } from '@/components/Spoiler';

it('hides the content until it is revealed', async () => {
  const user = userEvent.setup();
  render(<Spoiler>Array, Simulation</Spoiler>);
  expect(screen.queryByText('Array, Simulation')).not.toBeInTheDocument();

  await user.click(screen.getByRole('button', { name: 'Show topics' }));

  expect(screen.getByText('Array, Simulation')).toBeVisible();
  expect(
    screen.queryByRole('button', { name: 'Show topics' }),
  ).not.toBeInTheDocument();
});
