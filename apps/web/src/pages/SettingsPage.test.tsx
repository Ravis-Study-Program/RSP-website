import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { PracticePreferencesPanel } from '@/pages/SettingsPage';

const disabledGoals = {
  premiumOptIn: false,
  goalsEnabled: false,
  easyMinutes: 20,
  mediumMinutes: 35,
  hardMinutes: 50,
  revision: 1,
};

describe('PracticePreferencesPanel', () => {
  it('lets the member change premium opt-in but not self-enable personal goals', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <PracticePreferencesPanel value={disabledGoals} onChange={onChange} />,
    );

    const premium = screen.getByRole('switch', {
      name: 'Include premium LeetCode problems',
    });
    const goals = screen.getByRole('switch', {
      name: 'Use personal time goals',
    });
    expect(premium).toBeEnabled();
    expect(goals).toHaveAttribute('aria-disabled', 'true');
    expect(goals).toHaveAttribute('tabindex', '-1');
    expect(screen.getByLabelText('Easy minutes')).toBeDisabled();
    expect(
      screen.getByText(
        /assigned mentor or programme administrator must enable/i,
      ),
    ).toBeVisible();

    await user.click(premium);
    expect(onChange).toHaveBeenCalledWith({
      ...disabledGoals,
      premiumOptIn: true,
    });
  });

  it('enables goal values after relationship-based enablement and reports invalid ranges', () => {
    const onChange = vi.fn();
    const enabledGoals = { ...disabledGoals, goalsEnabled: true, revision: 2 };
    const { rerender } = render(
      <PracticePreferencesPanel value={enabledGoals} onChange={onChange} />,
    );

    const easy = screen.getByLabelText('Easy minutes');
    expect(easy).toBeEnabled();
    fireEvent.change(easy, { target: { value: '25' } });
    expect(onChange).toHaveBeenCalledWith({ ...enabledGoals, easyMinutes: 25 });

    rerender(
      <PracticePreferencesPanel
        value={{ ...enabledGoals, easyMinutes: 181 }}
        onChange={onChange}
      />,
    );
    expect(screen.getByText('Enter 5 to 180 minutes.')).toBeVisible();
    expect(screen.getByLabelText('Easy minutes')).toHaveAttribute(
      'aria-invalid',
      'true',
    );
  });
});
