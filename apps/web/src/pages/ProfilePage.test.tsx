import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';

import { apiRequest } from '@/api/client';
import {
  useActivitySummary,
  useMockInterviews,
  useCurrentUser,
  useSeasons,
  useUserAttempts,
  useUserProfile,
} from '@/api/queries';
import { ProfilePage } from '@/pages/ProfilePage';
import type { CurrentUser } from '@/types';

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>();
  return { ...actual, apiRequest: vi.fn() };
});

vi.mock('@/api/queries', () => ({
  currentUserOptions: { queryKey: ['current-user'] },
  demoMode: false,
  useCurrentUser: vi.fn(),
  useSeasons: vi.fn(),
  useUserAttempts: vi.fn(),
  useUserProfile: vi.fn(),
  useLeetcodeProblems: vi.fn(() => ({ data: { items: [] }, isError: false })),
  useSeasonWeeks: vi.fn(() => ({ data: { items: [] }, isError: false })),
  useActivitySummary: vi.fn(() => ({
    data: {
      attemptCount: 0,
      mockInterviewCount: 0,
      mocksReceived: 0,
      mocksConducted: 0,
      lastActivityAt: null,
    },
    isError: false,
  })),
  useMockInterviews: vi.fn(() => ({
    data: { items: [] },
    isLoading: false,
    isError: false,
  })),
  useUserParticipation: vi.fn(() => ({
    data: {
      items: [
        {
          id: 'enrollment-1',
          seasonId: 'season-1',
          seasonName: 'Season One',
          startAt: '2026-01-01',
          endAt: '2026-12-31',
          role: 'student',
          state: 'active',
          lastStudentLevel: 'beginner',
          periods: [],
        },
      ],
    },
    isLoading: false,
    isError: false,
  })),
}));

const currentUser: CurrentUser = {
  id: 'user-1',
  name: 'Member One',
  slug: 'member-one',
  avatarUrl: null,
  email: 'member@example.test',
  timezone: 'Australia/Adelaide',
  timezoneConfigured: true,
  emailVerified: true,
  mfaVerified: false,
  accountState: 'active',
  globalRoles: [],
  seasonRoles: [
    {
      seasonId: 'season-1',
      seasonSlug: 'season-one',
      role: 'student',
      state: 'active',
    },
  ],
  alumni: false,
  attemptCount: 0,
  mockInterviewCount: 0,
};

describe('profile slug suggestion', () => {
  it('requests a uniqueness-aware suggestion from the API', async () => {
    vi.mocked(useCurrentUser).mockReturnValue({
      data: currentUser,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useCurrentUser>);
    vi.mocked(useSeasons).mockReturnValue({
      data: {
        items: [
          {
            id: 'season-1',
            slug: 'season-one',
            name: 'Season One',
            startsAt: '2026-01-01T00:00:00Z',
            endsAt: '2026-12-31T00:00:00Z',
          },
        ],
      },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSeasons>);
    vi.mocked(useUserProfile).mockReturnValue({
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useUserProfile>);
    vi.mocked(useUserAttempts).mockReturnValue({
      data: { items: [] },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useUserAttempts>);
    vi.mocked(apiRequest).mockResolvedValueOnce({ slug: 'member-one-2' });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const user = userEvent.setup();
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={['/profile']}>
          <ProfilePage />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(screen.getByLabelText('View activity')).toHaveValue('');
    await user.selectOptions(
      screen.getByLabelText('View activity'),
      'season-1',
    );
    expect(useActivitySummary).toHaveBeenLastCalledWith('user-1', 'season-1');
    expect(useUserAttempts).toHaveBeenLastCalledWith(
      'user-1',
      true,
      'season-1',
      expect.any(Object),
    );
    expect(useMockInterviews).toHaveBeenLastCalledWith(
      true,
      'season-1',
      undefined,
      'user-1',
    );
    await user.selectOptions(screen.getByLabelText('View activity'), '');
    expect(useActivitySummary).toHaveBeenLastCalledWith('user-1', undefined);
    await user.click(screen.getByRole('button', { name: /edit profile/i }));
    await user.click(screen.getByRole('button', { name: 'Suggest' }));

    expect(apiRequest).toHaveBeenCalledWith('/me/slug-suggestion');
    expect(await screen.findByLabelText('Profile slug')).toHaveValue(
      'member-one-2',
    );
  });
});

describe('basic profiles', () => {
  it('loads without season data and only exposes basic profile fields', async () => {
    const nonmember = {
      ...currentUser,
      seasonRoles: [],
      alumni: false,
    };
    vi.mocked(useCurrentUser).mockReturnValue({
      data: nonmember,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useCurrentUser>);
    vi.mocked(useSeasons).mockReturnValue({
      isLoading: false,
      isError: true,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSeasons>);
    vi.mocked(useUserProfile).mockReturnValue({
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useUserProfile>);
    vi.mocked(useUserAttempts).mockReturnValue({
      data: { items: [] },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useUserAttempts>);

    const user = userEvent.setup();
    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={['/profile']}>
          <ProfilePage />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(
      await screen.findByRole('heading', { name: 'Member One' }),
    ).toBeVisible();
    expect(
      screen.queryByText('Programme participation'),
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /edit profile/i }));
    expect(screen.getByLabelText('Name')).toHaveValue('Member One');
    expect(screen.queryByLabelText('Profile slug')).not.toBeInTheDocument();
  });
});
