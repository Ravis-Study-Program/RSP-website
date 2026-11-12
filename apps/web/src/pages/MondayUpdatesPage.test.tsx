import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

import { defaultMondayUpdateRange } from '@/activityDates';
import type { UseQueryResult } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import type {
  MondayUpdate,
  MondayUpdateActivity,
  MondayUpdateMark,
  MondayUpdateMarkRecord,
  MondayMentorReport,
} from '@/api/generated/models';
import {
  useCurrentUser,
  useMondayMentorReports,
  useMondayUpdates,
  useMondayUpdateRangeActivity,
  useMondayUpdateUnlocks,
  useSeasonMentorships,
  useSeasonPeople,
  useSeasons,
  useSeasonWeeks,
} from '@/api/queries';
import { MondayUpdatesPage } from '@/pages/MondayUpdatesPage';
import type { CurrentUser, Person } from '@/types';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));

vi.mock('@/api/queries', () => ({
  demoMode: false,
  useCurrentUser: vi.fn(),
  useMondayMentorReports: vi.fn(),
  useMondayUpdates: vi.fn(),
  useMondayUpdateRangeActivity: vi.fn(),
  useMondayUpdateUnlocks: vi.fn(),
  useSeasonMentorships: vi.fn(),
  useSeasonPeople: vi.fn(),
  useSeasons: vi.fn(),
  useSeasonWeeks: vi.fn(),
  useWeeklyHours: () => 30,
  useWeeklyTargets: () => ({ hours: 30, easy: 3, medium: 5, hard: 2 }),
}));

vi.mock('@/components/RichTextEditor', () => ({
  RichTextEditor: ({
    id,
    label,
    value,
    onChange,
    error,
  }: {
    id: string;
    label: string;
    value: string;
    onChange: (value: string) => void;
    error?: string;
  }) => (
    <div>
      <label htmlFor={id}>{label}</label>
      <textarea
        id={id}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
      {error ? <p role="alert">{error}</p> : null}
    </div>
  ),
}));

// Tests run on a Monday, Adelaide time, so last week's update is due today.
vi.useFakeTimers({ toFake: ['Date'] });
vi.setSystemTime(new Date('2026-11-09T02:00:00Z'));

const range = defaultMondayUpdateRange();
const season = {
  id: 'season-1',
  slug: 'season-one',
  name: 'Season One',
  status: 'open',
};
const week = {
  id: 'week-1',
  number: 1,
  seasonId: season.id,
  startAt: `${range.startDate}T00:00:00+10:30`,
  endAt: `${range.endDate}T23:59:59+10:30`,
  resourcesHtml: '',
};
const studentIdentity = {
  id: 'student-1',
  name: 'Amelia Chen',
  slug: 'amelia-chen',
};

function queryResult<T>(data: unknown) {
  return {
    data,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  } as T;
}

function userWith(overrides: Partial<CurrentUser>): CurrentUser {
  return {
    id: 'user-1',
    name: 'User One',
    slug: 'user-one',
    avatarUrl: null,
    email: 'user@example.test',
    timezone: 'Australia/Adelaide',
    timezoneConfigured: true,
    emailVerified: true,
    mfaVerified: false,
    accountState: 'active',
    globalRoles: [],
    seasonRoles: [],
    alumni: false,
    attemptCount: 0,
    mockInterviewCount: 0,
    ...overrides,
  };
}

function person(id: string, name: string, slug: string): Person {
  return {
    id,
    name,
    slug,
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
  };
}

function markRecord(
  id: string,
  mentorId: string,
  mentorName: string,
  mark: MondayUpdateMark,
  commentHtml: string,
): MondayUpdateMarkRecord {
  return {
    id,
    mentor: { id: mentorId, slug: mentorId, name: mentorName, avatarUrl: null },
    mark,
    commentHtml,
    updatedAt: '2026-10-06T03:00:00Z',
  };
}

function mondayUpdate(
  id: string,
  overrides: Partial<MondayUpdate> = {},
): MondayUpdate {
  return {
    id,
    seasonId: 'season-1',
    student: { ...studentIdentity, avatarUrl: null },
    startDate: range.startDate,
    endDate: range.endDate,
    contentHtml: `<p>Update ${id}</p>`,
    hours: 30,
    createdAt: '2026-10-05T02:00:00Z',
    editedAt: null,
    marks: [],
    activity: {
      attempts: [
        {
          id: `attempt-${id}`,
          problemId: 'problem-1',
          problem: 'Two Sum',
          problemUrl: 'https://leetcode.com/problems/two-sum/',
          difficulty: 'easy',
          outcome: 'solved',
          minutes: 25,
          attemptedAt: '2026-10-06T02:00:00Z',
        },
      ],
      mocks: [
        {
          id: `mock-${id}`,
          occurredAt: '2026-10-07T02:00:00Z',
          durationMinutes: 60,
        },
      ],
    },
    ...overrides,
  };
}

function renderPage() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={['/seasons/season-one/monday-updates']}>
        <Routes>
          <Route
            path="/seasons/:slug/monday-updates"
            element={<MondayUpdatesPage />}
          />
          <Route path="/forbidden" element={<p>Forbidden page</p>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.mocked(apiRequest).mockReset();
  vi.mocked(apiRequest).mockResolvedValue({});
  vi.mocked(useSeasons).mockReturnValue(
    queryResult<ReturnType<typeof useSeasons>>({ items: [season] }),
  );
  vi.mocked(useSeasonWeeks).mockReturnValue(
    queryResult<ReturnType<typeof useSeasonWeeks>>({ items: [week] }),
  );
  vi.mocked(useSeasonMentorships).mockReturnValue(
    queryResult<ReturnType<typeof useSeasonMentorships>>({ items: [] }),
  );
  vi.mocked(useSeasonPeople).mockReturnValue(
    queryResult<ReturnType<typeof useSeasonPeople>>({ items: [] }),
  );
  vi.mocked(useMondayUpdates).mockReturnValue(
    queryResult<ReturnType<typeof useMondayUpdates>>({ items: [] }),
  );
  vi.mocked(useMondayMentorReports).mockReturnValue(
    queryResult<ReturnType<typeof useMondayMentorReports>>([]),
  );
  vi.mocked(useMondayUpdateRangeActivity).mockReturnValue(
    queryResult<UseQueryResult<MondayUpdateActivity>>({
      attempts: [],
      mocks: [],
    }),
  );
  vi.mocked(useMondayUpdateUnlocks).mockReturnValue(
    queryResult<ReturnType<typeof useMondayUpdateUnlocks>>([]),
  );
});

/** A mentor's approved unlock of the January 2025 week for the student. */
function approvedUnlock() {
  vi.mocked(useMondayUpdateUnlocks).mockReturnValue(
    queryResult<ReturnType<typeof useMondayUpdateUnlocks>>([
      {
        id: 'unlock-1',
        seasonId: 'season-1',
        student: { ...studentIdentity, avatarUrl: null },
        startDate: '2025-01-06',
        endDate: '2025-01-12',
        reason: 'I was sick.',
        state: 'approved',
        requestedAt: '2025-01-20T02:00:00Z',
        decidedAt: '2025-01-20T03:00:00Z',
        decidedBy: null,
      },
    ]),
  );
}

describe('student Monday Updates', () => {
  it('lists each week, posts the next one from the header and never renders marks', async () => {
    vi.mocked(useCurrentUser).mockReturnValue(
      queryResult<ReturnType<typeof useCurrentUser>>(
        userWith({
          id: 'student-1',
          seasonRoles: [
            {
              seasonId: 'season-1',
              seasonSlug: 'season-one',
              role: 'student',
              state: 'active',
            },
          ],
        }),
      ),
    );
    vi.mocked(useMondayUpdates).mockReturnValue(
      queryResult<ReturnType<typeof useMondayUpdates>>({
        items: [
          mondayUpdate('update-1', {
            startDate: '2025-01-06',
            endDate: '2025-01-12',
            createdAt: '2025-01-13T02:00:00Z',
            marks: [
              markRecord(
                'mark-1',
                'mentor-1',
                'Mentor One',
                'on_track',
                '<p>Good week.</p>',
              ),
            ],
          }),
        ],
      }),
    );
    vi.mocked(useMondayUpdateRangeActivity).mockReturnValue(
      queryResult<UseQueryResult<MondayUpdateActivity>>({
        attempts: [
          { ...mondayUpdate('range').activity.attempts[0], id: 'r-1' },
          {
            ...mondayUpdate('range').activity.attempts[0],
            id: 'r-2',
            difficulty: 'hard',
            outcome: 'solved',
          },
          {
            ...mondayUpdate('range').activity.attempts[0],
            id: 'r-3',
            difficulty: 'medium',
            outcome: 'unsolved',
          },
        ],
        mocks: [
          {
            id: 'm-1',
            occurredAt: '2026-10-07T02:00:00Z',
            durationMinutes: 45,
          },
          {
            id: 'm-2',
            occurredAt: '2026-10-08T02:00:00Z',
            durationMinutes: 60,
          },
        ],
      }),
    );

    // Week 1 has its update; this week (Week 2) is still to write.
    vi.mocked(useSeasonWeeks).mockReturnValue(
      queryResult<ReturnType<typeof useSeasonWeeks>>({
        items: [
          {
            ...week,
            id: 'earlier',
            number: 1,
            startAt: '2025-01-06T00:00:00+10:30',
            endAt: '2025-01-12T23:59:59+10:30',
          },
          { ...week, number: 2 },
        ],
      }),
    );

    renderPage();

    // Students get the table, not a week picker.
    expect(
      screen.queryByRole('combobox', { name: 'Program week' }),
    ).not.toBeInTheDocument();
    const table = screen.getByRole('table');
    const [, weekRow, earlierRow] = within(table).getAllByRole('row');
    expect(within(weekRow).getByText('Week 2')).toBeInTheDocument();
    expect(within(weekRow).getByText('Not submitted')).toBeInTheDocument();
    expect(within(earlierRow).getByText('Submitted')).toBeInTheDocument();

    // A posted update expands to the same summary mentors see, not the
    // attempt list.
    expect(
      within(table).queryByText('Update update-1'),
    ).not.toBeInTheDocument();
    fireEvent.click(
      within(earlierRow).getByRole('button', {
        name: 'Show update for Week 1',
      }),
    );
    expect(
      await within(table).findByText('Update update-1'),
    ).toBeInTheDocument();
    // Hours and activity stay in the row; the details hold only the writing.
    expect(within(earlierRow).getByText('30')).toBeInTheDocument();
    expect(within(earlierRow).getByLabelText('1 Easy')).toBeInTheDocument();
    expect(within(table).queryAllByRole('definition')).toHaveLength(0);
    expect(screen.queryByText('Two Sum')).not.toBeInTheDocument();

    // ...but marks and comments are never part of the student experience.
    expect(screen.queryByText('On track')).not.toBeInTheDocument();
    expect(screen.queryByText('Good week.')).not.toBeInTheDocument();
    expect(screen.queryByText('Mentor One')).not.toBeInTheDocument();

    // The header writes the latest week without an update, in a dialog.
    fireEvent.click(
      screen.getByRole('button', { name: 'Write Week 2 Update' }),
    );
    const dialog = await screen.findByRole('dialog');
    // The week's own LeetCode and mock activity is pulled live.
    expect(vi.mocked(useMondayUpdateRangeActivity)).toHaveBeenLastCalledWith(
      true,
      'season-1',
      range.startDate,
      range.endDate,
    );
    const rangeActivity = within(dialog).getByRole('region', {
      name: 'Activity this week',
    });
    const [solved, mocks] = within(rangeActivity).getAllByRole('definition');
    expect(solved).toHaveTextContent(/^2 solved1Easy0Medium1Hard$/);
    expect(mocks).toHaveTextContent(/^2 interviews$/);

    fireEvent.change(within(dialog).getByLabelText('Your Update'), {
      target: { value: '<p>Intervals are clearer now.</p>' },
    });
    // Hours default to 0; an off-step value is blocked before any request.
    const hoursInput = within(dialog).getByLabelText(
      'Hours Spent (Expected: ~30h/week)',
    );
    expect(hoursInput).toHaveValue(0);
    fireEvent.change(hoursInput, { target: { value: '7.3' } });
    fireEvent.click(
      within(dialog).getByRole('button', { name: 'Post Update' }),
    );
    expect(
      within(dialog).getByText(
        'Enter hours between 0 and 168 in steps of 0.5.',
      ),
    ).toBeInTheDocument();
    expect(apiRequest).not.toHaveBeenCalled();

    fireEvent.change(hoursInput, { target: { value: '12.5' } });
    fireEvent.click(
      within(dialog).getByRole('button', { name: 'Post Update' }),
    );

    await waitFor(() => expect(apiRequest).toHaveBeenCalledTimes(1));
    expect(apiRequest).toHaveBeenCalledWith(
      '/seasons/season-1/monday-updates',
      {
        method: 'POST',
        body: JSON.stringify({
          startDate: range.startDate,
          endDate: range.endDate,
          contentHtml: '<p>Intervals are clearer now.</p>',
          hours: 12.5,
        }),
      },
    );
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
    );
  });
});

describe('mentor Monday Updates', () => {
  it('shows each mentee, pre-selects the existing mark and saves a change', async () => {
    vi.mocked(useCurrentUser).mockReturnValue(
      queryResult<ReturnType<typeof useCurrentUser>>(
        userWith({
          id: 'mentor-1',
          name: 'Mentor One',
          seasonRoles: [
            {
              seasonId: 'season-1',
              seasonSlug: 'season-one',
              role: 'mentor',
              state: 'active',
            },
          ],
        }),
      ),
    );
    vi.mocked(useSeasonMentorships).mockReturnValue(
      queryResult<ReturnType<typeof useSeasonMentorships>>({
        items: [
          {
            id: 'mentorship-1',
            seasonId: 'season-1',
            mentorUserId: 'mentor-1',
            studentUserId: 'student-1',
          },
          {
            id: 'mentorship-2',
            seasonId: 'season-1',
            mentorUserId: 'mentor-1',
            studentUserId: 'student-2',
          },
          {
            id: 'mentorship-3',
            seasonId: 'season-1',
            mentorUserId: 'mentor-9',
            studentUserId: 'student-3',
          },
        ],
      }),
    );
    vi.mocked(useSeasonPeople).mockReturnValue(
      queryResult<ReturnType<typeof useSeasonPeople>>({
        items: [
          person('student-1', 'Amelia Chen', 'amelia-chen'),
          person('student-2', 'Zara Khan', 'zara-khan'),
          person('student-3', 'Other Student', 'other-student'),
        ],
      }),
    );
    // Each mentee posts one update for the week.
    const first = mondayUpdate('update-1', {
      marks: [
        markRecord(
          'mark-1',
          'mentor-1',
          'Mentor One',
          'on_track',
          '<p>Good week.</p>',
        ),
      ],
    });
    const attempt = first.activity.attempts[0];
    const second = mondayUpdate('update-2', {
      student: {
        id: 'student-2',
        name: 'Zara Khan',
        slug: 'zara-khan',
        avatarUrl: null,
      },
      activity: {
        attempts: [
          {
            ...attempt,
            id: 'a-hard',
            difficulty: 'hard',
            outcome: 'solved',
          },
          {
            ...attempt,
            id: 'a-medium',
            difficulty: 'medium',
            outcome: 'unsolved',
          },
          { ...attempt, id: 'a-easy', outcome: 'solved' },
        ],
        mocks: [],
      },
    });
    vi.mocked(useMondayUpdates).mockReturnValue(
      queryResult<ReturnType<typeof useMondayUpdates>>({
        items: [first, second],
      }),
    );
    vi.mocked(useMondayUpdateUnlocks).mockReturnValue(
      queryResult<ReturnType<typeof useMondayUpdateUnlocks>>([
        {
          id: 'unlock-1',
          seasonId: 'season-1',
          student: { ...studentIdentity, avatarUrl: null },
          startDate: range.startDate,
          endDate: range.endDate,
          reason: 'I was sick.',
          state: 'approved',
          requestedAt: '2026-11-09T02:00:00Z',
          decidedAt: '2026-11-09T03:00:00Z',
          decidedBy: null,
        },
      ]),
    );

    const user = userEvent.setup();
    renderPage();

    // Every mentee is listed, and nobody else.
    expect(vi.mocked(useMondayUpdates)).toHaveBeenLastCalledWith(
      true,
      'season-1',
      {
        startDate: range.startDate,
        endDate: range.endDate,
        studentIds: ['student-1', 'student-2'],
      },
    );
    // One row per student for the week, by name; nobody else's students
    // appear, and with one week chosen there is no Week column.
    const table = screen.getByRole('table');
    const [, firstRow, secondRow] = within(table).getAllByRole('row');
    expect(within(firstRow).getByText('Amelia Chen')).toBeInTheDocument();
    expect(within(secondRow).getByText('Zara Khan')).toBeInTheDocument();
    expect(within(table).queryByText('Other Student')).not.toBeInTheDocument();
    expect(
      within(table).queryByRole('columnheader', { name: 'Week' }),
    ).not.toBeInTheDocument();
    // The week picker sits in the toolbar.
    expect(
      screen.getByRole('combobox', { name: 'Program week' }),
    ).toBeInTheDocument();

    // Mentors see solved counts by difficulty, not the attempt list, and the
    // row carries the mark.
    expect(within(secondRow).getByLabelText('1 Hard')).toBeInTheDocument();
    expect(within(firstRow).getByText('On track')).toBeInTheDocument();
    expect(within(secondRow).getByText('Not marked')).toBeInTheDocument();
    expect(screen.queryByText('Two Sum')).not.toBeInTheDocument();

    // An approved late week can be locked again from the same row menu.
    await user.click(
      within(firstRow).getByRole('button', { name: /^More actions for/ }),
    );
    expect(
      await screen.findByRole('menuitem', { name: 'Lock Update' }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole('menuitem', { name: 'Lock Update' }));
    await waitFor(() => expect(apiRequest).toHaveBeenCalledTimes(1));
    expect(apiRequest).toHaveBeenCalledWith(
      '/seasons/season-1/monday-update-unlocks/unlock-1/relock',
      { method: 'POST' },
    );
    vi.mocked(apiRequest).mockClear();

    // Expanding a row shows what the student wrote and the mentor's notes;
    // hours and activity are already in the row.
    fireEvent.click(
      within(firstRow).getByRole('button', { name: /^Show update from/ }),
    );
    expect(within(table).getByText('Update update-1')).toBeInTheDocument();
    expect(within(table).getByText('Good week.')).toBeInTheDocument();
    expect(within(table).queryAllByRole('definition')).toHaveLength(0);

    // An existing mark can be replaced from the row menu, prefilled.
    await user.click(
      within(firstRow).getByRole('button', { name: /^More actions for/ }),
    );
    await user.click(
      await screen.findByRole('menuitem', { name: 'Edit Notes' }),
    );
    const dialog = await screen.findByRole('dialog');
    const markField = within(dialog).getByRole('combobox', {
      name: `Mark for ${first.student.name}`,
    });
    const commentField = within(dialog).getByLabelText(
      `Comment for ${first.student.name}`,
    );
    expect(markField).toHaveTextContent('On track');
    expect(commentField).toHaveValue('<p>Good week.</p>');
    await user.click(markField);
    await user.click(await screen.findByRole('option', { name: 'At risk' }));
    fireEvent.change(commentField, {
      target: { value: '<p>Needs a check-in.</p>' },
    });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Save Mark' }));

    await waitFor(() => expect(apiRequest).toHaveBeenCalledTimes(1));
    expect(apiRequest).toHaveBeenCalledWith(
      '/seasons/season-1/monday-updates/update-1/mark',
      {
        method: 'PUT',
        body: JSON.stringify({
          mark: 'at_risk',
          commentHtml: '<p>Needs a check-in.</p>',
        }),
      },
    );
    // A successful save closes the dialog.
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
    );
  });
});

describe('reviewer Monday Updates', () => {
  it("lists each mentor's week and expands to their students' marks and notes", () => {
    vi.mocked(useCurrentUser).mockReturnValue(
      queryResult<ReturnType<typeof useCurrentUser>>(
        userWith({
          id: 'coordinator-1',
          seasonRoles: [
            {
              seasonId: 'season-1',
              seasonSlug: 'season-one',
              role: 'coordinator',
              state: 'active',
            },
          ],
        }),
      ),
    );
    const reports: MondayMentorReport[] = [
      {
        mentor: {
          id: 'mentor-1',
          slug: 'mentor-one',
          name: 'Mentor One',
          avatarUrl: null,
        },
        startDate: range.startDate,
        endDate: range.endDate,
        students: [
          {
            student: { ...studentIdentity, avatarUrl: null },
            studentLevel: 'intermediate',
            kickedAt: null,
            targets: { hours: 30 },
            updateId: 'update-1',
            postedAt: '2026-11-08T22:00:00Z',
            editedAt: null,
            hours: 30,
            leetcodeCounts: { easy: 2, medium: 0, hard: 1 },
            mockCount: 2,
            mark: 'doing_good',
            commentHtml: '<p>Solid progress.</p>',
          },
          {
            student: {
              id: 'student-2',
              slug: 'zara-khan',
              name: 'Zara Khan',
              avatarUrl: null,
            },
            studentLevel: 'beginner',
            kickedAt: null,
            targets: { hours: 30 },
            updateId: null,
            postedAt: null,
            editedAt: null,
            hours: null,
            leetcodeCounts: { easy: 0, medium: 0, hard: 0 },
            mockCount: 0,
            mark: 'at_risk',
            commentHtml: '<p>No update this week.</p>',
          },
        ],
      },
    ];
    vi.mocked(useMondayMentorReports).mockReturnValue(
      queryResult<ReturnType<typeof useMondayMentorReports>>(reports),
    );

    renderPage();

    // One row per mentor, for the latest ended week, with a summary.
    expect(vi.mocked(useMondayMentorReports)).toHaveBeenLastCalledWith(
      'season-1',
      expect.objectContaining({
        startDate: range.startDate,
        endDate: range.endDate,
      }),
      undefined,
    );
    const table = screen.getByRole('table');
    const row = within(table).getAllByRole('row')[1];
    expect(within(row).getByText('Mentor One')).toBeInTheDocument();
    // Both weeks are marked; one of the two students posted.
    expect(within(row).getAllByText(/^[12] of 2$/)).toHaveLength(2);
    expect(within(row).getByLabelText('1 Doing good')).toBeInTheDocument();
    expect(within(row).getByLabelText('1 At risk')).toBeInTheDocument();

    // Expanding shows each student's week and the mentor's notes only.
    fireEvent.click(
      within(row).getByRole('button', {
        name: 'Show students for Mentor One',
      }),
    );
    expect(within(table).getByText('Solid progress.')).toBeInTheDocument();
    expect(within(table).getByText('No update this week.')).toBeInTheDocument();
    // Due today, the unposted week is not yet overdue.
    expect(within(table).getByText('Not submitted')).toBeInTheDocument();
  });
});

describe('Monday update access', () => {
  it('sends an actor without a role in the season to the forbidden page', () => {
    vi.mocked(useCurrentUser).mockReturnValue(
      queryResult<ReturnType<typeof useCurrentUser>>(
        userWith({
          seasonRoles: [
            {
              seasonId: 'season-9',
              seasonSlug: 'other-season',
              role: 'student',
              state: 'active',
            },
          ],
        }),
      ),
    );

    renderPage();

    expect(screen.getByText('Forbidden page')).toBeInTheDocument();
  });

  it('gives a global manager the reviewer experience without a season role', () => {
    vi.mocked(useCurrentUser).mockReturnValue(
      queryResult<ReturnType<typeof useCurrentUser>>(
        userWith({ id: 'director-1', globalRoles: ['director'] }),
      ),
    );

    renderPage();

    expect(vi.mocked(useMondayMentorReports)).toHaveBeenLastCalledWith(
      'season-1',
      expect.objectContaining({ startDate: range.startDate }),
      undefined,
    );
    expect(
      screen.getByRole('region', { name: 'Mentor Monday updates' }),
    ).toBeInTheDocument();
  });
});

describe('fixed weekly submissions', () => {
  beforeEach(() => {
    vi.mocked(useCurrentUser).mockReturnValue(
      queryResult<ReturnType<typeof useCurrentUser>>(
        userWith({
          id: studentIdentity.id,
          seasonRoles: [
            {
              seasonId: season.id,
              seasonSlug: season.slug,
              role: 'student',
              state: 'active',
            },
          ],
        }),
      ),
    );
  });
  it('edits a posted week from its row menu instead of offering a second one', async () => {
    vi.mocked(useMondayUpdates).mockReturnValue(
      queryResult<ReturnType<typeof useMondayUpdates>>({
        items: [mondayUpdate('existing')],
      }),
    );
    const user = userEvent.setup();
    renderPage();
    expect(
      screen.queryByRole('button', { name: /^Write Week/ }),
    ).not.toBeInTheDocument();
    await user.click(
      screen.getByRole('button', { name: 'More actions for Week 1' }),
    );
    expect(
      screen.queryByRole('menuitem', { name: 'Write Update' }),
    ).not.toBeInTheDocument();
    await user.click(
      await screen.findByRole('menuitem', { name: 'Edit Update' }),
    );
    const dialog = await screen.findByRole('dialog');
    const content = within(dialog).getByLabelText('Your Update');
    expect(content).toHaveValue('<p>Update existing</p>');
    expect(
      within(dialog).getByLabelText('Hours Spent (Expected: ~30h/week)'),
    ).toHaveValue(30);
    fireEvent.change(content, { target: { value: '<p>Rewritten</p>' } });
    fireEvent.click(
      within(dialog).getByRole('button', { name: 'Save Update' }),
    );
    await waitFor(() => expect(apiRequest).toHaveBeenCalledTimes(1));
    expect(apiRequest).toHaveBeenCalledWith(
      '/seasons/season-1/monday-updates/existing',
      {
        method: 'PATCH',
        body: JSON.stringify({ contentHtml: '<p>Rewritten</p>', hours: 30 }),
      },
    );
  });
  it('shows the lock icon beside a submitted update after the week locks', () => {
    vi.setSystemTime(new Date('2026-11-10T02:00:00Z'));
    vi.mocked(useMondayUpdates).mockReturnValue(
      queryResult<ReturnType<typeof useMondayUpdates>>({
        items: [
          mondayUpdate('submitted', {
            createdAt: '2026-11-03T02:00:00Z',
          }),
        ],
      }),
    );

    try {
      renderPage();
      const row = within(screen.getByRole('table')).getAllByRole('row')[1];
      expect(within(row).getByText('Submitted')).toBeInTheDocument();
      expect(
        within(row).getByLabelText(
          'Locked. Request an unlock from this row’s menu.',
        ),
      ).toBeInTheDocument();
    } finally {
      vi.setSystemTime(new Date('2026-11-09T02:00:00Z'));
    }
  });
  it('warns that editing an unlocked late update marks it late', async () => {
    approvedUnlock();
    vi.mocked(useSeasonWeeks).mockReturnValue(
      queryResult<ReturnType<typeof useSeasonWeeks>>({
        items: [
          {
            ...week,
            id: 'earlier',
            number: 1,
            startAt: '2025-01-06T00:00:00+10:30',
            endAt: '2025-01-12T23:59:59+10:30',
          },
          { ...week, number: 2 },
        ],
      }),
    );
    vi.mocked(useMondayUpdates).mockReturnValue(
      queryResult<ReturnType<typeof useMondayUpdates>>({
        items: [
          mondayUpdate('old', {
            startDate: '2025-01-06',
            endDate: '2025-01-12',
            createdAt: '2025-01-13T02:00:00Z',
          }),
        ],
      }),
    );
    const user = userEvent.setup();
    renderPage();
    await user.click(
      screen.getByRole('button', { name: 'More actions for Week 1' }),
    );
    // Week 1 was posted on time, so it reads Submitted until edited late.
    expect(screen.getAllByText('Submitted').length).toBeGreaterThan(0);
    await user.click(
      await screen.findByRole('menuitem', { name: 'Edit Update' }),
    );
    expect(await screen.findByRole('dialog')).toHaveTextContent(
      'Saving this edit marks it late.',
    );
  });
  it('locks a missed week until its mentor unlocks it, then takes a late update', async () => {
    vi.mocked(useSeasonWeeks).mockReturnValue(
      queryResult<ReturnType<typeof useSeasonWeeks>>({
        items: [
          {
            ...week,
            id: 'late',
            number: 1,
            startAt: '2025-01-06T00:00:00+10:30',
            endAt: '2025-01-12T23:59:59+10:30',
          },
          { ...week, number: 2 },
          {
            ...week,
            id: 'future',
            number: 3,
            startAt: '2099-01-05T00:00:00+10:30',
            endAt: '2099-01-11T23:59:59+10:30',
          },
        ],
      }),
    );
    const user = userEvent.setup();
    renderPage();
    const table = screen.getByRole('table');
    expect(within(table).getByText('Week 2')).toBeInTheDocument();
    expect(within(table).queryByText('Week 3')).not.toBeInTheDocument();
    // The latest week is still due; the earlier missed one is overdue.
    const [, latestRow, missedRow] = within(table).getAllByRole('row');
    expect(within(latestRow).getByText('Not submitted')).toBeInTheDocument();
    expect(within(missedRow).getByText('Overdue')).toBeInTheDocument();
    // Locked: the student asks for an unlock, with a reason.
    await user.click(
      screen.getByRole('button', { name: 'More actions for Week 1' }),
    );
    expect(
      screen.queryByRole('menuitem', { name: 'Write Update' }),
    ).not.toBeInTheDocument();
    await user.click(
      await screen.findByRole('menuitem', { name: 'Request Unlock' }),
    );
    const request = await screen.findByRole('dialog');
    fireEvent.change(within(request).getByLabelText('Reason'), {
      target: { value: 'I was sick.' },
    });
    fireEvent.click(
      within(request).getByRole('button', { name: 'Send Request' }),
    );
    await waitFor(() =>
      expect(apiRequest).toHaveBeenCalledWith(
        '/seasons/season-1/monday-update-unlocks',
        {
          method: 'POST',
          body: JSON.stringify({
            startDate: '2025-01-06',
            endDate: '2025-01-12',
            reason: 'I was sick.',
          }),
        },
      ),
    );
    vi.mocked(apiRequest).mockClear();
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
    );

    // Once the mentor unlocks it, the late week can be written.
    approvedUnlock();
    cleanup();
    renderPage();
    await user.click(
      screen.getByRole('button', { name: 'More actions for Week 1' }),
    );
    await user.click(
      await screen.findByRole('menuitem', { name: 'Write Update' }),
    );
    const dialog = await screen.findByRole('dialog');
    // Writing is optional: an update can be just its hours.
    fireEvent.click(
      within(dialog).getByRole('button', { name: 'Post Update' }),
    );
    await waitFor(() => expect(apiRequest).toHaveBeenCalledTimes(1));
    const options = vi.mocked(apiRequest).mock.calls[0][1];
    expect(JSON.parse(options?.body as string)).toMatchObject({
      startDate: '2025-01-06',
      endDate: '2025-01-12',
      contentHtml: '',
    });
  });
  it('keeps closed seasons read-only', () => {
    vi.mocked(useSeasons).mockReturnValue(
      queryResult<ReturnType<typeof useSeasons>>({
        items: [{ ...season, status: 'closed' }],
      }),
    );
    renderPage();
    expect(screen.getByText('Updates are read-only.')).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: /^Write Week/ }),
    ).not.toBeInTheDocument();
  });
});
