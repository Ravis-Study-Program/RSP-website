import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { SignInPage } from './RouteStatePages';
import { signUpWithEmail } from '@/api/authClient';
import { savePendingInvitation } from '@/pendingInvitation';

vi.mock('@/api/client', () => ({ clearAccessToken: vi.fn() }));
vi.mock('@/api/queries', () => ({
  useAuthProviders: () => ({ data: { discord: true } }),
}));
vi.mock('@/api/authClient', () => ({
  isEmailVerificationError: () => false,
  requestPasswordReset: vi.fn(),
  resetPassword: vi.fn(),
  sendVerificationEmail: vi.fn(),
  signInWithEmail: vi.fn(),
  signInWithGoogle: vi.fn(),
  signInWithDiscord: vi.fn(),
  socialAuthError: () => '',
  signOut: vi.fn(),
  signUpWithEmail: vi.fn(),
  verifyTwoFactor: vi.fn(),
}));

const token = 'a'.repeat(43);

function renderSignIn(state?: unknown) {
  render(
    <MemoryRouter initialEntries={[{ pathname: '/sign-in', state }]}>
      <Routes>
        <Route path="/sign-in" element={<SignInPage />} />
        <Route path="/verify-email" element={<p>Check your email</p>} />
      </Routes>
    </MemoryRouter>,
  );
}

afterEach(() => {
  localStorage.clear();
  vi.mocked(signUpWithEmail).mockReset();
});

it('offers sign-up only to someone arriving from an invitation', () => {
  renderSignIn();
  expect(screen.queryByRole('tab', { name: 'Create Account' })).toBeNull();
  expect(
    screen.getByText(/Accounts are by invitation only/),
  ).toBeInTheDocument();
  expect(screen.queryByLabelText('Name')).toBeNull();
});

it('keeps the invited name and email and lets the invitee choose a username', async () => {
  const user = userEvent.setup();
  savePendingInvitation(token);
  vi.mocked(signUpWithEmail).mockResolvedValue({ token: null });
  renderSignIn({
    signUp: true,
    name: 'Invited Member',
    email: 'invited@example.test',
    username: 'invited-member',
  });
  expect(screen.getByRole('tab', { name: 'Create Account' })).toHaveAttribute(
    'aria-selected',
    'true',
  );
  expect(screen.queryByText(/Accounts are by invitation only/)).toBeNull();
  // Discord is only for returning members.
  expect(
    screen.queryByRole('button', { name: 'Continue with Discord' }),
  ).toBeNull();
  const name = screen.getByLabelText('Name');
  expect(name).toHaveValue('Invited Member');
  expect(name).toHaveAttribute('readonly');
  expect(screen.getByLabelText('Email')).toHaveAttribute('readonly');
  const username = screen.getByLabelText('Username');
  expect(username).toHaveValue('invited-member');
  expect(username).not.toHaveAttribute('readonly');

  await user.type(screen.getByLabelText('Password'), 'a-long-password-1');
  await user.clear(username);
  await user.type(username, 'no spaces');
  await user.click(screen.getByRole('button', { name: 'Create Account' }));
  expect(
    await screen.findByText(
      'Use 2–50 lowercase letters, numbers and single hyphens.',
    ),
  ).toBeInTheDocument();
  expect(signUpWithEmail).not.toHaveBeenCalled();

  await user.clear(username);
  await user.type(username, 'Chosen-Name');
  await user.click(screen.getByRole('button', { name: 'Create Account' }));
  expect(await screen.findByText('Check your email')).toBeInTheDocument();
  expect(signUpWithEmail).toHaveBeenCalledWith(
    'Invited Member',
    'invited@example.test',
    'a-long-password-1',
    token,
    'chosen-name',
  );
});
