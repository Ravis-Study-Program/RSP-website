import { fireEvent, render, screen, within } from '@testing-library/react';
import type { UseQueryResult } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

import type {
  MondayUpdate,
  MondayUpdateMarkRecord,
  MondayUpdatePage,
} from '@/api/generated/models';
import {
  useCurrentUser,
  useMondayUpdates,
  useSeasonPeople,
  useSeasons,
} from '@/api/queries';
import { MenteesPage } from '@/pages/MenteesPage';
import type { CurrentUser, Person } from '@/types';
import { formatDate } from '@/utils';

vi.mock('@/api/queries', () => ({
  demoMode: true,
  useCurrentUser: vi.fn(),
  useMondayUpdates: vi.fn(),
  useSeasonPeople: vi.fn(),
  useSeasons: vi.fn(),
}));

const season = {
  id: 'season-1',
  slug: 'season-one',
  name: 'Season One',
  startsAt: '2026-08-02T14:30:00Z',
  endsAt: '2026-12-20T13:30:00Z',
};

beforeEach(() => {
  vi.mocked(useMondayUpdates).mockReturnValue({
    data: { items: [] },
    isLoading: false,
    isError: false,
  } as unknown as UseQueryResult<MondayUpdatePage>);
});

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
        items: [season],
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
            previousMentorIds: [mentor.id],
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

    const table = screen.getByRole('region', { name: 'Assigned students' });
    expect(
      within(table).getAllByText('Assigned Student').length,
    ).toBeGreaterThan(0);
    expect(screen.queryByText('Another Team')).not.toBeInTheDocument();
    expect(screen.queryByText('Unassigned Student')).not.toBeInTheDocument();
    expect(screen.queryByText('Completed Student')).not.toBeInTheDocument();
    fireEvent.click(
      screen.getByRole('checkbox', { name: 'Include previous students' }),
    );
    expect(screen.getAllByText('Completed Student').length).toBeGreaterThan(0);
  });
});

function update(
  id: string,
  studentId: string,
  startDate: string,
  endDate: string,
  marks: MondayUpdateMarkRecord[] = [],
): MondayUpdate {
  return {
    id,
    seasonId: season.id,
    student: {
      id: studentId,
      slug: studentId,
      name: studentId,
      avatarUrl: null,
    },
    startDate,
    endDate,
    contentHtml: '<p>Week</p>',
    hours: 20,
    createdAt: `${endDate}T02:00:00Z`,
    marks,
    activity: { attempts: [], mocks: [] },
  };
}

function mark(
  id: string,
  value: MondayUpdateMarkRecord['mark'],
  updatedAt: string,
): MondayUpdateMarkRecord {
  return {
    id,
    mentor: {
      id: 'mentor-1',
      slug: 'mentor-1',
      name: 'Mentor One',
      avatarUrl: null,
    },
    mark: value,
    commentHtml: '',
    updatedAt,
  };
}

function renderWith(
  user: CurrentUser,
  people: Person[],
  updates: MondayUpdate[],
) {
  vi.mocked(useCurrentUser).mockReturnValue({
    data: user,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  } as unknown as UseQueryResult<CurrentUser>);
  vi.mocked(useSeasons).mockReturnValue({
    data: { items: [season] },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useSeasons>);
  vi.mocked(useSeasonPeople).mockReturnValue({
    data: { items: people },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useSeasonPeople>);
  vi.mocked(useMondayUpdates).mockReturnValue({
    data: { items: updates },
    isLoading: false,
    isError: false,
  } as unknown as UseQueryResult<MondayUpdatePage>);
  render(
    <MemoryRouter initialEntries={['/seasons/season-one/mentees']}>
      <Routes>
        <Route path="/seasons/:slug/mentees" element={<MenteesPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('latest Monday mark column', () => {
  const people = [
    student('marked', 'Marked Student', { mentorshipMentorId: mentor.id }),
    student('unmarked', 'Unmarked Student', { mentorshipMentorId: mentor.id }),
    student('silent', 'Silent Student', { mentorshipMentorId: mentor.id }),
  ];

  it("shows each mentee's latest update mark across the whole season", () => {
    renderWith(mentor, people, [
      // An older update with a mark is superseded by the newer one.
      update('old', 'marked', '2026-09-07', '2026-09-13', [
        mark('k-old', 'way_ahead', '2026-09-14T00:00:00Z'),
      ]),
      // Two marks on the latest update: the most recently saved wins.
      update('new', 'marked', '2026-10-05', '2026-10-11', [
        mark('k-1', 'doing_good', '2026-10-12T00:00:00Z'),
        mark('k-2', 'at_risk', '2026-10-13T00:00:00Z'),
      ]),
      update('pending', 'unmarked', '2026-10-12', '2026-10-18'),
    ]);

    // The whole season is read for the mentor's current mentees only.
    expect(vi.mocked(useMondayUpdates)).toHaveBeenLastCalledWith(
      true,
      season.id,
      {
        startDate: '2026-08-03',
        endDate: '2026-12-21',
        studentIds: ['marked', 'unmarked', 'silent'],
      },
    );
    const table = screen.getByRole('table');
    const row = (name: string) =>
      within(table)
        .getAllByRole('row')
        .find((item) => within(item).queryByText(name))!;
    expect(
      within(table).getByRole('columnheader', { name: /Latest Monday mark/ }),
    ).toBeInTheDocument();
    expect(
      within(row('Marked Student')).getByText('At risk'),
    ).toBeInTheDocument();
    expect(
      within(row('Marked Student')).queryByText('Way ahead'),
    ).not.toBeInTheDocument();
    expect(
      within(row('Marked Student')).getByText(
        `${formatDate('2026-10-05')} – ${formatDate('2026-10-11')}`,
      ),
    ).toBeInTheDocument();
    expect(
      within(row('Unmarked Student')).getByText('Not marked'),
    ).toBeInTheDocument();
    expect(
      within(row('Silent Student')).getByText('No updates yet'),
    ).toBeInTheDocument();
  });

  it('is hidden for coordinators, who have no marks of their own', () => {
    renderWith(
      {
        ...mentor,
        seasonRoles: [
          {
            seasonId: season.id,
            seasonSlug: season.slug,
            role: 'coordinator',
            state: 'active',
          },
        ],
      } as CurrentUser,
      people,
      [],
    );
    expect(
      screen.queryByRole('columnheader', { name: /Latest Monday mark/ }),
    ).not.toBeInTheDocument();
    expect(vi.mocked(useMondayUpdates)).toHaveBeenLastCalledWith(
      false,
      season.id,
      expect.anything(),
    );
  });
});
