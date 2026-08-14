import { render, screen, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

import { useSeasonPeople, useSeasons } from '@/api/queries';
import { MentorTeamsPage } from '@/pages/MentorTeamsPage';
import type { Person } from '@/types';

vi.mock('@/api/queries', () => ({
  demoMode: true,
  useSeasonPeople: vi.fn(),
  useSeasons: vi.fn(),
}));

function person(id: string, name: string, overrides: Partial<Person>): Person {
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
    ...overrides,
  };
}

describe('mentor team presentation', () => {
  it('renders stored mentor-student relationships and a separate unassigned list', () => {
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
          person('mentor-1', 'Mentor One', { roles: ['mentor'] }),
          person('assigned', 'Assigned Student', {
            mentorshipMentorId: 'mentor-1',
          }),
          person('unassigned', 'Unassigned Student', { status: 'unassigned' }),
        ],
      },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSeasonPeople>);

    render(
      <MemoryRouter initialEntries={['/seasons/season-one/mentor-teams']}>
        <Routes>
          <Route
            path="/seasons/:slug/mentor-teams"
            element={<MentorTeamsPage />}
          />
        </Routes>
      </MemoryRouter>,
    );

    expect(
      screen.getByText('1 student needs a mentor assignment.'),
    ).toBeVisible();
    const mentorTeam = screen.getByRole('article');
    expect(within(mentorTeam).getByText('Mentor One')).toBeVisible();
    expect(within(mentorTeam).getByText('Assigned Student')).toBeVisible();
    expect(
      within(mentorTeam).queryByText('Unassigned Student'),
    ).not.toBeInTheDocument();

    const unassignedHeading = screen.getByRole('heading', {
      name: 'Unassigned students',
    });
    const unassignedSection = unassignedHeading.closest('section');
    expect(unassignedSection).not.toBeNull();
    expect(
      within(unassignedSection!).getByText('Unassigned Student'),
    ).toBeVisible();
    expect(
      within(unassignedSection!).queryByText('Assigned Student'),
    ).not.toBeInTheDocument();
  });
});
