import { render, screen, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

import { useCurrentUser, useSeasonPeople, useSeasons } from '@/api/queries';
import { MenteesPage } from '@/pages/MenteesPage';
import type { CurrentUser, Person } from '@/types';

vi.mock('@/api/queries', () => ({
  demoMode: true,
  useCurrentUser: vi.fn(),
  useSeasonPeople: vi.fn(),
  useSeasons: vi.fn(),
}));

const mentor: CurrentUser = {
  id: 'mentor-1',
  name: 'Mentor One',
  slug: 'mentor-one',
  avatarUrl: null,
  email: 'mentor@example.test',
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
      role: 'mentor',
      state: 'active',
    },
  ],
  alumni: false,
  attemptCount: 0,
  mockInterviewCount: 0,
};

function student(
  id: string,
  name: string,
  overrides: Partial<Person> = {},
): Person {
  return {
    id,
    name,
    slug: id,
    initials: name
      .split(' ')
      .map((part) => part[0])
      .join(''),
    avatarUrl: null,
    roles: ['student'],
    season: 'Season One',
    status: 'active',
    attempts: 0,
    interviews: 0,
    lastActiveAt: null,
    enrollmentId: `enrollment-${id}`,
    enrollmentState: 'active',
    ...overrides,
  };
}

describe('mentor mentee filtering', () => {
  it('shows only active students assigned to the signed-in mentor', () => {
    vi.mocked(useCurrentUser).mockReturnValue({
      data: mentor,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useCurrentUser>);
    vi.mocked(useSeasons).mockReturnValue({
      data: {
        items: [{ id: 'season-1', slug: 'season-one', name: 'Season One' }],
      },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSeasons>);
    vi.mocked(useSeasonPeople).mockReturnValue({
      data: {
        items: [
          student('assigned', 'Assigned Student', {
            mentorshipMentorId: mentor.id,
          }),
          student('another-team', 'Another Team', {
            mentorshipMentorId: 'mentor-2',
          }),
          student('unassigned', 'Unassigned Student', { status: 'unassigned' }),
          student('completed', 'Completed Student', {
            status: 'completed',
            enrollmentState: 'completed',
          }),
        ],
      },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSeasonPeople>);

    render(
      <MemoryRouter initialEntries={['/seasons/season-one/mentees']}>
        <Routes>
          <Route path="/seasons/:slug/mentees" element={<MenteesPage />} />
        </Routes>
      </MemoryRouter>,
    );

    const table = screen.getByRole('region', { name: 'Assigned mentees' });
    expect(
      within(table).getAllByText('Assigned Student').length,
    ).toBeGreaterThan(0);
    expect(screen.queryByText('Another Team')).not.toBeInTheDocument();
    expect(screen.queryByText('Unassigned Student')).not.toBeInTheDocument();
    expect(screen.queryByText('Completed Student')).not.toBeInTheDocument();
  });
});
