import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { PracticePreferencesPanel } from '@/pages/SettingsPage';

const goals = { Easy: 10, Medium: 25, Hard: 40 };

describe('practice goals', () => {
  it('shows the program goals read-only to members', () => {
    render(<PracticePreferencesPanel goals={goals} />);
    for (const [label, minutes] of [
      ['Easy (minutes)', '10'],
      ['Medium (minutes)', '25'],
      ['Hard (minutes)', '40'],
    ]) {
      const field = screen.getByLabelText(label);
      expect(field).toHaveValue(minutes);
      expect(field).toHaveAttribute('readonly');
    }
  });

  it('lets coordinators and administrators edit each goal', async () => {
    const onChange = vi.fn();
    render(
      <PracticePreferencesPanel
        goals={goals}
        drafts={{ Easy: '10', Medium: '25', Hard: '0' }}
        onChange={onChange}
      />,
    );
    expect(screen.getAllByRole('spinbutton')).toHaveLength(3);
    // Each goal must be 1 to 180 minutes.
    expect(screen.getByText('Enter 1 to 180 minutes.')).toBeVisible();
    await userEvent.type(screen.getByLabelText('Easy (minutes)'), '5');
    expect(onChange).toHaveBeenLastCalledWith('Easy', '105');
  });
});
