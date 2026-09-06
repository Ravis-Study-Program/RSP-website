import {
  adaptAttemptPage,
  adaptCurrentUser,
  adaptMockInterviewPage,
  adaptSeasonPage,
  adaptUserPage,
  indexProblems,
} from '@/api/adapters';
import type {
  AttemptPage,
  LeetcodeProblem,
  Me,
  MockInterviewPage,
  SeasonPage,
  UserPage,
} from '@/api/generated/models';

const pageInfo = { nextCursor: null, previousCursor: null, hasMore: false };
const problem: LeetcodeProblem = {
  id: 'problem-347',
  number: 347,
  title: 'Top K Frequent Elements',
  link: 'https://leetcode.com/problems/top-k-frequent-elements/',
  difficulty: 'medium',
  categories: ['Hash maps', 'Sorting'],
  premium: false,
};

describe('OpenAPI DTO adapters', () => {
  it('maps the canonical /me payload without inventing identity fields', () => {
    const dto: Me = {
      id: 'user-1',
      slug: 'amelia-chen',
      name: 'Amelia Chen',
      avatarUrl: null,
      timezone: 'Australia/Adelaide',
      timezoneConfigured: true,
      globalRoles: ['director'],
      attemptCount: 12,
      mockInterviewCount: 3,
      email: 'amelia@example.test',
      accountState: 'active',
      emailVerified: true,
      mfaVerified: true,
      seasonRoles: [
        {
          seasonId: 'season-1',
          seasonSlug: 'summer-2025-26',
          role: 'coordinator',
          state: 'active',
        },
      ],
      alumni: true,
    };

    expect(adaptCurrentUser(dto)).toEqual({
      ...dto,
      avatarUrl: null,
      globalRoles: ['director'],
      seasonRoles: [
        {
          seasonId: 'season-1',
          seasonSlug: 'summer-2025-26',
          role: 'coordinator',
          state: 'active',
        },
      ],
    });
  });

  it('maps seasons and public users while leaving unavailable aggregates unknown', () => {
    const seasons: SeasonPage = {
      pageInfo,
      totalCount: 1,
      items: [
        {
          id: 'season-1',
          slug: 'summer-2025-26',
          name: 'Summer 2025/26',
          status: 'open',
          startAt: '2026-07-27T00:00:00Z',
          endAt: '2026-11-20T00:00:00Z',
          location: 'Adelaide University',
          imageUrl: 'https://example.test/season.png',
          resourcesUrl: 'https://example.test/resources',
        },
      ],
    };
    const users: UserPage = {
      pageInfo,
      totalCount: 1,
      items: [
        {
          id: 'user-2',
          slug: 'noah',
          name: 'Noah Williams',
          avatarUrl: null,
          timezone: 'UTC',
          timezoneConfigured: true,
          globalRoles: [],
          seasonRoles: [],
          attemptCount: 0,
          mockInterviewCount: 0,
        },
      ],
    };

    expect(adaptSeasonPage(seasons).items[0]).toMatchObject({
      startsAt: '2026-07-27T00:00:00Z',
      endsAt: '2026-11-20T00:00:00Z',
      location: 'Adelaide University',
      memberCount: null,
      weekCount: null,
    });
    expect(adaptUserPage(users).items[0]).toMatchObject({
      roles: [],
      season: null,
      status: null,
      attempts: 0,
      interviews: 0,
      lastActiveAt: null,
    });
  });

  it('joins attempt DTOs to their catalog problem', () => {
    const attempts: AttemptPage = {
      pageInfo,
      totalCount: 1,
      items: [
        {
          id: 'attempt-1',
          problemId: problem.id,
          outcome: 'solved_with_hints',
          confidence: 3,
          minutes: 32,
          notes: '<p>Used a heap.</p>',
          attemptedAt: '2026-08-13T00:00:00Z',
        },
      ],
    };
    expect(adaptAttemptPage(attempts, [problem]).items[0]).toMatchObject({
      problemId: problem.id,
      problem: problem.title,
      difficulty: 'Medium',
      category: 'Hash maps, Sorting',
      categories: ['Hash maps', 'Sorting'],
    });
  });

  it('maps mock interview IDs, score dimensions and per-round review data', () => {
    const interviews: MockInterviewPage = {
      pageInfo,
      totalCount: 1,
      items: [
        {
          id: 'mock-1',
          interviewerId: 'interviewer',
          intervieweeId: 'interviewee',
          occurredAt: '2026-08-12T03:00:00Z',
          interviewer: {
            id: 'interviewer',
            slug: 'mentor',
            name: 'Mentor User',
          },
          interviewee: {
            id: 'interviewee',
            slug: 'student',
            name: 'Student User',
          },
          durationMinutes: 60,
          notes: '',
          rounds: [
            {
              id: 'round-1',
              type: 'leetcode',
              problemId: problem.id,
              content: '<p>Clear solution.</p>',
              scores: { algorithmDesign: 8, coding: 6, testing: 7 },
              reviewed: true,
              intervieweeComment: 'Useful feedback.',
            },
          ],
        },
      ],
    };
    const mapped = adaptMockInterviewPage(interviews, {
      users: new Map(),
      seasons: new Map(),
      problems: indexProblems([problem]),
    }).items[0];

    expect(mapped).toMatchObject({
      interviewee: { name: 'Student User' },
      interviewer: { name: 'Mentor User' },
      reviewStatus: 'reviewed',
      reviewComments: 'Useful feedback.',
      rounds: [
        {
          title: problem.title,
          score: 6,
          scores: { algorithmDesign: 8, coding: 6, testing: 7 },
        },
      ],
    });
  });
});
