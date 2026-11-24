import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {
  GoogleSignInRow,
  PracticePreferencesPanel,
} from '@/pages/SettingsPage';
import { linkGoogleAccount, unlinkAccount } from '@/api/authClient';

vi.mock('@/api/authClient', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api/authClient')>()),
  linkGoogleAccount: vi.fn(),
  unlinkAccount: vi.fn(),
}));

const goals = { Easy: 10, Medium: 25, Hard: 40 };

describe('practice goals', () => {
  it('shows the program goals as non-editable to members', async () => {
    render(<PracticePreferencesPanel goals={goals} />);
    const user = userEvent.setup();
    for (const [label, minutes] of [
      ['Easy (minutes)', '10'],
      ['Medium (minutes)', '25'],
      ['Hard (minutes)', '40'],
    ]) {
      const field = screen.getByLabelText(label);
      expect(field).toHaveValue(minutes);
      expect(field).toBeDisabled();
      expect(field).toHaveClass('disabled:cursor-not-allowed');
      expect(field).toHaveClass('disabled:select-none');
      await user.type(field, '5');
      expect(field).toHaveValue(minutes);
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

describe('Google sign-in', () => {
  afterEach(() => vi.mocked(unlinkAccount).mockReset());

  it('disconnects Google when a password stays connected', async () => {
    vi.mocked(unlinkAccount).mockResolvedValue(undefined);
    const onDisconnected = vi.fn();
    render(
      <GoogleSignInRow
        methods={{ googleAccountId: 'google-1', password: true }}
        onDisconnected={onDisconnected}
        onError={vi.fn()}
      />,
    );
    expect(
      screen.getByText(/Disconnecting signs you out on all devices/),
    ).toBeVisible();
    await userEvent.click(
      screen.getByRole('button', { name: 'Disconnect Google' }),
    );
    expect(unlinkAccount).toHaveBeenCalledWith('google-1');
    expect(onDisconnected).toHaveBeenCalled();
  });

  it('asks for a password before Google, the only sign-in, can be disconnected', () => {
    render(
      <GoogleSignInRow
        methods={{ googleAccountId: 'google-1', password: false }}
        onDisconnected={vi.fn()}
        onError={vi.fn()}
      />,
    );
    expect(
      screen.getByRole('button', { name: 'Disconnect Google' }),
    ).toBeDisabled();
    expect(
      screen.getByText(/Add a password before disconnecting Google/),
    ).toBeVisible();
  });

  it('shows the auth service’s refusal and lets the member try again', async () => {
    vi.mocked(unlinkAccount).mockRejectedValue(
      new Error('Sign in again before disconnecting a sign-in method.'),
    );
    const onError = vi.fn();
    render(
      <GoogleSignInRow
        methods={{ googleAccountId: 'google-1', password: true }}
        onDisconnected={vi.fn()}
        onError={onError}
      />,
    );
    const button = screen.getByRole('button', { name: 'Disconnect Google' });
    await userEvent.click(button);
    expect(onError).toHaveBeenLastCalledWith(
      'Sign in again before disconnecting a sign-in method.',
    );
    expect(button).toBeEnabled();
  });

  it('connects Google when it is not connected', async () => {
    vi.mocked(linkGoogleAccount).mockResolvedValue(undefined as never);
    render(
      <GoogleSignInRow
        methods={{ googleAccountId: null, password: true }}
        onDisconnected={vi.fn()}
        onError={vi.fn()}
      />,
    );
    await userEvent.click(
      screen.getByRole('button', { name: 'Connect Google' }),
    );
    expect(linkGoogleAccount).toHaveBeenCalled();
  });
});
