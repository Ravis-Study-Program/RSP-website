import { fireEvent, render, screen } from '@testing-library/react';

import { PracticePreferencesPanel } from '@/pages/SettingsPage';

const disabledGoals = {
  goalsEnabled: false,
  easyMinutes: 10,
  mediumMinutes: 20,
  hardMinutes: 45,
};
const enabledGoals = { ...disabledGoals, goalsEnabled: true };

describe('PracticePreferencesPanel', () => {
  it('does not let the member self-enable personal goals', () => {
    const onChange = vi.fn();
    render(
      <PracticePreferencesPanel value={disabledGoals} onChange={onChange} />,
    );

    const goals = screen.getByRole('switch', {
      name: 'Use personal time goals',
    });
    expect(goals).toHaveAttribute('aria-disabled', 'true');
    expect(goals).toHaveAttribute('tabindex', '-1');
    expect(screen.getByLabelText('Easy minutes')).toBeDisabled();
    expect(
      screen.getByText(
        /assigned mentor or programme administrator must enable/i,
      ),
    ).toBeVisible();

    expect(onChange).not.toHaveBeenCalled();
  });

  it('enables goal values after relationship-based enablement and reports invalid ranges', () => {
    const onChange = vi.fn();
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
    expect(screen.getByText('Enter 1 to 180 minutes.')).toBeVisible();
    expect(screen.getByLabelText('Easy minutes')).toHaveAttribute(
      'aria-invalid',
      'true',
    );
  });
});
