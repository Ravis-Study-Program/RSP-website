import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { InvitationAcceptPage } from './InvitationAcceptPage';
import { apiRequest } from '@/api/client';
import { getBrowserAuthSession } from '@/api/authClient';
import { useCurrentUser } from '@/api/queries';

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
function setup(verified: boolean, signedIn = true) {
  sessionStorage.setItem('rsp-pending-invitation', token);
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
    <MemoryRouter>
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <InvitationAcceptPage />
      </QueryClientProvider>
    </MemoryRouter>,
  );
}
afterEach(() => {
  sessionStorage.clear();
  vi.clearAllMocks();
});

it('keeps the invitation while a new or existing member signs in', async () => {
  setup(false, false);
  expect(
    await screen.findByRole('link', { name: 'Sign in or create an account' }),
  ).toHaveAttribute('href', '/sign-in');
  expect(sessionStorage.getItem('rsp-pending-invitation')).toBe(token);
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
    await screen.findByRole('button', { name: 'Accept invitation' }),
  );
  expect(await screen.findByRole('link', { name: 'Continue' })).toHaveAttribute(
    'href',
    '/settings',
  );
  expect(apiRequest).toHaveBeenCalledWith('/invitations/accept', {
    method: 'POST',
    body: JSON.stringify({ token }),
  });
  expect(sessionStorage.getItem('rsp-pending-invitation')).toBeNull();
});
