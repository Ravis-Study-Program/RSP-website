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
import { apiRequest } from '@/api/client';
import type {
  MondayUpdate,
  MondayUpdateMark,
  MondayUpdateMarkRecord,
  MondayUpdateReviewRow,
} from '@/api/generated/models';
import {
  useCurrentUser,
  useMondayUpdateReview,
  useMondayUpdates,
  useSeasonMentorships,
  useSeasonPeople,
  useSeasons,
} from '@/api/queries';
import { MondayUpdatesPage } from '@/pages/MondayUpdatesPage';
import type { CurrentUser, Person } from '@/types';
import { formatDate } from '@/utils';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));

vi.mock('@/api/queries', () => ({
  demoMode: false,
  useCurrentUser: vi.fn(),
  useMondayUpdateReview: vi.fn(),
  useMondayUpdates: vi.fn(),
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

/** The date a react-aria date picker currently holds, read from its field. */
function pickerValue(label: string) {
  return screen
    .getAllByLabelText(label)
    .map((field) => field.querySelector('input[type="text"]'))
    .find((input): input is HTMLInputElement => input !== null)?.value;
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

    renderPage();

    // The default Monday-to-Sunday range is prefilled into both pickers.
    expect(pickerValue('Start date')).toBe(range.startDate);
    expect(pickerValue('End date')).toBe(range.endDate);

    // The student's own update and its activity are rendered...
    expect(screen.getByText('Update update-1')).toBeInTheDocument();
    expect(screen.getByText('Attempts (1)')).toBeInTheDocument();
    expect(screen.getByText('Mock interviews (1)')).toBeInTheDocument();
    expect(screen.getByText('Two Sum')).toBeInTheDocument();

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
    // Hours default to 30; an off-step value is blocked before any request.
    const hoursInput = screen.getByLabelText('Hours worked');
    expect(hoursInput).toHaveValue(30);
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
    const second = mondayUpdate('update-2', {
      startDate: range.endDate,
      endDate: range.endDate,
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

    const firstLabel = `${formatDate(first.startDate)} – ${formatDate(first.endDate)}`;
    const secondLabel = `${formatDate(second.startDate)} – ${formatDate(second.endDate)}`;
    expect(
      screen.getByRole('combobox', { name: `Mark for ${firstLabel}` }),
    ).toHaveTextContent('On track');
    expect(screen.getByLabelText(`Comment for ${firstLabel}`)).toHaveValue(
      '<p>Good week.</p>',
    );
    expect(
      screen.getByRole('combobox', { name: `Mark for ${secondLabel}` }),
    ).toHaveTextContent('No mark yet');
    const saveButtons = screen.getAllByRole('button', { name: 'Save mark' });
    expect(saveButtons).toHaveLength(2);
    expect(saveButtons[1]).toBeDisabled();

    // An existing mark can be replaced.
    await user.click(
      screen.getByRole('combobox', { name: `Mark for ${firstLabel}` }),
    );
    await user.click(await screen.findByRole('option', { name: 'At risk' }));
    fireEvent.change(screen.getByLabelText(`Comment for ${firstLabel}`), {
      target: { value: '<p>Needs a check-in.</p>' },
    });
    fireEvent.click(saveButtons[0]);

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
    expect(within(table).getAllByText('Amelia Chen').length).toBeGreaterThan(0);
    expect(within(table).getByText('2 / 0 / 1')).toBeInTheDocument();
    expect(within(table).getByText('easy / medium / hard')).toBeInTheDocument();
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
    expect(within(table).getByText('Doing good')).toBeInTheDocument();
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
