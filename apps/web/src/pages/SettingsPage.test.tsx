import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { PracticePreferencesPanel } from '@/pages/SettingsPage';

function renderPanel(role: string) {
  window.localStorage.setItem('rsp-demo-role', role);
  render(
    <QueryClientProvider client={new QueryClient()}>
      <PracticePreferencesPanel />
    </QueryClientProvider>,
  );
}

afterEach(() => window.localStorage.removeItem('rsp-demo-role'));

describe('practice goals', () => {
  it('shows the programme goals to a student without edit controls', async () => {
    renderPanel('student');
    for (const minutes of [10, 25, 40])
      expect(await screen.findByText(`${minutes} minutes`)).toBeVisible();
    expect(
      screen.getByText('These programme time goals apply to everyone.'),
    ).toBeVisible();
    expect(
      screen.queryByRole('button', { name: 'Edit programme goals' }),
    ).not.toBeInTheDocument();
  });

  it('lets a Director change the programme goals', async () => {
    const user = userEvent.setup();
    renderPanel('director');
    expect(
      await screen.findByText(/Programme goals: 10 \/ 25 \/ 40 minutes/),
    ).toBeVisible();
    await user.click(
      screen.getByRole('button', { name: 'Edit programme goals' }),
    );
    const dialog = await screen.findByRole('dialog');
    const easy = within(dialog).getByLabelText('Easy (minutes)');
    const save = within(dialog).getByRole('button', { name: 'Save goals' });
    expect(save).toBeDisabled();

    await user.clear(easy);
    await user.type(easy, '0');
    expect(within(dialog).getByText('Enter 1–180 minutes.')).toBeVisible();
    expect(save).toBeDisabled();

    await user.clear(easy);
    await user.type(easy, '12');
    await user.click(save);
    expect(
      await screen.findByText(/Programme goals: 12 \/ 25 \/ 40 minutes/),
    ).toBeVisible();
    expect(screen.getByText('12 minutes')).toBeVisible();
  });
});
