import type {
  MondayUpdate,
  MondayUpdateActivity,
  MondayUpdateAttempt,
  MondayUpdateMark,
  MondayUpdateMarkRecord,
  MondayUpdateMock,
  PersonSummary,
} from '@/api/generated/models';
import { defaultMondayUpdateRange, programmeTimezone } from '@/activityDates';
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
      'The current programme for students building interview confidence through practice and mentoring.',
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
      'Completed programme history, available read-only to participants and alumni.',
  },
];

export const attempts: Attempt[] = [
  {
    id: 'attempt_7',
    problemId: 'lc_102',
    problem: 'Binary Tree Level Order Traversal',
    difficulty: 'Medium',
    category: 'Trees',
    outcome: 'independently_solved',
    confidence: 4,
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
    outcome: 'solved_with_hints',
    confidence: 3,
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
    outcome: 'not_solved',
    confidence: 2,
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
    outcome: 'independently_solved',
    confidence: 5,
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
    outcome: 'unknown',
    confidence: null,
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

export const mockInterviews: MockInterview[] = [
  {
    id: 'mock_203',
    interviewee: people[0],
    interviewer: currentPerson,
    season: 'Summer 2025/26',
    occurredAt: '2025-12-15T04:00:00Z',
    durationMinutes: 55,
    notes: 'Focus on clear communication and explicit trade-offs.',
    rounds: [
      {
        id: 'round_1',
        type: 'technical',
        apiType: 'custom',
        title: 'Array and graph problem',
        score: 7,
        scores: { coding: 7 },
        notes: 'Clear baseline, then improved it.',
        reviewed: true,
        intervieweeComment: 'The feedback was specific and useful.',
      },
      {
        id: 'round_2',
        type: 'behavioural',
        apiType: 'behavioural',
        title: 'Teamwork reflection',
        score: 8,
        scores: { behavioural: 8 },
        notes: 'Specific and thoughtful example.',
        reviewed: true,
        intervieweeComment: 'The feedback was specific and useful.',
      },
    ],
    reviewStatus: 'reviewed',
    reviewComments: 'The feedback was specific and useful.',
  },
  {
    id: 'mock_202',
    interviewee: people[2],
    interviewer: people[3],
    season: 'Summer 2025/26',
    occurredAt: '2025-12-12T06:30:00Z',
    durationMinutes: 45,
    notes: '',
    rounds: [
      {
        id: 'round_3',
        type: 'technical',
        apiType: 'custom',
        title: 'Caching design',
        score: 4,
        scores: { algorithmDesign: 4 },
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
    rounds: [
      {
        id: 'round_4',
        type: 'technical',
        apiType: 'custom',
        title: 'Data structure implementation',
        score: 9,
        scores: { coding: 9 },
        notes: 'Excellent testing discipline.',
        reviewed: true,
        intervieweeComment: 'Great session.',
      },
    ],
    reviewStatus: 'reviewed',
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

/**
 * Monday update fixtures are anchored to the week the range pickers default to,
 * so the demo screens always have several updates per student to show. Offsets
 * are 0 = Monday … 6 = Sunday of that week.
 */
const demoMondayWeek = defaultMondayUpdateRange();

function demoMondayDay(offset: number) {
  const date = new Date(`${demoMondayWeek.startDate}T00:00:00Z`);
  date.setUTCDate(date.getUTCDate() + offset);
  return date.toISOString().slice(0, 10);
}

/** Adelaide midday, so an activity timestamp keeps the calendar date it names. */
const demoMondayInstant = (dayOffset: number) =>
  `${demoMondayDay(dayOffset)}T04:00:00Z`;

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

function demoMondayAttempt(
  id: string,
  dayOffset: number,
  problem: (typeof demoMondayProblems)[keyof typeof demoMondayProblems],
  outcome: MondayUpdateAttempt['outcome'],
  minutes: number,
): MondayUpdateAttempt {
  return {
    id,
    problemId: problem.id,
    problem: problem.title,
    problemUrl: `https://leetcode.com/problems/${problem.slug}/`,
    difficulty: problem.difficulty,
    outcome,
    minutes,
    attemptedAt: demoMondayInstant(dayOffset),
  };
}

function demoMondayMock(
  id: string,
  dayOffset: number,
  durationMinutes: number,
): MondayUpdateMock {
  return {
    id,
    occurredAt: demoMondayInstant(dayOffset),
    durationMinutes,
  };
}

const demoMondayStudents: PersonSummary[] = [people[0], people[2]].map(
  (person) => ({
    id: person.id,
    slug: person.slug,
    name: person.name,
    avatarUrl: person.avatarUrl,
  }),
);

const demoMondayMentor: PersonSummary = {
  id: people[1].id,
  slug: people[1].slug,
  name: people[1].name,
  avatarUrl: people[1].avatarUrl,
};

const demoMondayActivity: Record<string, MondayUpdateActivity> = {
  [demoMondayStudents[0].id]: {
    attempts: [
      demoMondayAttempt(
        'monday_attempt_amelia_1',
        0,
        demoMondayProblems.twoSum,
        'independently_solved',
        12,
      ),
      demoMondayAttempt(
        'monday_attempt_amelia_2',
        1,
        demoMondayProblems.mergeIntervals,
        'solved_with_hints',
        38,
      ),
      demoMondayAttempt(
        'monday_attempt_amelia_3',
        3,
        demoMondayProblems.levelOrder,
        'not_solved',
        44,
      ),
      demoMondayAttempt(
        'monday_attempt_amelia_4',
        4,
        demoMondayProblems.trappingRainWater,
        'independently_solved',
        52,
      ),
      demoMondayAttempt(
        'monday_attempt_amelia_5',
        6,
        demoMondayProblems.climbingStairs,
        'unknown',
        9,
      ),
    ],
    mocks: [
      demoMondayMock('monday_mock_amelia_1', 1, 45),
      demoMondayMock('monday_mock_amelia_2', 5, 55),
    ],
  },
  [demoMondayStudents[1].id]: {
    attempts: [
      demoMondayAttempt(
        'monday_attempt_zara_1',
        0,
        demoMondayProblems.validAnagram,
        'independently_solved',
        16,
      ),
      demoMondayAttempt(
        'monday_attempt_zara_2',
        2,
        demoMondayProblems.mergeIntervals,
        'solved_with_hints',
        41,
      ),
      demoMondayAttempt(
        'monday_attempt_zara_3',
        3,
        demoMondayProblems.trappingRainWater,
        'not_solved',
        50,
      ),
      demoMondayAttempt(
        'monday_attempt_zara_4',
        5,
        demoMondayProblems.levelOrder,
        'independently_solved',
        34,
      ),
    ],
    mocks: [
      demoMondayMock('monday_mock_zara_1', 2, 60),
      demoMondayMock('monday_mock_zara_2', 6, 30),
    ],
  },
};

function demoMondayMark(
  id: string,
  mark: MondayUpdateMark,
  commentHtml: string,
  dayOffset: number,
): MondayUpdateMarkRecord {
  return {
    id,
    mentor: demoMondayMentor,
    mark,
    commentHtml,
    updatedAt: demoMondayInstant(dayOffset),
  };
}

function demoMondayUpdate(
  id: string,
  student: PersonSummary,
  startOffset: number,
  endOffset: number,
  contentHtml: string,
  marks: MondayUpdateMarkRecord[],
): MondayUpdate {
  const startDate = demoMondayDay(startOffset);
  const endDate = demoMondayDay(endOffset);
  const activity = demoMondayActivity[student.id] ?? {
    attempts: [],
    mocks: [],
  };
  // The real API derives activity from the update's own Adelaide date range.
  const within = (at: string) => {
    const dateKey = calendarDateKey(at, programmeTimezone);
    return dateKey >= startDate && dateKey <= endDate;
  };
  return {
    id,
    seasonId: seasons[0].id,
    student,
    startDate,
    endDate,
    contentHtml,
    createdAt: `${endDate}T11:30:00Z`,
    marks,
    activity: {
      attempts: activity.attempts.filter((attempt) =>
        within(attempt.attemptedAt),
      ),
      mocks: activity.mocks.filter((mock) => within(mock.occurredAt)),
    },
  };
}

export const mondayUpdates: MondayUpdate[] = [
  demoMondayUpdate(
    'monday_update_amelia_1',
    demoMondayStudents[0],
    0,
    2,
    '<p>Finished the hash map pattern this week and slowed down on invariants before coding.</p>',
    [
      demoMondayMark(
        'monday_mark_amelia_1',
        'on_track',
        '<p>Good week. Keep narrating the invariant before you start typing.</p>',
        3,
      ),
    ],
  ),
  demoMondayUpdate(
    'monday_update_amelia_2',
    demoMondayStudents[0],
    3,
    6,
    '<p>Intervals are still shaky; two mocks helped me talk through the trade-offs.</p>',
    [],
  ),
  demoMondayUpdate(
    'monday_update_zara_1',
    demoMondayStudents[1],
    0,
    3,
    '<p>Started the week with anagrams and moved on to interval merging.</p>',
    [],
  ),
  demoMondayUpdate(
    'monday_update_zara_2',
    demoMondayStudents[1],
    2,
    6,
    '<p>Overlapping week: kept going on intervals and finished with tree traversal.</p>',
    [
      demoMondayMark(
        'monday_mark_zara_2',
        'doing_good',
        '<p>Solid progress on interval reasoning. Next: re-derive the merge condition aloud.</p>',
        7,
      ),
    ],
  ),
];
