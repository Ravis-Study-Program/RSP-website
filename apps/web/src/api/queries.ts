import {
  activityFilterQuery,
  emptyActivityFilters,
  type ActivityFilters,
} from '@/activityFilters';
import { programmeTimezone } from '@/activityDates';
import { calendarDateKey, practiceGoalMinutes } from '@/utils';
import {
  queryOptions,
  useInfiniteQuery,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query';

import {
  adaptAttemptPage,
  adaptCurrentUser,
  adaptMockInterviewPage,
  adaptMockParticipantPage,
  adaptPerson,
  adaptSeasonPage,
  adaptUserPage,
} from '@/api/adapters';
import { ApiError, apiRequest } from '@/api/client';
import { demoModeForEnvironment } from '@/api/demoMode';
import type {
  ActivitySummary,
  AttemptPage as ApiAttemptPage,
  CategoryList,
  Enrollment,
  EnrollmentCandidate,
  EnrollmentCandidatePage,
  EnrollmentPage,
  LeetcodeProblem,
  Me,
  MockInterviewPage as ApiMockInterviewPage,
  MockParticipantSummaryPage,
  MentorshipPage,
  Mentorship,
  MondayUpdate,
  MondayUpdateActivity,
  MondayUpdateDifficultyCounts,
  MondayUpdateReviewRow,
  PersonSummary,
  ProblemPage,
  PracticeSettings,
  Participation,
  User as ApiUser,
  UserPrivate,
  UserPage as ApiUserPage,
  Week,
  WeekPage,
} from '@/api/generated/models';
import { fetchPracticeSettings } from '@/api/practiceSettingsClient';
import {
  attempts,
  demoUser,
  mockInterviews,
  demoMondayUpdates,
  people,
  seasonWeeks,
  seasons,
} from '@/data/demo';
import type { CurrentUser, MockInterview, Page, Person } from '@/types';

export const demoMode = demoModeForEnvironment(
  import.meta.env.VITE_USE_DEMO_DATA,
  import.meta.env.MODE,
);

const wait = <T>(value: T) =>
  new Promise<T>((resolve) => window.setTimeout(() => resolve(value), 90));
const page = <T>(items: T[]): Page<T> => ({
  items,
  totalCount: items.length,
  pageInfo: { nextCursor: null, previousCursor: null, hasMore: false },
});

interface ApiPage<T> {
  items: T[];
  totalCount: number;
  pageInfo: {
    nextCursor: string | null;
    previousCursor: string | null;
    hasMore: boolean;
  };
}

/**
 * Resolve every cursor page when a screen needs a complete relationship graph
 * (participant selectors and season joins). The API cursor remains opaque and
 * is always passed back verbatim.
 */
export async function fetchAllPages<T>(path: string): Promise<ApiPage<T>> {
  const items: T[] = [];
  const seen = new Set<string>();
  let cursor: string | null = null;
  let totalCount = 0;
  do {
    const separator = path.includes('?') ? '&' : '?';
    const response: ApiPage<T> = await apiRequest<ApiPage<T>>(
      `${path}${cursor ? `${separator}cursor=${encodeURIComponent(cursor)}` : ''}`,
    );
    items.push(...response.items);
    totalCount = response.totalCount;
    const next: string | null = response.pageInfo.nextCursor ?? null;
    if (!response.pageInfo.hasMore || !next || seen.has(next)) break;
    seen.add(next);
    cursor = next;
  } while (cursor);
  return {
    items,
    totalCount: Math.max(totalCount, items.length),
    pageInfo: { nextCursor: null, previousCursor: null, hasMore: false },
  };
}

export const demoPracticeSettings: PracticeSettings = {
  goalsEnabled: true,
  easyMinutes: practiceGoalMinutes.Easy,
  mediumMinutes: practiceGoalMinutes.Medium,
  hardMinutes: practiceGoalMinutes.Hard,
};

function selectedDemoUser(): CurrentUser {
  const role = window.localStorage.getItem('rsp-demo-role');
  if (role === 'unverified')
    return {
      ...demoUser,
      emailVerified: false,
      globalRoles: [],
      seasonRoles: [],
      alumni: false,
    };
  if (role === 'student' || role === 'mentor' || role === 'coordinator') {
    return {
      ...demoUser,
      globalRoles: [],
      seasonRoles: [
        {
          seasonId: seasons[0].id,
          seasonSlug: seasons[0].slug,
          role,
          state: 'active',
        },
      ],
      alumni: role === 'mentor',
    };
  }
  if (role === 'graduate')
    return { ...demoUser, globalRoles: [], seasonRoles: [], alumni: true };
  if (role === 'former_member' || role === 'kicked')
    return {
      ...demoUser,
      globalRoles: [],
      seasonRoles: [
        {
          seasonId: seasons[1].id,
          seasonSlug: seasons[1].slug,
          role: role === 'kicked' ? 'student' : 'mentor',
          state: role === 'kicked' ? 'kicked' : 'completed',
        },
      ],
      alumni: false,
    };
  if (role === 'director')
    return {
      ...demoUser,
      globalRoles: ['director'],
      seasonRoles: [],
      alumni: true,
    };
  if (role === 'system_admin')
    return {
      ...demoUser,
      globalRoles: ['system_admin'],
      seasonRoles: [],
      alumni: true,
    };
  if (role === 'nonmember')
    return { ...demoUser, globalRoles: [], seasonRoles: [], alumni: false };
  if (role === 'suspended')
    return {
      ...demoUser,
      accountState: 'suspended',
      globalRoles: [],
      seasonRoles: [],
      alumni: false,
    };
  if (role === 'deletion_pending')
    return {
      ...demoUser,
      accountState: 'deletion_pending',
      globalRoles: [],
      seasonRoles: [],
      alumni: false,
    };
  return demoUser;
}

function demoAuthenticationError() {
  return new ApiError(401, {
    message: 'Sign in to continue.',
    requestId: 'demo',
  });
}

const demoProblems: LeetcodeProblem[] = [
  ...new Map(
    attempts.map((attempt) => [
      attempt.problemId,
      {
        id: attempt.problemId,
        number: 0,
        title: attempt.problem,
        link: `https://leetcode.com/problemset/?search=${encodeURIComponent(attempt.problem)}`,
        difficulty: (
          attempt.difficulty ?? 'Medium'
        ).toLowerCase() as LeetcodeProblem['difficulty'],
        categories: attempt.category ? [attempt.category] : [],
        premium: false,
      },
    ]),
  ).values(),
].map((problem, index) => ({ ...problem, number: index + 1 }));
const demoProblemById = new Map(
  demoProblems.map((problem) => [problem.id, problem]),
);

const problemSearchLimit = 20;

/** Demo stand-in for /me/problem-suggestion over the complete demo history. */
export function suggestDemoProblem(
  filters: Pick<ActivityFilters, 'difficulties' | 'categories'>,
): LeetcodeProblem | null {
  const attempted = new Set(attempts.map((attempt) => attempt.problemId));
  const candidates = demoProblems.filter(
    (problem) =>
      !attempted.has(problem.id) &&
      (!filters.difficulties.length ||
        filters.difficulties.includes(problem.difficulty)) &&
      (!filters.categories.length ||
        problem.categories.some((c) => filters.categories.includes(c))),
  );
  return candidates[Math.floor(Math.random() * candidates.length)] ?? null;
}

/** Searches the catalogue by title or exact number; never loads it whole. */
export function useProblemSearch(query: string) {
  const q = query.trim();
  return useQuery({
    queryKey: ['leetcode-problems', 'search', q],
    enabled: q.length > 0,
    staleTime: 5 * 60_000,
    queryFn: async (): Promise<LeetcodeProblem[]> => {
      if (demoMode) {
        const needle = q.toLowerCase();
        return wait(
          demoProblems
            .filter(
              (problem) =>
                String(problem.number) === needle ||
                problem.title.toLowerCase().includes(needle),
            )
            .slice(0, problemSearchLimit),
        );
      }
      const result = await apiRequest<ProblemPage>(
        `/leetcode-problems?limit=${problemSearchLimit}&q=${encodeURIComponent(q)}`,
      );
      return [...result.items].sort(
        (left, right) => left.number - right.number,
      );
    },
  });
}

export function useLeetcodeCategories() {
  return useQuery({
    queryKey: ['leetcode-categories'],
    staleTime: 30 * 60_000,
    queryFn: async (): Promise<string[]> =>
      demoMode
        ? wait([...new Set(demoProblems.flatMap((p) => p.categories))].sort())
        : (await apiRequest<CategoryList>('/leetcode-categories')).items,
  });
}

async function getUsers(): Promise<ApiUserPage> {
  return fetchAllPages<ApiUser>('/users?limit=100');
}

export const currentUserOptions = queryOptions({
  queryKey: ['me'],
  queryFn: async () => {
    if (!demoMode) return adaptCurrentUser(await apiRequest<Me>('/me'));
    if (window.localStorage.getItem('rsp-demo-auth-state') === 'signed-out')
      throw demoAuthenticationError();
    return wait(selectedDemoUser());
  },
  staleTime: 60_000,
});

export const seasonsOptions = queryOptions({
  queryKey: ['seasons'],
  queryFn: async () =>
    demoMode
      ? wait(page(seasons))
      : adaptSeasonPage(await fetchAllPages('/seasons?limit=100')),
  staleTime: demoMode ? Infinity : 0,
});

export const practiceSettingsOptions = queryOptions({
  queryKey: ['me', 'practice-settings'],
  queryFn: () =>
    demoMode ? wait(demoPracticeSettings) : fetchPracticeSettings(),
  staleTime: 60_000,
});

export function useCurrentUser() {
  return useQuery(currentUserOptions);
}

export function useSeasons(enabled = true) {
  return useQuery({
    ...seasonsOptions,
    enabled,
  });
}

export function usePracticeSettings() {
  return useQuery(practiceSettingsOptions);
}

export const seasonWeeksQueryKey = (seasonId?: string) =>
  ['season-weeks', seasonId] as const;
export const seasonEnrollmentsQueryKey = (seasonId?: string) =>
  ['season-enrollments', seasonId] as const;
export const seasonMentorshipsQueryKey = (seasonId?: string) =>
  ['season-mentorships', seasonId] as const;
export const enrollmentCandidatesQueryKey = (seasonId?: string) =>
  ['enrollment-candidates', seasonId] as const;

export interface MondayUpdateFilters {
  startDate: string;
  endDate: string;
  studentIds: string[];
}

export const mondayUpdatesQueryKey = (
  seasonId: string | undefined,
  filters: MondayUpdateFilters,
) => ['monday-updates', seasonId, filters] as const;

export const mondayUpdateReviewQueryKey = (
  seasonId: string | undefined,
  startDate: string,
  endDate: string,
) => ['monday-update-review', seasonId, startDate, endDate] as const;

function demoWeekPage(seasonId: string): WeekPage {
  const season = seasons.find((season) => season.id === seasonId);
  if (!season) return page([]);
  const start = new Date(season.startsAt).getTime();
  const items: Week[] = seasonWeeks.map((week) => ({
    id: `week_${seasonId}_${week.week}`,
    seasonId,
    number: week.week,
    startAt: new Date(start + (week.week - 1) * 7 * 86_400_000).toISOString(),
    endAt: new Date(start + week.week * 7 * 86_400_000 - 1).toISOString(),
    resourceUrl: `https://example.test/resources/week-${week.week}`,
  }));
  return page(items);
}

export function demoActivityWeekId(seasonId: string | undefined, at: string) {
  return seasonId
    ? demoWeekPage(seasonId).items.find(
        (week) => at >= week.startAt && at <= week.endAt,
      )?.id
    : undefined;
}

function demoEnrollmentPage(seasonId: string): EnrollmentPage {
  if (!seasons.some((season) => season.id === seasonId)) return page([]);
  const items: Enrollment[] = people
    .filter((person) =>
      person.roles.some(
        (role) =>
          role === 'student' || role === 'mentor' || role === 'coordinator',
      ),
    )
    .map((person) => ({
      id: `enrol_${seasonId}_${person.id}`,
      seasonId,
      userId: person.id,
      role: person.roles.find(
        (role) =>
          role === 'student' || role === 'mentor' || role === 'coordinator',
      ) as Enrollment['role'],
      studentLevel: person.roles.includes('student')
        ? 'beginner'
        : 'not_applicable',
      state: person.status === 'completed' ? 'completed' : 'active',
      assignmentState: 'active',
      removalReason: null,
    }));
  return page(items);
}

function demoMentorshipPage(seasonId: string): MentorshipPage {
  if (!seasons.some((season) => season.id === seasonId)) return page([]);
  const mentor = people.find((person) => person.roles.includes('mentor'));
  const student = people.find(
    (person) =>
      person.roles.includes('student') && person.status !== 'unassigned',
  );
  const items: Mentorship[] =
    mentor && student
      ? [
          {
            id: `mentorship_${seasonId}_${student.id}`,
            seasonId,
            mentorUserId: mentor.id,
            studentUserId: student.id,
          },
        ]
      : [];
  return page(items);
}

const dateRangesOverlap = (
  startDate: string,
  endDate: string,
  rangeStart: string,
  rangeEnd: string,
) => startDate <= rangeEnd && endDate >= rangeStart;

function uniqueById<T extends { id: string }>(items: T[]) {
  return [...new Map(items.map((item) => [item.id, item])).values()];
}

function demoMondayUpdatePage(
  seasonId: string,
  filters: MondayUpdateFilters,
): Page<MondayUpdate> {
  if (!seasons.some((season) => season.id === seasonId)) return page([]);
  return page(
    demoMondayUpdates().filter(
      (update) =>
        update.seasonId === seasonId &&
        dateRangesOverlap(
          update.startDate,
          update.endDate,
          filters.startDate,
          filters.endDate,
        ) &&
        (filters.studentIds.length === 0 ||
          filters.studentIds.includes(update.student.id)),
    ),
  );
}

function demoMondayUpdateReview(
  seasonId: string,
  startDate: string,
  endDate: string,
): Page<MondayUpdateReviewRow> {
  const students: PersonSummary[] = demoEnrollmentPage(seasonId).items.flatMap(
    (enrollment) => {
      const person = people.find((item) => item.id === enrollment.userId);
      return enrollment.role === 'student' && person
        ? [
            {
              id: person.id,
              slug: person.slug,
              name: person.name,
              avatarUrl: person.avatarUrl,
            },
          ]
        : [];
    },
  );
  const allUpdates = demoMondayUpdates();
  return page(
    students.map((student): MondayUpdateReviewRow => {
      const updates = allUpdates.filter(
        (update) =>
          update.student.id === student.id &&
          dateRangesOverlap(
            update.startDate,
            update.endDate,
            startDate,
            endDate,
          ),
      );
      // Per-student totals deduplicate activity across overlapping updates.
      const attempts = uniqueById(
        updates.flatMap((update) => update.activity.attempts),
      ).filter(
        (attempt) =>
          attempt.outcome === 'independently_solved' ||
          attempt.outcome === 'solved_with_hints',
      );
      const leetcodeCounts: MondayUpdateDifficultyCounts = {
        easy: 0,
        medium: 0,
        hard: 0,
      };
      for (const attempt of attempts) {
        const difficulty = attempt.difficulty.toLowerCase();
        if (
          difficulty === 'easy' ||
          difficulty === 'medium' ||
          difficulty === 'hard'
        )
          leetcodeCounts[difficulty] += 1;
      }
      return {
        student,
        leetcodeCounts,
        mockCount: uniqueById(
          updates.flatMap((update) => update.activity.mocks),
        ).length,
        updateCount: updates.length,
        updates: updates.map((update) => ({
          id: update.id,
          startDate: update.startDate,
          endDate: update.endDate,
          contentHtml: update.contentHtml,
          hours: update.hours,
          createdAt: update.createdAt,
          marks: update.marks,
        })),
      };
    }),
  );
}

export function useSeasonWeeks(seasonId?: string) {
  return useQuery({
    queryKey: seasonWeeksQueryKey(seasonId),
    enabled: Boolean(seasonId),
    queryFn: () => {
      if (!seasonId) return page<Week>([]);
      return demoMode
        ? wait(demoWeekPage(seasonId))
        : fetchAllPages<Week>(
            `/seasons/${encodeURIComponent(seasonId)}/weeks?limit=100`,
          );
    },
  });
}

export function useSeasonEnrollments(seasonId?: string) {
  return useQuery({
    queryKey: seasonEnrollmentsQueryKey(seasonId),
    enabled: Boolean(seasonId),
    queryFn: () => {
      if (!seasonId) return page<Enrollment>([]);
      return demoMode
        ? wait(demoEnrollmentPage(seasonId))
        : fetchAllPages<Enrollment>(
            `/seasons/${encodeURIComponent(seasonId)}/members?limit=100`,
          );
    },
  });
}

export function useSeasonMentorships(seasonId?: string) {
  return useQuery({
    queryKey: seasonMentorshipsQueryKey(seasonId),
    enabled: Boolean(seasonId),
    queryFn: () => {
      if (!seasonId) return page<Mentorship>([]);
      return demoMode
        ? wait(demoMentorshipPage(seasonId))
        : fetchAllPages<Mentorship>(
            `/seasons/${encodeURIComponent(seasonId)}/mentorships?limit=100`,
          );
    },
  });
}

export function useEnrollmentCandidates(seasonId?: string, enabled = true) {
  return useQuery({
    queryKey: enrollmentCandidatesQueryKey(seasonId),
    enabled: Boolean(seasonId) && enabled,
    queryFn: async (): Promise<EnrollmentCandidatePage> => {
      if (!seasonId) return page<EnrollmentCandidate>([]);
      if (demoMode)
        return wait(
          page([
            {
              id: 'person_candidate',
              slug: 'priya-singh',
              name: 'Priya Singh',
              avatarUrl: null,
            },
          ]),
        );
      return fetchAllPages<EnrollmentCandidate>(
        `/seasons/${encodeURIComponent(seasonId)}/enrollment-candidates?limit=100`,
      );
    },
  });
}

export function useMondayUpdates(
  enabled: boolean,
  seasonId: string | undefined,
  filters: MondayUpdateFilters,
) {
  return useQuery({
    queryKey: mondayUpdatesQueryKey(seasonId, filters),
    enabled: enabled && Boolean(seasonId),
    queryFn: () => {
      if (!seasonId) return page<MondayUpdate>([]);
      return demoMode
        ? wait(demoMondayUpdatePage(seasonId, filters))
        : fetchAllPages<MondayUpdate>(
            `/seasons/${encodeURIComponent(seasonId)}/monday-updates?limit=100&startDate=${encodeURIComponent(filters.startDate)}&endDate=${encodeURIComponent(filters.endDate)}${filters.studentIds
              .map((studentId) => `&studentId=${encodeURIComponent(studentId)}`)
              .join('')}`,
          );
    },
  });
}

export function useMondayUpdateReview(
  seasonId: string | undefined,
  startDate: string,
  endDate: string,
) {
  return useQuery({
    queryKey: mondayUpdateReviewQueryKey(seasonId, startDate, endDate),
    enabled: Boolean(seasonId),
    queryFn: () => {
      if (!seasonId) return page<MondayUpdateReviewRow>([]);
      return demoMode
        ? wait(demoMondayUpdateReview(seasonId, startDate, endDate))
        : fetchAllPages<MondayUpdateReviewRow>(
            `/seasons/${encodeURIComponent(seasonId)}/monday-update-review?limit=100&startDate=${encodeURIComponent(startDate)}&endDate=${encodeURIComponent(endDate)}`,
          );
    },
  });
}

/** The signed-in student's own activity inside a Monday update range. */
export function useMondayUpdateRangeActivity(
  enabled: boolean,
  seasonId: string | undefined,
  startDate: string,
  endDate: string,
) {
  return useQuery({
    queryKey: ['monday-update-activity', seasonId, startDate, endDate],
    enabled: enabled && Boolean(seasonId),
    queryFn: (): Promise<MondayUpdateActivity> => {
      if (demoMode) {
        // Demo activity lives on the demo updates; collect what falls in range.
        const inRange = (at: string) => {
          const key = calendarDateKey(at, programmeTimezone);
          return key >= startDate && key <= endDate;
        };
        const updates = demoMondayUpdates();
        return wait({
          attempts: uniqueById(
            updates.flatMap((update) => update.activity.attempts),
          ).filter((attempt) => inRange(attempt.attemptedAt)),
          mocks: uniqueById(
            updates.flatMap((update) => update.activity.mocks),
          ).filter((mock) => inRange(mock.occurredAt)),
        });
      }
      return apiRequest<MondayUpdateActivity>(
        `/seasons/${encodeURIComponent(seasonId!)}/monday-update-activity?startDate=${encodeURIComponent(startDate)}&endDate=${encodeURIComponent(endDate)}`,
      );
    },
  });
}

export function useAttempts(
  enabled = true,
  seasonId?: string,
  filters: ActivityFilters = emptyActivityFilters,
) {
  return useQuery({
    queryKey: ['problem-attempts', { seasonId, filters }],
    enabled,
    queryFn: async () => {
      if (demoMode)
        return wait(
          page(
            attempts
              .filter((item) => {
                const season = seasons.find((item) => item.id === seasonId);
                return (
                  !seasonId ||
                  Boolean(
                    season &&
                    item.attemptedAt >= season.startsAt &&
                    item.attemptedAt <= season.endsAt,
                  )
                );
              })
              .map((item) => ({
                ...item,
                catalogProblem: demoProblemById.get(item.problemId),
                weekId: demoActivityWeekId(seasonId, item.attemptedAt),
              })),
          ),
        );
      const adapted = adaptAttemptPage(
        await fetchAllPages<ApiAttemptPage['items'][number]>(
          `/problem-attempts?limit=100${seasonId ? `&seasonId=${encodeURIComponent(seasonId)}` : ''}${activityFilterQuery(filters)}`,
        ),
      );
      return {
        ...adapted,
        items: [...adapted.items].sort((left, right) =>
          right.attemptedAt.localeCompare(left.attemptedAt),
        ),
      };
    },
  });
}

export function usePeople(enabled = true) {
  return useQuery({
    queryKey: ['people'],
    enabled,
    queryFn: async () =>
      demoMode ? wait(page(people)) : adaptUserPage(await getUsers()),
  });
}

export function useAdminUsers(enabled = true) {
  return useQuery({
    queryKey: ['admin-users'],
    enabled,
    queryFn: async (): Promise<ApiPage<UserPrivate>> => {
      if (!demoMode)
        return fetchAllPages<UserPrivate>('/admin/users?limit=100');
      return wait(
        page(
          people.map((person) => ({
            id: person.id,
            slug: person.slug,
            name: person.name,
            avatarUrl: person.avatarUrl,
            timezone: 'Australia/Adelaide',
            timezoneConfigured: true,
            globalRoles: person.roles.filter(
              (role): role is UserPrivate['globalRoles'][number] =>
                role === 'director' || role === 'system_admin',
            ),
            seasonRoles: [],
            attemptCount: person.attempts ?? 0,
            mockInterviewCount: person.interviews ?? 0,
            email: person.email ?? `${person.slug}@example.test`,
            accountState: 'active',
          })),
        ),
      );
    },
  });
}

export function useMockParticipants() {
  return useQuery({
    queryKey: ['mock-interview-participants'],
    queryFn: async () =>
      demoMode
        ? wait(page(people))
        : adaptMockParticipantPage(
            await fetchAllPages<MockParticipantSummaryPage['items'][number]>(
              '/mock-interviews/participants?limit=100',
            ),
          ),
  });
}

export function useUserProfile(identifier?: string, seasonId?: string) {
  return useQuery({
    queryKey: ['users', identifier, { seasonId }],
    enabled: Boolean(identifier),
    queryFn: async () => {
      if (!identifier) return null;
      if (demoMode)
        return wait(
          people.find(
            (person) => person.id === identifier || person.slug === identifier,
          ) ?? null,
        );
      return adaptPerson(
        await apiRequest<ApiUser>(
          `/users/${encodeURIComponent(identifier)}${seasonId ? `?seasonId=${encodeURIComponent(seasonId)}` : ''}`,
        ),
      );
    },
  });
}

export function useUserAttempts(
  userId?: string,
  enabled = true,
  seasonId?: string,
  filters: ActivityFilters = emptyActivityFilters,
) {
  return useQuery({
    queryKey: ['problem-attempts', 'user', userId, { seasonId, filters }],
    enabled: Boolean(userId) && enabled,
    queryFn: async () => {
      if (!userId) return page([]);
      if (demoMode)
        return wait(
          page(
            attempts
              .filter((attempt) => {
                const season = seasons.find((item) => item.id === seasonId);
                return (
                  !seasonId ||
                  Boolean(
                    season &&
                    attempt.attemptedAt >= season.startsAt &&
                    attempt.attemptedAt <= season.endsAt,
                  )
                );
              })
              .map((attempt) => ({
                ...attempt,
                catalogProblem: demoProblemById.get(attempt.problemId),
                weekId: demoActivityWeekId(seasonId, attempt.attemptedAt),
              })),
          ),
        );
      const adapted = adaptAttemptPage(
        await fetchAllPages<ApiAttemptPage['items'][number]>(
          `/problem-attempts?limit=100&userId=${encodeURIComponent(userId)}${seasonId ? `&seasonId=${encodeURIComponent(seasonId)}` : ''}${activityFilterQuery(filters)}`,
        ),
      );
      return {
        ...adapted,
        items: [...adapted.items].sort((left, right) =>
          right.attemptedAt.localeCompare(left.attemptedAt),
        ),
      };
    },
  });
}

export function useSeasonPeople(seasonId?: string, seasonName?: string) {
  return useQuery<Page<Person>>({
    queryKey: ['season-people', seasonId],
    enabled: demoMode || Boolean(seasonId),
    queryFn: async () => {
      if (demoMode) {
        if (!seasonId) return page([]);
        const enrollments = demoEnrollmentPage(seasonId).items;
        const mentorships = demoMentorshipPage(seasonId).items;
        return wait(
          page(
            enrollments.flatMap((enrollment) => {
              const person = people.find(
                (item) => item.id === enrollment.userId,
              );
              if (!person) return [];
              return [
                {
                  ...person,
                  season: seasonName ?? person.season,
                  enrollmentId: enrollment.id,
                  enrollmentState: enrollment.state,
                  studentLevel: enrollment.studentLevel,
                  seasonRole: enrollment.role,
                  mentorshipMentorId: mentorships.find(
                    (item) => item.studentUserId === person.id,
                  )?.mentorUserId,
                },
              ];
            }),
          ),
        );
      }
      if (!seasonId) return page([]);
      const [enrollmentPage, mentorshipPage] = await Promise.all([
        fetchAllPages<Enrollment>(
          `/seasons/${encodeURIComponent(seasonId)}/members?limit=100`,
        ),
        fetchAllPages<Mentorship>(
          `/seasons/${encodeURIComponent(seasonId)}/mentorships?limit=100&includeEnded=true`,
        ),
      ]);
      const mentorForStudent = new Map(
        mentorshipPage.items
          .filter((mentorship) => !mentorship.endedAt)
          .map((mentorship) => [
            mentorship.studentUserId,
            mentorship.mentorUserId,
          ]),
      );
      const assignedStudents = new Set(mentorForStudent.keys());
      const items = enrollmentPage.items.flatMap((enrollment) => {
        const member = enrollment.member;
        if (!member) return [];
        const user = {
          id: member.id,
          name: member.name,
          slug: member.slug,
          initials: member.name
            .split(' ')
            .map((part) => part[0])
            .slice(0, 2)
            .join(''),
          avatarUrl: member.avatarUrl,
          roles: [enrollment.role],
          attempts: member.activity.attemptCount,
          interviews: member.activity.mockInterviewCount,
          mocksReceived: member.activity.mocksReceived,
          mocksConducted: member.activity.mocksConducted,
          lastActiveAt: member.activity.lastActivityAt,
        };
        return [
          {
            ...user,
            roles: [...new Set([...user.roles, enrollment.role])],
            season: seasonName ?? null,
            status:
              enrollment.state === 'completed'
                ? ('completed' as const)
                : enrollment.state === 'active' &&
                    enrollment.role === 'student' &&
                    !assignedStudents.has(enrollment.userId)
                  ? ('unassigned' as const)
                  : enrollment.state === 'active'
                    ? ('active' as const)
                    : enrollment.state,
            previousMentorIds: mentorshipPage.items
              .filter((m) => m.studentUserId === enrollment.userId)
              .map((m) => m.mentorUserId),
            enrollmentId: enrollment.id,
            enrollmentState: enrollment.state,
            studentLevel: enrollment.studentLevel,
            seasonRole: enrollment.role,
            ...(mentorForStudent.has(enrollment.userId)
              ? { mentorshipMentorId: mentorForStudent.get(enrollment.userId) }
              : {}),
          },
        ];
      });
      return page(items);
    },
  });
}

export type MockInterviewMode = 'received' | 'given' | 'all';

export interface MockInterviewQuery {
  /** received and given are the caller's own; all is every visible interview. */
  mode?: MockInterviewMode;
  /** Narrows to one person's interviews in either direction. */
  userId?: string;
  seasonId?: string;
  filters?: ActivityFilters;
}

const mockInterviewPageSize = 100;

function mockInterviewPath(
  {
    mode = 'all',
    userId,
    seasonId,
    filters = emptyActivityFilters,
  }: MockInterviewQuery,
  cursor?: string | null,
) {
  return `/mock-interviews?limit=${mockInterviewPageSize}&mode=${mode}${userId ? `&userId=${encodeURIComponent(userId)}` : ''}${seasonId ? `&seasonId=${encodeURIComponent(seasonId)}` : ''}${activityFilterQuery(filters)}${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''}`;
}

function demoMockInterviewPage({
  userId,
  seasonId,
}: MockInterviewQuery): Page<MockInterview> {
  const season = seasons.find((item) => item.id === seasonId);
  return page(
    mockInterviews
      .filter(
        (item) =>
          (!userId ||
            item.interviewee.id === userId ||
            item.interviewer.id === userId) &&
          (!seasonId ||
            Boolean(
              season &&
              item.occurredAt >= season.startsAt &&
              item.occurredAt <= season.endsAt,
            )),
      )
      .map((item) => ({
        ...item,
        rounds: item.rounds.map((round) => ({
          ...round,
          catalogProblem: round.problemId
            ? demoProblemById.get(round.problemId)
            : undefined,
        })),
        weekId: demoActivityWeekId(seasonId, item.occurredAt),
      })),
  );
}

/**
 * Resolves season names from the cached season list instead of downloading
 * it again with every interview request.
 */
function useAdaptMockInterviews() {
  const queryClient = useQueryClient();
  return async (source: ApiMockInterviewPage) => {
    const knownSeasons = await queryClient.ensureQueryData(seasonsOptions);
    return adaptMockInterviewPage(source, {
      users: new Map(),
      seasons: new Map(knownSeasons.items.map((season) => [season.id, season])),
    });
  };
}

/**
 * One person's interviews: the caller's received or given interviews, or a
 * single user's history. These stay small, so every page is loaded.
 * Use useSharedMockInterviews for the platform-wide list.
 */
export function useMockInterviews(
  query: MockInterviewQuery & { enabled?: boolean },
) {
  const { enabled = true, ...request } = query;
  const adapt = useAdaptMockInterviews();
  return useQuery({
    queryKey: ['mock-interviews', 'scoped', request],
    enabled,
    queryFn: async () => {
      if (demoMode) return wait(demoMockInterviewPage(request));
      const adapted = await adapt(
        await fetchAllPages<ApiMockInterviewPage['items'][number]>(
          mockInterviewPath(request),
        ),
      );
      return {
        ...adapted,
        items: [...adapted.items].sort((left, right) =>
          right.occurredAt.localeCompare(left.occurredAt),
        ),
      };
    },
  });
}

/**
 * Every interview the caller may read, newest first, one page at a time.
 * The next page is requested only through fetchNextPage.
 */
export function useSharedMockInterviews(
  enabled: boolean,
  seasonId?: string,
  filters: ActivityFilters = emptyActivityFilters,
) {
  const adapt = useAdaptMockInterviews();
  return useInfiniteQuery({
    queryKey: ['mock-interviews', 'shared', { seasonId, filters }],
    enabled,
    initialPageParam: null as string | null,
    queryFn: async ({ pageParam }): Promise<Page<MockInterview>> => {
      if (demoMode) return wait(demoMockInterviewPage({ seasonId }));
      return adapt(
        await apiRequest<ApiMockInterviewPage>(
          mockInterviewPath({ mode: 'all', seasonId, filters }, pageParam),
        ),
      );
    },
    getNextPageParam: (last) =>
      last.pageInfo.hasMore
        ? (last.pageInfo.nextCursor ?? undefined)
        : undefined,
  });
}

export function useActivitySummary(userId?: string, seasonId?: string) {
  return useQuery({
    queryKey: ['activity-summary', { userId, seasonId }],
    enabled: Boolean(userId || seasonId),
    queryFn: async (): Promise<ActivitySummary> => {
      if (demoMode) {
        const matchingAttempts = attempts.filter(
          (a) => !seasonId || a.seasonId === seasonId,
        );
        const matchingMocks = mockInterviews.filter(
          (m) =>
            (!seasonId || m.seasonId === seasonId) &&
            (!userId ||
              m.interviewee.id === userId ||
              m.interviewer.id === userId),
        );
        const dates = [
          ...matchingAttempts.map((a) => a.attemptedAt),
          ...matchingMocks.map((m) => m.occurredAt),
        ].sort();
        return {
          attemptCount: matchingAttempts.length,
          mockInterviewCount: matchingMocks.length,
          mocksReceived: matchingMocks.filter(
            (m) => !userId || m.interviewee.id === userId,
          ).length,
          mocksConducted: matchingMocks.filter(
            (m) => !userId || m.interviewer.id === userId,
          ).length,
          lastActivityAt: dates.at(-1) ?? null,
        };
      }
      return apiRequest<ActivitySummary>(
        userId
          ? `/users/${encodeURIComponent(userId)}/activity-summary${seasonId ? `?seasonId=${encodeURIComponent(seasonId)}` : ''}`
          : `/seasons/${encodeURIComponent(seasonId!)}/activity-summary`,
      );
    },
  });
}

export function useUserParticipation(
  userId?: string,
  enabled = true,
  seasonId?: string,
) {
  return useQuery({
    queryKey: ['participation', userId, { seasonId }],
    enabled: Boolean(userId) && enabled,
    queryFn: async (): Promise<Page<Participation>> => {
      if (demoMode)
        return page(
          seasons.flatMap((season) =>
            demoEnrollmentPage(season.id)
              .items.filter((e) => e.userId === userId)
              .map((e) => ({
                ...e,
                seasonName: season.name,
                seasonSlug: season.slug,
                startAt: season.startsAt,
                endAt: season.endsAt,
                lastStudentLevel: e.studentLevel,
                periods: [
                  {
                    role: e.role,
                    startedAt: season.startsAt,
                    endedAt: season.status === 'closed' ? season.endsAt : null,
                  },
                ],
              })),
          ),
        );
      return apiRequest<Page<Participation>>(
        `/users/${encodeURIComponent(userId!)}/participation${seasonId ? `?seasonId=${encodeURIComponent(seasonId)}` : ''}`,
      );
    },
  });
}
