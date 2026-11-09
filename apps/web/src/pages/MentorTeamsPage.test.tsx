import userEvent from '@testing-library/user-event';
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
  it('renders stored mentor-student relationships and a separate unassigned list', async () => {
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

    expect(screen.getByText('1 student needs a mentor.')).toBeVisible();
    // Mentors are table rows with a student count; their students open
    // from the Details chevron.
    const mentorRow = screen.getByRole('row', { name: /Mentor One/ });
    expect(within(mentorRow).getByText('1')).toBeVisible();
    await userEvent.click(
      within(mentorRow).getByRole('button', {
        name: 'Show students of Mentor One',
      }),
    );
    const team = screen.getByRole('list', { name: 'Students of Mentor One' });
    expect(within(team).getByText('Assigned Student')).toBeVisible();
    expect(
      within(team).queryByText('Unassigned Student'),
    ).not.toBeInTheDocument();

    const unassignedHeading = screen.getByRole('heading', {
      name: 'Unassigned Students',
    });
    const unassignedSection = unassignedHeading.closest('section');
    expect(unassignedSection).not.toBeNull();
    expect(
      within(unassignedSection!).getByText('Unassigned Student'),
    ).toBeVisible();
    expect(
      within(unassignedSection!).queryByText('Assigned Student'),
    ).not.toBeInTheDocument();

    // An unassigned student's arrow opens the dialog with them chosen.
    await userEvent.click(
      screen.getByRole('button', {
        name: 'Assign a mentor to Unassigned Student',
      }),
    );
    const dialog = screen.getByRole('dialog', { name: 'Add Mentorship' });
    expect(
      within(dialog).getByRole('combobox', { name: 'Student' }),
    ).toHaveTextContent('Unassigned Student');
  });
});
