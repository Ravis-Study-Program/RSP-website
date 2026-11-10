import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

import { apiRequest } from '@/api/client';
import {
  useActivitySummary,
  useMockInterviewPages,
  useCurrentUser,
  useSeasons,
  useUserAttempts,
  useUserProfile,
} from '@/api/queries';
import { ProfilePage } from '@/pages/ProfilePage';
import type { CurrentUser, Person } from '@/types';

vi.mock('@/api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/client')>();
  return { ...actual, apiRequest: vi.fn() };
});

vi.mock('@/api/queries', () => ({
  currentUserOptions: { queryKey: ['current-user'] },
  demoMode: false,
  useCurrentUser: vi.fn(),
  useSeasons: vi.fn(),
  useSeasonPeople: vi.fn(() => ({ data: undefined })),
  useUserAttempts: vi.fn(),
  usePracticeGoals: vi.fn(() => ({ Easy: 10, Medium: 25, Hard: 40 })),
  useUserProfile: vi.fn(),
  useLeetcodeCategories: vi.fn(() => ({ data: [], isError: false })),
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
  useMockInterviewPages: vi.fn(() => ({
    data: undefined,
    isLoading: false,
    isError: false,
  })),
  useMockInterviewSummary: vi.fn(() => ({ data: undefined })),
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

    // Profiles are all time only, with no season picker.
    expect(
      screen.queryByRole('combobox', { name: 'View activity' }),
    ).not.toBeInTheDocument();
    expect(useActivitySummary).toHaveBeenLastCalledWith('user-1');
    expect(
      screen.getByRole('region', { name: 'Recent Activity' }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('region', { name: 'LeetCode per week' }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('region', { name: 'Mock Interviews per week' }),
    ).toBeInTheDocument();
    expect(useUserAttempts).toHaveBeenLastCalledWith('user-1', true);
    expect(useMockInterviewPages).toHaveBeenLastCalledWith(
      expect.objectContaining({ mode: 'all', userId: 'user-1' }),
      false,
    );
    await user.click(screen.getByRole('button', { name: /edit profile/i }));
    await user.click(screen.getByRole('button', { name: 'Suggest' }));

    expect(apiRequest).toHaveBeenCalledWith('/me/slug-suggestion');
    expect(await screen.findByLabelText('Profile Slug')).toHaveValue(
      'member-one-2',
    );
  });
});

describe('mentor profiles', () => {
  it('does not show the weekly charts', () => {
    const mentor: Person = {
      id: 'user-2',
      name: 'Mentor One',
      slug: 'dev-mentor',
      initials: 'MO',
      avatarUrl: null,
      roles: ['mentor'],
      season: 'Season One',
      status: 'active',
      attempts: 0,
      interviews: 32,
      lastActiveAt: null,
    };
    vi.mocked(useCurrentUser).mockReturnValue({
      data: currentUser,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useCurrentUser>);
    vi.mocked(useSeasons).mockReturnValue({
      data: { items: [] },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSeasons>);
    vi.mocked(useUserProfile).mockReturnValue({
      data: mentor,
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

    render(
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={['/people/dev-mentor']}>
          <Routes>
            <Route path="/people/:slug" element={<ProfilePage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    expect(screen.getByRole('heading', { name: 'Mentor One' })).toBeVisible();
    expect(
      screen.queryByRole('region', { name: 'Recent Activity' }),
    ).not.toBeInTheDocument();
    expect(useUserAttempts).toHaveBeenCalledWith('user-2', false);
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
    expect(screen.queryByText('Program participation')).not.toBeInTheDocument();
    expect(
      screen.queryByRole('region', { name: 'Recent Activity' }),
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /edit profile/i }));
    expect(screen.getByLabelText('Name')).toHaveValue('Member One');
    expect(screen.queryByLabelText('Profile Slug')).not.toBeInTheDocument();
  });
});
