import { format, parseISO } from 'date-fns';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import {
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
  MondayUpdateReviewRow,
} from '@/api/generated/models';
import {
  useCurrentUser,
  useMondayUpdateReview,
  useMondayUpdates,
  useMondayUpdateRangeActivity,
  useSeasonMentorships,
  useSeasonPeople,
  useSeasons,
} from '@/api/queries';
import { MondayUpdatesPage } from '@/pages/MondayUpdatesPage';
import type { CurrentUser, Person } from '@/types';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));

vi.mock('@/api/queries', () => ({
  demoMode: false,
  useCurrentUser: vi.fn(),
  useMondayUpdateReview: vi.fn(),
  useMondayUpdates: vi.fn(),
  useMondayUpdateRangeActivity: vi.fn(),
  useSeasonMentorships: vi.fn(),
  useSeasonPeople: vi.fn(),
  useSeasons: vi.fn(),
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

const range = defaultMondayUpdateRange();
const season = { id: 'season-1', slug: 'season-one', name: 'Season One' };
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
    marks: [],
    activity: {
      attempts: [
        {
          id: `attempt-${id}`,
          problemId: 'problem-1',
          problem: 'Two Sum',
          problemUrl: 'https://leetcode.com/problems/two-sum/',
          difficulty: 'easy',
          outcome: 'independently_solved',
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
  vi.mocked(useSeasonMentorships).mockReturnValue(
    queryResult<ReturnType<typeof useSeasonMentorships>>({ items: [] }),
  );
  vi.mocked(useSeasonPeople).mockReturnValue(
    queryResult<ReturnType<typeof useSeasonPeople>>({ items: [] }),
  );
  vi.mocked(useMondayUpdates).mockReturnValue(
    queryResult<ReturnType<typeof useMondayUpdates>>({ items: [] }),
  );
  vi.mocked(useMondayUpdateReview).mockReturnValue(
    queryResult<ReturnType<typeof useMondayUpdateReview>>({ items: [] }),
  );
  vi.mocked(useMondayUpdateRangeActivity).mockReturnValue(
    queryResult<UseQueryResult<MondayUpdateActivity>>({
      attempts: [],
      mocks: [],
    }),
  );
});

describe('student Monday updates', () => {
  it('posts the default range and never renders marks', async () => {
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
            outcome: 'solved_with_hints',
          },
          {
            ...mondayUpdate('range').activity.attempts[0],
            id: 'r-3',
            difficulty: 'medium',
            outcome: 'not_solved',
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

    renderPage();

    // The default Monday-to-Sunday range is prefilled into the range picker.
    expect(screen.getByLabelText('Dates')).toHaveTextContent(
      `${format(parseISO(range.startDate), 'd MMM yyyy')} – ${format(parseISO(range.endDate), 'd MMM yyyy')}`,
    );

    // The selected range's own LeetCode and mock activity is pulled live,
    // summarised exactly as mentors see it.
    expect(vi.mocked(useMondayUpdateRangeActivity)).toHaveBeenLastCalledWith(
      true,
      'season-1',
      range.startDate,
      range.endDate,
    );
    const rangeActivity = screen.getByRole('region', {
      name: 'Activity in this range',
    });
    const [solved, mocks] = within(rangeActivity).getAllByRole('definition');
    expect(solved).toHaveTextContent(/^2 solved1Easy0Medium1Hard$/);
    expect(mocks).toHaveTextContent(/^2 interviews105 min total$/);

    // The student's own update shows the same summary mentors see, not the
    // attempt list.
    const card = screen.getByRole('article');
    expect(within(card).getByText('Update update-1')).toBeInTheDocument();
    const [cardHours, cardSolved, cardMocks] =
      within(card).getAllByRole('definition');
    expect(cardHours).toHaveTextContent(/^30 hours$/);
    expect(cardSolved).toHaveTextContent(/^1 solved1Easy0Medium0Hard$/);
    expect(cardMocks).toHaveTextContent(/^1 interview60 min total$/);
    expect(screen.queryByText('Two Sum')).not.toBeInTheDocument();

    // ...but marks and comments are never part of the student experience.
    expect(screen.queryByText('On track')).not.toBeInTheDocument();
    expect(screen.queryByText('Good week.')).not.toBeInTheDocument();
    expect(screen.queryByText('Mentor One')).not.toBeInTheDocument();
    expect(
      screen.queryByRole('combobox', { name: /^Mark for/ }),
    ).not.toBeInTheDocument();

    // Empty content is blocked before any request leaves the page.
    fireEvent.click(screen.getByRole('button', { name: 'Post update' }));
    expect(apiRequest).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText('Your update'), {
      target: { value: '<p>Intervals are clearer now.</p>' },
    });
    // Hours default to 0; an off-step value is blocked before any request.
    const hoursInput = screen.getByLabelText(
      'Hours spent (Expected: ~30h/week)',
    );
    expect(hoursInput).toHaveValue(0);
    fireEvent.change(hoursInput, { target: { value: '7.3' } });
    fireEvent.click(screen.getByRole('button', { name: 'Post update' }));
    expect(
      screen.getByText('Enter hours between 0 and 168 in steps of 0.5.'),
    ).toBeInTheDocument();
    expect(apiRequest).not.toHaveBeenCalled();

    fireEvent.change(hoursInput, { target: { value: '12.5' } });
    fireEvent.click(screen.getByRole('button', { name: 'Post update' }));

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
    expect(
      await screen.findByText('Your update was posted.'),
    ).toBeInTheDocument();
  });
});

describe('mentor Monday updates', () => {
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
    const first = mondayUpdate('update-1', {
      startDate: range.startDate,
      endDate: range.startDate,
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
      startDate: range.endDate,
      endDate: range.endDate,
      activity: {
        attempts: [
          {
            ...attempt,
            id: 'a-hard',
            difficulty: 'hard',
            outcome: 'solved_with_hints',
          },
          {
            ...attempt,
            id: 'a-medium',
            difficulty: 'medium',
            outcome: 'not_solved',
          },
          { ...attempt, id: 'a-easy', outcome: 'independently_solved' },
        ],
        mocks: [],
      },
    });
    vi.mocked(useMondayUpdates).mockReturnValue(
      queryResult<ReturnType<typeof useMondayUpdates>>({
        items: [first, second],
      }),
    );

    const user = userEvent.setup();
    renderPage();

    // Every mentee is selected by default, and nobody else is.
    expect(vi.mocked(useMondayUpdates)).toHaveBeenLastCalledWith(
      true,
      'season-1',
      {
        startDate: range.startDate,
        endDate: range.endDate,
        studentIds: ['student-1', 'student-2'],
      },
    );
    expect(screen.getByRole('checkbox', { name: 'Amelia Chen' })).toBeChecked();
    expect(screen.getByRole('checkbox', { name: 'Zara Khan' })).toBeChecked();
    expect(
      screen.queryByRole('checkbox', { name: 'Other Student' }),
    ).not.toBeInTheDocument();

    // Several updates in one range are expected, not an error.
    expect(screen.getByText('2 updates in this range')).toBeInTheDocument();
    expect(screen.getByText('No update in this range.')).toBeInTheDocument();

    // Mentors see solved counts by difficulty, not the attempt list.
    const [firstHours, firstSolved, firstMocks, , secondSolved, secondMocks] =
      screen.getAllByRole('definition');
    expect(firstHours).toHaveTextContent(/^30 hours$/);
    expect(firstSolved).toHaveTextContent(/^1 solved1Easy0Medium0Hard$/);
    expect(firstMocks).toHaveTextContent(/^1 interview60 min total$/);
    expect(secondSolved).toHaveTextContent(/^2 solved1Easy0Medium1Hard$/);
    expect(secondMocks).toHaveTextContent(/^0 interviews$/);
    expect(screen.queryByText('Two Sum')).not.toBeInTheDocument();

    // The card header carries the mark; marking happens in a dialog.
    const cards = screen.getAllByRole('article');
    expect(within(cards[0]).getByText('On track')).toBeInTheDocument();
    expect(within(cards[0]).getByText('Good week.')).toBeInTheDocument();
    expect(within(cards[1]).getByText('Not marked')).toBeInTheDocument();
    expect(
      screen.queryByRole('combobox', {
        name: `Mark for ${first.student.name}`,
      }),
    ).not.toBeInTheDocument();
    expect(
      within(cards[1]).getByRole('button', { name: 'Add mark' }),
    ).toBeInTheDocument();

    // An existing mark can be replaced from its dialog, prefilled.
    await user.click(
      within(cards[0]).getByRole('button', { name: 'Edit mark' }),
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
    fireEvent.click(within(dialog).getByRole('button', { name: 'Save mark' }));

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

describe('reviewer Monday updates', () => {
  it('renders one row per student with counts and expands to updates and marks', async () => {
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
    const rows: MondayUpdateReviewRow[] = [
      {
        student: { ...studentIdentity, avatarUrl: null },
        leetcodeCounts: { easy: 2, medium: 0, hard: 1 },
        mockCount: 2,
        updateCount: 1,
        updates: [
          {
            id: 'update-1',
            startDate: range.startDate,
            endDate: range.endDate,
            contentHtml: '<p>Finished the hash map pattern.</p>',
            hours: 30,
            createdAt: '2026-10-05T02:00:00Z',
            marks: [
              markRecord(
                'mark-1',
                'mentor-1',
                'Mentor One',
                'doing_good',
                '<p>Solid progress.</p>',
              ),
            ],
          },
        ],
      },
    ];
    vi.mocked(useMondayUpdateReview).mockReturnValue(
      queryResult<ReturnType<typeof useMondayUpdateReview>>({ items: rows }),
    );

    renderPage();

    expect(vi.mocked(useMondayUpdateReview)).toHaveBeenLastCalledWith(
      'season-1',
      range.startDate,
      range.endDate,
    );
    // Reviewers get no student selector and no per-update mark control.
    expect(
      screen.queryByRole('group', { name: 'Students' }),
    ).not.toBeInTheDocument();

    const table = screen.getByRole('table');
    expect(
      within(table).getByRole('columnheader', {
        name: /LeetCode \(easy\/medium\/hard\)/,
      }),
    ).toBeInTheDocument();
    const row = within(table).getAllByRole('row')[1];
    expect(within(row).getByText('Amelia Chen')).toBeInTheDocument();
    // The slug is no longer shown under the name, nor the difficulty legend.
    expect(
      within(row).queryByText(studentIdentity.slug),
    ).not.toBeInTheDocument();
    expect(
      within(row).queryByText('easy / medium / hard'),
    ).not.toBeInTheDocument();
    expect(within(row).getByText('2 / 0 / 1')).toBeInTheDocument();
    // The latest update in range shows its mark and who gave it.
    expect(within(row).getByText('Doing good')).toBeInTheDocument();
    expect(within(row).getByText(/by Mentor One/)).toBeInTheDocument();
    expect(
      within(table).queryByRole('columnheader', { name: 'Mentor' }),
    ).not.toBeInTheDocument();
    expect(
      within(table).queryByText('Finished the hash map pattern.'),
    ).not.toBeInTheDocument();

    fireEvent.click(
      within(table).getByRole('button', {
        name: 'Show details for Amelia Chen',
      }),
    );

    expect(
      await within(table).findByText('Finished the hash map pattern.'),
    ).toBeInTheDocument();
    expect(within(table).getAllByText('Doing good')).toHaveLength(2);
    expect(within(table).getByText('Mentor One')).toBeInTheDocument();
    expect(within(table).getByText('Solid progress.')).toBeInTheDocument();
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

    expect(vi.mocked(useMondayUpdateReview)).toHaveBeenLastCalledWith(
      'season-1',
      range.startDate,
      range.endDate,
    );
    expect(
      screen.getByRole('region', { name: 'Monday update review' }),
    ).toBeInTheDocument();
  });
});
