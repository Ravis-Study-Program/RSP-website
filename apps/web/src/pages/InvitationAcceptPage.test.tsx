import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { InvitationAcceptPage } from './InvitationAcceptPage';
import { apiRequest } from '@/api/client';
import { getBrowserAuthSession } from '@/api/authClient';
import { useCurrentUser } from '@/api/queries';
import { pendingInvitation, savePendingInvitation } from '@/pendingInvitation';

vi.mock('@/api/client', () => ({
  apiRequest: vi.fn(),
  clearAccessToken: vi.fn(),
}));
vi.mock('@/api/authClient', () => ({
  getBrowserAuthSession: vi.fn(),
  signOut: vi.fn(),
}));
vi.mock('@/api/queries', () => ({ useCurrentUser: vi.fn() }));
const token = 'a'.repeat(43);
function Landed() {
  return <p>Landed on {useLocation().pathname}</p>;
}
function setup(verified: boolean, signedIn = true) {
  savePendingInvitation(token);
  vi.mocked(useCurrentUser).mockReturnValue({
    data: signedIn ? { id: 'member', emailVerified: verified } : undefined,
    isLoading: false,
  } as ReturnType<typeof useCurrentUser>);
  vi.mocked(getBrowserAuthSession).mockResolvedValue(
    signedIn
      ? {
          session: { id: 'session' },
          user: {
            id: 'member',
            name: 'Member',
            email: 'member@example.test',
            emailVerified: verified,
          },
        }
      : null,
  );
  return render(
    <MemoryRouter initialEntries={['/invitations/accept']}>
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <Routes>
          <Route
            path="/invitations/accept"
            element={<InvitationAcceptPage />}
          />
          <Route path="*" element={<Landed />} />
        </Routes>
      </QueryClientProvider>
    </MemoryRouter>,
  );
}
afterEach(() => {
  localStorage.clear();
  vi.clearAllMocks();
});

it('keeps the invitation while a new or existing member signs in', async () => {
  setup(false, false);
  expect(
    await screen.findByRole('link', { name: 'Sign in or create an account' }),
  ).toHaveAttribute('href', '/sign-in');
  expect(pendingInvitation()).toBe(token);
  expect(apiRequest).not.toHaveBeenCalled();
});
it('requires email verification before revealing or accepting the invitation', async () => {
  setup(false);
  expect(
    await screen.findByRole('link', { name: 'Verify email' }),
  ).toHaveAttribute('href', '/verify-email');
  expect(apiRequest).not.toHaveBeenCalled();
});
it('accepts after review and directs a pending coordinator to MFA settings', async () => {
  vi.mocked(apiRequest).mockImplementation(async (path) =>
    path.endsWith('/preview')
      ? {
          id: 'invite',
          seasonName: 'Summer',
          seasonSlug: 'summer',
          email: 'member@example.test',
          role: 'coordinator',
        }
      : { assignmentState: 'pending_mfa' },
  );
  setup(true);
  const user = userEvent.setup();
  await user.click(
    await screen.findByRole('button', { name: 'Accept Invitation' }),
  );
  // No confirmation screen: straight on, to MFA settings for a coordinator.
  expect(await screen.findByText('Landed on /settings')).toBeInTheDocument();
  expect(apiRequest).toHaveBeenCalledWith('/invitations/accept', {
    method: 'POST',
    body: JSON.stringify({ token }),
  });
  expect(pendingInvitation()).toBe('');
});
it('forgets a saved invitation once invitations would have expired', () => {
  localStorage.setItem(
    'rsp-pending-invitation',
    JSON.stringify({ token, savedAt: Date.now() - 8 * 86400000 }),
  );
  expect(pendingInvitation()).toBe('');
  expect(localStorage.getItem('rsp-pending-invitation')).toBeNull();
});
it('sends a signed-out visitor to sign-up with the invited name and email', async () => {
  savePendingInvitation(token);
  vi.mocked(useCurrentUser).mockReturnValue({
    data: undefined,
    isLoading: false,
  } as ReturnType<typeof useCurrentUser>);
  vi.mocked(getBrowserAuthSession).mockResolvedValue(null);
  const fetchMock = vi.fn(async () =>
    Response.json({ name: 'Invited Member', email: 'invited@example.test' }),
  );
  vi.stubGlobal('fetch', fetchMock);
  function SignIn() {
    return <p>{JSON.stringify(useLocation().state)}</p>;
  }
  render(
    <MemoryRouter initialEntries={['/invitations/accept']}>
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <Routes>
          <Route
            path="/invitations/accept"
            element={<InvitationAcceptPage />}
          />
          <Route path="/sign-in" element={<SignIn />} />
        </Routes>
      </QueryClientProvider>
    </MemoryRouter>,
  );
  expect(
    await screen.findByText(
      JSON.stringify({
        signUp: true,
        name: 'Invited Member',
        email: 'invited@example.test',
      }),
    ),
  ).toBeInTheDocument();
  expect(fetchMock).toHaveBeenCalledWith(
    '/api/v2/invitations/invitee',
    expect.objectContaining({ body: JSON.stringify({ token }) }),
  );
  vi.unstubAllGlobals();
});

it('forgets an invitation that can no longer be used', async () => {
  vi.mocked(apiRequest).mockRejectedValue(
    new Error(
      'This invitation is expired, cancelled, already used, or belongs to another email address.',
    ),
  );
  setup(true);
  expect(await screen.findByText(/already used/)).toBeInTheDocument();
  expect(pendingInvitation()).toBe('');
});
