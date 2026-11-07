import type {
  MondayUpdate,
  MondayUpdateActivity,
  MondayUpdateAttempt,
  MondayUpdateMark,
  MondayUpdateMarkRecord,
  MondayUpdateMock,
  PersonSummary,
  MondayUpdateUnlock,
  MondayWeekMark,
} from '@/api/generated/models';
import { defaultMondayUpdateRange, programTimezone } from '@/activityDates';
import type {
  Attempt,
  CurrentUser,
  MockInterview,
  Person,
  Season,
} from '@/types';
import { calendarDateKey } from '@/utils';

export const demoUser: CurrentUser = {
  id: 'usr_01j_rsp_director',
  name: 'Avery Example',
  slug: 'avery-example',
  avatarUrl: null,
  email: 'avery@example.test',
  timezone: 'Australia/Adelaide',
  timezoneConfigured: true,
  emailVerified: true,
  mfaVerified: true,
  accountState: 'active',
  globalRoles: ['director'],
  seasonRoles: [
    {
      seasonId: 'season_2026_s2',
      seasonSlug: 'summer-2025-26',
      role: 'coordinator',
      state: 'active',
    },
  ],
  alumni: true,
  attemptCount: 42,
  mockInterviewCount: 15,
};

export const seasons: Season[] = [
  {
    id: 'season_2026_s2',
    slug: 'summer-2025-26',
    name: 'Summer 2025/26',
    status: 'open',
    startsAt: '2025-11-30T13:30:00Z',
    endsAt: '2026-02-28T13:29:59Z',
    location: 'Adelaide University',
    imageUrl: '/assets/rsp-logo.png',
    resourcesUrl: 'https://example.test/resources/summer-2025-26',
    memberCount: 46,
    weekCount: 12,
    summary:
      'The current program for students building interview confidence through practice and mentoring.',
  },
  {
    id: 'season_2026_s1',
    slug: 'summer-2024-25',
    name: 'Summer 2024/25',
    status: 'closed',
    startsAt: '2024-11-30T13:30:00Z',
    endsAt: '2025-02-28T13:29:59Z',
    location: 'Adelaide University',
    imageUrl: '/assets/rsp-logo.png',
    resourcesUrl: 'https://example.test/resources/summer-2024-25',
    memberCount: 39,
    weekCount: 12,
    summary:
      'Completed program history, available read-only to participants and alumni.',
  },
];

export const attempts: Attempt[] = [
  {
    id: 'attempt_7',
    problemId: 'lc_102',
    problem: 'Binary Tree Level Order Traversal',
    difficulty: 'Medium',
    category: 'Trees',
    outcome: 'solved',
    reviewed: true,
    minutes: 31,
    attemptedAt: '2025-12-16T09:15:00Z',
    notes: '<p>Used a queue and checked the empty tree first.</p>',
  },
  {
    id: 'attempt_6',
    problemId: 'lc_242',
    problem: 'Valid Anagram',
    difficulty: 'Easy',
    category: 'Hash maps',
    outcome: 'unsolved',
    reviewed: true,
    minutes: 18,
    attemptedAt: '2025-12-13T05:40:00Z',
    notes: '<p>Forgot to consider Unicode constraints.</p>',
  },
  {
    id: 'attempt_5',
    problemId: 'lc_56',
    problem: 'Merge Intervals',
    difficulty: 'Medium',
    category: 'Intervals',
    outcome: 'unsolved',
    reviewed: false,
    minutes: 47,
    attemptedAt: '2025-12-09T08:05:00Z',
    notes: '<p>Revisit sorting invariant.</p>',
  },
  {
    id: 'attempt_4',
    problemId: 'lc_1',
    problem: 'Two Sum',
    difficulty: 'Easy',
    category: 'Hash maps',
    outcome: 'solved',
    reviewed: false,
    minutes: 9,
    attemptedAt: '2025-12-04T04:20:00Z',
    notes: '<p>Explained the complement invariant aloud.</p>',
  },
  {
    id: 'attempt_migrated',
    problemId: 'lc_70',
    problem: 'Climbing Stairs',
    difficulty: 'Easy',
    category: 'Dynamic programming',
    outcome: 'solved',
    reviewed: false,
    minutes: null,
    attemptedAt: '2025-10-18T03:00:00Z',
    notes: '',
  },
];

export const currentPerson: Person = {
  id: demoUser.id,
  name: demoUser.name,
  slug: demoUser.slug,
  initials: 'MM',
  avatarUrl: null,
  roles: ['director', 'graduate'],
  season: 'Summer 2025/26',
  status: 'active',
  attempts: 42,
  interviews: 15,
  email: demoUser.email,
  lastActiveAt: '2026-08-13T10:30:00Z',
};

export const people: Person[] = [
  {
    id: 'person_amelia',
    name: 'Amelia Chen',
    slug: 'amelia-chen',
    initials: 'AC',
    avatarUrl: null,
    roles: ['student'],
    season: 'Summer 2025/26',
    status: 'active',
    attempts: 14,
    interviews: 2,
    email: 'amelia@example.test',
    lastActiveAt: '2026-08-12T11:10:00Z',
  },
  {
    id: 'person_noah',
    name: 'Noah Williams',
    slug: 'noah-williams',
    initials: 'NW',
    avatarUrl: null,
    roles: ['mentor', 'graduate'],
    season: 'Summer 2025/26',
    status: 'active',
    attempts: 67,
    interviews: 12,
    email: 'noah@example.test',
    lastActiveAt: '2026-08-13T02:35:00Z',
  },
  {
    id: 'person_zara',
    name: 'Zara Ahmed',
    slug: 'zara-ahmed',
    initials: 'ZA',
    avatarUrl: null,
    roles: ['student'],
    season: 'Summer 2025/26',
    status: 'unassigned',
    attempts: 8,
    interviews: 1,
    email: 'zara@example.test',
    lastActiveAt: '2026-08-10T07:45:00Z',
  },
  {
    id: 'person_liam',
    name: 'Liam Foster',
    slug: 'liam-foster',
    initials: 'LF',
    avatarUrl: null,
    roles: ['coordinator'],
    season: 'Summer 2025/26',
    status: 'active',
    attempts: 31,
    interviews: 18,
    email: 'liam@example.test',
    lastActiveAt: '2026-08-12T23:20:00Z',
  },
  {
    id: 'person_mia',
    name: 'Mia Thompson',
    slug: 'mia-thompson',
    initials: 'MT',
    avatarUrl: null,
    roles: ['graduate'],
    season: 'Summer 2024/25',
    status: 'completed',
    attempts: 52,
    interviews: 9,
    lastActiveAt: '2026-08-09T01:30:00Z',
  },
  currentPerson,
];

// Every demo interview takes one of the shapes real interviews use: a
// behavioural round then two LeetCode rounds, a single behavioural round, a
// single custom round, or three custom rounds.
export const mockInterviews: MockInterview[] = [
  {
    id: 'mock_203',
    interviewee: people[0],
    interviewer: currentPerson,
    season: 'Summer 2025/26',
    occurredAt: '2025-12-15T04:00:00Z',
    durationMinutes: 55,
    notes: 'Focus on clear communication and explicit trade-offs.',
    intervieweeNotes: '',
    rounds: [
      {
        id: 'round_1',
        type: 'technical',
        apiType: 'custom',
        title: 'Array and graph problem',
        description: 'Array and graph problem',
        score: 7,
        scores: { custom: 7 },
        notes: 'Clear baseline, then improved it.',
        reviewed: true,
        intervieweeComment: 'The feedback was specific and useful.',
      },
      {
        id: 'round_2',
        type: 'technical',
        apiType: 'custom',
        title: 'Rate limiter design',
        description: 'Rate limiter design',
        link: 'https://example.test/questions/rate-limiter',
        score: 8,
        scores: { custom: 8 },
        notes: 'Good choice of a sliding window; justified the memory cost.',
        reviewed: true,
        intervieweeComment: 'The feedback was specific and useful.',
      },
      {
        id: 'round_6',
        type: 'technical',
        apiType: 'custom',
        title: 'Debugging a failing test',
        description: 'Debugging a failing test',
        score: 6,
        scores: { custom: 6 },
        notes: 'Found the bug, but read the stack trace late.',
        reviewed: true,
        intervieweeComment: '',
      },
    ],
    reviewStatus: 'reviewed',
    reviewComments:
      'The feedback was specific and useful.\n\nThe feedback was specific and useful.',
  },
  {
    id: 'mock_202',
    interviewee: people[2],
    interviewer: people[3],
    season: 'Summer 2025/26',
    occurredAt: '2025-12-12T06:30:00Z',
    durationMinutes: 45,
    notes: '',
    intervieweeNotes: '',
    rounds: [
      {
        id: 'round_3',
        type: 'technical',
        apiType: 'custom',
        title: 'Caching design',
        description: 'Caching design',
        score: 4,
        scores: { custom: 4 },
        notes: 'Needs a clearer eviction invariant.',
        reviewed: false,
        intervieweeComment: '',
      },
    ],
    reviewStatus: 'pending',
  },
  {
    id: 'mock_201',
    interviewee: currentPerson,
    interviewer: people[1],
    season: 'Summer 2024/25',
    occurredAt: '2025-01-19T03:00:00Z',
    durationMinutes: 60,
    notes: '',
    intervieweeNotes:
      'Calmer than last time; I need to leave more time for testing.',
    rounds: [
      {
        id: 'round_4',
        type: 'behavioural',
        apiType: 'behavioural',
        title: 'Behavioural round',
        score: 9,
        scores: { behavioural: 9 },
        notes: 'Excellent structure, with a measurable result.',
        reviewed: true,
        intervieweeComment: 'Great session.',
      },
      {
        id: 'round_5',
        type: 'technical',
        apiType: 'leetcode',
        problemId: 'lc_56',
        title: 'Merge Intervals',
        score: 6,
        scores: {
          confirmQuestions: 8,
          algorithmDesign: 7,
          complexityAnalysis: 6,
          coding: 8,
          testing: 9,
        },
        notes: 'Talked through sorting by start time before coding.',
        reviewed: false,
        intervieweeComment: '',
      },
      {
        id: 'round_7',
        type: 'technical',
        apiType: 'leetcode',
        problemId: 'lc_1',
        title: 'Two Sum',
        score: 7,
        scores: {
          confirmQuestions: 9,
          algorithmDesign: 9,
          complexityAnalysis: 8,
          coding: 9,
          testing: 7,
        },
        notes: 'Strong hash map reasoning; test duplicates sooner.',
        reviewed: false,
        intervieweeComment: '',
      },
    ],
    reviewStatus: 'pending',
    reviewComments: 'Great session.',
  },
];

export const weeklyActivity = [
  { week: 'W1', attempts: 2, interviews: 0 },
  { week: 'W2', attempts: 4, interviews: 1 },
  { week: 'W3', attempts: 3, interviews: 0 },
  { week: 'W4', attempts: 6, interviews: 1 },
  { week: 'W5', attempts: 5, interviews: 2 },
  { week: 'W6', attempts: 4, interviews: 1 },
];

export const seasonWeeks = [
  {
    week: 1,
    topic: 'Foundations and complexity',
    startsAt: '2025-11-30T13:30:00Z',
    resource: 'Complexity worksheet',
  },
  {
    week: 2,
    topic: 'Arrays and hash maps',
    startsAt: '2025-12-07T13:30:00Z',
    resource: 'Pattern guide',
  },
  {
    week: 3,
    topic: 'Linked structures',
    startsAt: '2025-12-14T13:30:00Z',
    resource: 'Practice set',
  },
  {
    week: 4,
    topic: 'Trees and traversal',
    startsAt: '2025-12-21T13:30:00Z',
    resource: 'Tree visualiser',
  },
];

const demoMondayProblems = {
  twoSum: { id: 'lc_1', title: 'Two Sum', slug: 'two-sum', difficulty: 'easy' },
  validAnagram: {
    id: 'lc_242',
    title: 'Valid Anagram',
    slug: 'valid-anagram',
    difficulty: 'easy',
  },
  climbingStairs: {
    id: 'lc_70',
    title: 'Climbing Stairs',
    slug: 'climbing-stairs',
    difficulty: 'easy',
  },
  mergeIntervals: {
    id: 'lc_56',
    title: 'Merge Intervals',
    slug: 'merge-intervals',
    difficulty: 'medium',
  },
  levelOrder: {
    id: 'lc_102',
    title: 'Binary Tree Level Order Traversal',
    slug: 'binary-tree-level-order-traversal',
    difficulty: 'medium',
  },
  trappingRainWater: {
    id: 'lc_42',
    title: 'Trapping Rain Water',
    slug: 'trapping-rain-water',
    difficulty: 'hard',
  },
} as const;

type DemoMondayProblem =
  (typeof demoMondayProblems)[keyof typeof demoMondayProblems];

function demoPersonSummary(person: Person): PersonSummary {
  return {
    id: person.id,
    slug: person.slug,
    name: person.name,
    avatarUrl: person.avatarUrl,
  };
}

/**
 * Monday update fixtures anchored to the week the range pickers default to, so
 * the demo screens always have several updates per student to show. Offsets are
 * 0 = Monday … 6 = Sunday of that week. Built on call, never at module load, so
 * production bundles can drop this module entirely.
 */
function fixtureMondayUpdates(): MondayUpdate[] {
  const week = defaultMondayUpdateRange();
  const day = (offset: number) => {
    const date = new Date(`${week.startDate}T00:00:00Z`);
    date.setUTCDate(date.getUTCDate() + offset);
    return date.toISOString().slice(0, 10);
  };
  // Adelaide midday, so an activity timestamp keeps the calendar date it names.
  const instant = (dayOffset: number) => `${day(dayOffset)}T04:00:00Z`;
  const attempt = (
    id: string,
    dayOffset: number,
    problem: DemoMondayProblem,
    outcome: MondayUpdateAttempt['outcome'],
    minutes: number,
  ): MondayUpdateAttempt => ({
    id,
    problemId: problem.id,
    problem: problem.title,
    problemUrl: `https://leetcode.com/problems/${problem.slug}/`,
    difficulty: problem.difficulty,
    outcome,
    minutes,
    attemptedAt: instant(dayOffset),
  });
  const mock = (
    id: string,
    dayOffset: number,
    durationMinutes: number,
  ): MondayUpdateMock => ({
    id,
    occurredAt: instant(dayOffset),
    durationMinutes,
  });

  const amelia = demoPersonSummary(people[0]);
  const zara = demoPersonSummary(people[2]);
  const mentor = demoPersonSummary(people[1]);
  const activity: Record<string, MondayUpdateActivity> = {
    [amelia.id]: {
      attempts: [
        attempt(
          'monday_attempt_amelia_1',
          0,
          demoMondayProblems.twoSum,
          'solved',
          12,
        ),
        attempt(
          'monday_attempt_amelia_2',
          1,
          demoMondayProblems.mergeIntervals,
          'unsolved',
          38,
        ),
        attempt(
          'monday_attempt_amelia_3',
          3,
          demoMondayProblems.levelOrder,
          'unsolved',
          44,
        ),
        attempt(
          'monday_attempt_amelia_4',
          4,
          demoMondayProblems.trappingRainWater,
          'solved',
          52,
        ),
        attempt(
          'monday_attempt_amelia_5',
          6,
          demoMondayProblems.climbingStairs,
          'solved',
          9,
        ),
      ],
      mocks: [
        mock('monday_mock_amelia_1', 1, 45),
        mock('monday_mock_amelia_2', 5, 55),
      ],
    },
    [zara.id]: {
      attempts: [
        attempt(
          'monday_attempt_zara_1',
          0,
          demoMondayProblems.validAnagram,
          'solved',
          16,
        ),
        attempt(
          'monday_attempt_zara_2',
          2,
          demoMondayProblems.mergeIntervals,
          'unsolved',
          41,
        ),
        attempt(
          'monday_attempt_zara_3',
          3,
          demoMondayProblems.trappingRainWater,
          'unsolved',
          50,
        ),
        attempt(
          'monday_attempt_zara_4',
          5,
          demoMondayProblems.levelOrder,
          'solved',
          34,
        ),
      ],
      mocks: [
        mock('monday_mock_zara_1', 2, 60),
        mock('monday_mock_zara_2', 6, 30),
      ],
    },
  };
  const mark = (
    id: string,
    value: MondayUpdateMark,
    commentHtml: string,
    dayOffset: number,
  ): MondayUpdateMarkRecord => ({
    id,
    mentor,
    mark: value,
    commentHtml,
    updatedAt: instant(dayOffset),
  });
  const update = (
    id: string,
    student: PersonSummary,
    startOffset: number,
    endOffset: number,
    contentHtml: string,
    hours: number,
    marks: MondayUpdateMarkRecord[],
  ): MondayUpdate => {
    const startDate = day(startOffset);
    const endDate = day(endOffset);
    const own = activity[student.id] ?? { attempts: [], mocks: [] };
    // The real API derives activity from the update's own Adelaide date range.
    const within = (at: string) => {
      const dateKey = calendarDateKey(at, programTimezone);
      return dateKey >= startDate && dateKey <= endDate;
    };
    return {
      id,
      seasonId: seasons[0].id,
      student,
      startDate,
      endDate,
      contentHtml,
      hours,
      createdAt: `${endDate}T11:30:00Z`,
      editedAt: null,
      marks,
      activity: {
        attempts: own.attempts.filter((item) => within(item.attemptedAt)),
        mocks: own.mocks.filter((item) => within(item.occurredAt)),
      },
    };
  };

  return [
    update(
      'monday_update_amelia_1',
      amelia,
      0,
      2,
      '<p>Finished the hash map pattern this week and slowed down on invariants before coding.</p>',
      18,
      [
        mark(
          'monday_mark_amelia_1',
          'on_track',
          '<p>Good week. Keep narrating the invariant before you start typing.</p>',
          3,
        ),
      ],
    ),
    update(
      'monday_update_amelia_2',
      amelia,
      3,
      6,
      '<p>Intervals are still shaky; two mocks helped me talk through the trade-offs.</p>',
      22.5,
      [],
    ),
    update(
      'monday_update_zara_1',
      zara,
      0,
      3,
      '<p>Started the week with anagrams and moved on to interval merging.</p>',
      30,
      [],
    ),
    update(
      'monday_update_zara_2',
      zara,
      2,
      6,
      '<p>Overlapping week: kept going on intervals and finished with tree traversal.</p>',
      27,
      [
        mark(
          'monday_mark_zara_2',
          'doing_good',
          '<p>Solid progress on interval reasoning. Next: re-derive the merge condition aloud.</p>',
          7,
        ),
      ],
    ),
  ];
}

const mondayStorageKey = 'rsp-demo-monday-updates';
function storedMondayUpdates(): { added: MondayUpdate[]; deleted: string[] } {
  try {
    return JSON.parse(
      localStorage.getItem(mondayStorageKey) ?? '{"added":[],"deleted":[]}',
    );
  } catch {
    return { added: [], deleted: [] };
  }
}

export function demoMondayUpdates(): MondayUpdate[] {
  const stored = storedMondayUpdates();
  // An edited fixture is stored under its own id and replaces the original.
  const addedIds = new Set(stored.added.map((update) => update.id));
  return [
    ...stored.added,
    ...fixtureMondayUpdates().filter((update) => !addedIds.has(update.id)),
  ].filter((update) => !stored.deleted.includes(update.id));
}

export function saveDemoMondayUpdate(update: MondayUpdate) {
  const stored = storedMondayUpdates();
  if (
    demoMondayUpdates().some(
      (item) =>
        item.seasonId === update.seasonId &&
        item.student.id === update.student.id &&
        item.startDate === update.startDate &&
        item.endDate === update.endDate,
    )
  ) {
    throw new Error('You have already submitted an update for this week.');
  }
  localStorage.setItem(
    mondayStorageKey,
    JSON.stringify({ ...stored, added: [update, ...stored.added] }),
  );
}

export function deleteDemoMondayUpdate(id: string) {
  const stored = storedMondayUpdates();
  localStorage.setItem(
    mondayStorageKey,
    JSON.stringify({ ...stored, deleted: [...stored.deleted, id] }),
  );
}

/** Rewrites a demo update's content and hours, keeping its dates. */
export function editDemoMondayUpdate(
  id: string,
  changes: Pick<MondayUpdate, 'contentHtml' | 'hours'>,
) {
  const current = demoMondayUpdates().find((item) => item.id === id);
  if (!current) throw new Error('This update no longer exists.');
  const stored = storedMondayUpdates();
  const edited = { ...current, ...changes };
  localStorage.setItem(
    mondayStorageKey,
    JSON.stringify({
      ...stored,
      added: [edited, ...stored.added.filter((item) => item.id !== id)],
    }),
  );
  return edited;
}

const mondayUnlockStorageKey = 'rsp-demo-monday-unlocks';

/** Demo requests to unlock late weeks, kept for the browser session. */
export function demoMondayUpdateUnlocks(): MondayUpdateUnlock[] {
  try {
    return JSON.parse(localStorage.getItem(mondayUnlockStorageKey) ?? '[]');
  } catch {
    return [];
  }
}

/** Saves a demo request, or replaces one with the same id. */
export function saveDemoMondayUpdateUnlock(unlock: MondayUpdateUnlock) {
  localStorage.setItem(
    mondayUnlockStorageKey,
    JSON.stringify([
      unlock,
      ...demoMondayUpdateUnlocks().filter((item) => item.id !== unlock.id),
    ]),
  );
  return unlock;
}

const weekMarkStorageKey = 'rsp-demo-week-marks';

/** Demo mentor marks on weeks without an update. */
export function demoWeekMarks(): MondayWeekMark[] {
  try {
    return JSON.parse(localStorage.getItem(weekMarkStorageKey) ?? '[]');
  } catch {
    return [];
  }
}

export function saveDemoWeekMark(mark: MondayWeekMark) {
  localStorage.setItem(
    weekMarkStorageKey,
    JSON.stringify([
      mark,
      ...demoWeekMarks().filter(
        (item) =>
          !(
            item.seasonId === mark.seasonId &&
            item.studentId === mark.studentId &&
            item.mentorId === mark.mentorId &&
            item.startDate === mark.startDate
          ),
      ),
    ]),
  );
  return mark;
}

/** Removes a demo mentor's mark on a week ("Not marked"). */
export function clearDemoWeekMark(
  seasonId: string,
  studentId: string,
  mentorId: string,
  startDate: string,
) {
  localStorage.setItem(
    weekMarkStorageKey,
    JSON.stringify(
      demoWeekMarks().filter(
        (item) =>
          !(
            item.seasonId === seasonId &&
            item.studentId === studentId &&
            item.mentorId === mentorId &&
            item.startDate === startDate
          ),
      ),
    ),
  );
}
