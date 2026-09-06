import { render, screen } from '@testing-library/react';
import { PracticePreferencesPanel } from '@/pages/SettingsPage';

describe('fixed practice goals', () => {
  it('shows automatic thresholds without customisation controls', () => {
    render(<PracticePreferencesPanel />);
    for (const minutes of [20, 35, 50])
      expect(screen.getByText(`${minutes} minutes`)).toBeVisible();
    expect(screen.queryByRole('spinbutton')).not.toBeInTheDocument();
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();
  });
});
