import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

import {
  useEnrollmentCandidates,
  useCurrentUser,
  usePeople,
  useSeasons,
  useSeasonPeople,
} from '@/api/queries';
import { PeoplePage } from '@/pages/PeoplePage';
import type { CurrentUser, Person, SeasonRole } from '@/types';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));

vi.mock('@/api/queries', () => ({
  demoMode: false,
  useEnrollmentCandidates: vi.fn(),
  useCurrentUser: vi.fn(() => ({ data: undefined })),
  usePeople: vi.fn(),
  useSeasons: vi.fn(),
  useSeasonPeople: vi.fn(),
  useAdminUsers: () => ({ data: undefined, isLoading: false }),
  useSeasonEnrollments: () => ({ data: { items: [] }, refetch: vi.fn() }),
}));

function member(role: SeasonRole): Person {
  const label = role[0].toUpperCase() + role.slice(1);
  return {
    id: role,
    name: `${label} Member`,
    slug: `${role}-member`,
    initials: `${label[0]}M`,
    avatarUrl: null,
    roles: [role],
    seasonRole: role,
    studentLevel: role === 'student' ? 'beginner' : 'not_applicable',
    season: 'Season One',
    status: 'active',
    attempts: 0,
    interviews: 0,
    lastActiveAt: null,
    enrollmentId: `enrollment-${role}`,
    enrollmentState: 'active',
  };
}

describe('season people directory', () => {
  beforeEach(() => {
    vi.mocked(useCurrentUser).mockReturnValue({ data: undefined } as ReturnType<
      typeof useCurrentUser
    >);
    vi.mocked(useSeasons).mockReturnValue({
      data: {
        items: [
          {
            id: 'season-1',
            slug: 'season-one',
            name: 'Season One',
            status: 'open',
          },
        ],
      },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSeasons>);
    vi.mocked(useSeasonPeople).mockReturnValue({
      data: {
        items: [member('student'), member('mentor'), member('coordinator')],
      },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSeasonPeople>);
    vi.mocked(usePeople).mockReturnValue({
      data: { items: [] },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof usePeople>);
    vi.mocked(useEnrollmentCandidates).mockReturnValue({
      data: { items: [] },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useEnrollmentCandidates>);
  });

  function renderDirectory() {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={['/seasons/season-one/people']}>
          <Routes>
            <Route path="/seasons/:slug/people" element={<PeoplePage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );
    return client;
  }

  it('renders enrollment roles and keeps level actions hidden without permission', () => {
    renderDirectory();
    const directory = screen.getByRole('region', { name: 'People directory' });
    for (const label of ['Student', 'Mentor', 'Coordinator']) {
      const row = within(directory).getByRole('row', {
        name: new RegExp(`${label} Member`),
      });
      expect(within(row).getByText(label, { selector: 'span' })).toBeVisible();
    }
    expect(
      screen.queryByRole('button', { name: 'Edit details' }),
    ).not.toBeInTheDocument();
  });

  function setMentor(seasonId = 'season-1') {
    vi.mocked(useCurrentUser).mockReturnValue({
      data: {
        id: 'mentor',
        name: 'Mentor',
        slug: 'mentor',
        avatarUrl: null,
        email: 'mentor@example.test',
        timezone: 'Australia/Adelaide',
        timezoneConfigured: true,
        mfaVerified: false,
        alumni: false,
        attemptCount: 0,
        mockInterviewCount: 0,
        emailVerified: true,
        accountState: 'active',
        globalRoles: [],
        seasonRoles: [
          {
            seasonId,
            seasonSlug: 'season-one',
            role: 'mentor',
            state: 'active',
          },
        ],
      } as CurrentUser,
    } as ReturnType<typeof useCurrentUser>);
  }

  it('lets a mentor manage only their own students', () => {
    setMentor();
    vi.mocked(useSeasonPeople).mockReturnValue({
      data: {
        items: [
          { ...member('student'), mentorshipMentorId: 'mentor' },
          { ...member('student'), id: 'other', name: 'Other Student' },
          member('coordinator'),
        ],
      },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSeasonPeople>);
    renderDirectory();
    expect(
      screen.getByRole('button', { name: 'More actions for Student Member' }),
    ).toBeInTheDocument();
    for (const name of ['Other Student', 'Coordinator Member'])
      expect(
        screen.queryByRole('button', { name: `More actions for ${name}` }),
      ).not.toBeInTheDocument();
  });

  it('lets a coordinator manage everyone in their season', () => {
    setMentor();
    const viewer = vi.mocked(useCurrentUser)().data!;
    vi.mocked(useCurrentUser).mockReturnValue({
      data: {
        ...viewer,
        seasonRoles: [{ ...viewer.seasonRoles[0], role: 'coordinator' }],
      },
    } as ReturnType<typeof useCurrentUser>);
    renderDirectory();
    for (const name of [
      'Student Member',
      'Mentor Member',
      'Coordinator Member',
    ])
      expect(
        screen.getByRole('button', { name: `More actions for ${name}` }),
      ).toBeInTheDocument();
  });
});
