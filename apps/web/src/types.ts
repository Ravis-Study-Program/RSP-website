export type GlobalRole = 'director' | 'system_admin';
export type SeasonRole = 'student' | 'mentor' | 'coordinator';
export type Role = GlobalRole | SeasonRole | 'graduate' | 'former_member';
export type SeasonStatus = 'open' | 'closed';
export type Difficulty = 'Easy' | 'Medium' | 'Hard';
export type AttemptOutcome =
  'independently_solved' | 'solved_with_hints' | 'not_solved' | 'unknown';

export interface CurrentUser {
  id: string;
  name: string;
  slug: string;
  avatarUrl: string | null;
  email: string;
  timezone: string;
  timezoneConfigured: boolean;
  emailVerified: boolean;
  mfaVerified: boolean;
  accountState: 'active' | 'suspended' | 'deletion_pending' | 'deleted';
  globalRoles: GlobalRole[];
  seasonRoles: Array<{
    seasonId: string;
    seasonSlug: string;
    role: SeasonRole;
    state: 'active' | 'completed' | 'kicked' | 'withdrawn';
  }>;
  alumni: boolean;
  attemptCount: number;
  mockInterviewCount: number;
  revision: number;
}

export interface Season {
  id: string;
  slug: string;
  name: string;
  status: SeasonStatus;
  startsAt: string;
  endsAt: string;
  location: string;
  imageUrl: string;
  resourcesUrl: string;
  memberCount: number | null;
  weekCount: number | null;
  summary: string;
  revision: number;
}

export interface Recommendation {
  id: string;
  problemId: string;
  title: string;
  difficulty: Difficulty;
  category: string;
  rationale: string;
  estimatedMinutes: number;
  externalUrl: string;
  active: boolean;
}

export interface Attempt {
  id: string;
  problemId: string;
  problem: string;
  difficulty: Difficulty | null;
  category: string | null;
  outcome: AttemptOutcome;
  confidence: number | null;
  minutes: number | null;
  attemptedAt: string;
  notes: string;
  revision: number;
}

export interface Person {
  id: string;
  name: string;
  slug: string;
  initials: string;
  avatarUrl: string | null;
  roles: Role[];
  season: string | null;
  status: 'active' | 'completed' | 'unassigned' | null;
  attempts: number | null;
  interviews: number | null;
  email?: string;
  lastActiveAt: string | null;
  revision?: number;
  enrollmentId?: string;
  enrollmentRevision?: number;
  enrollmentState?: 'active' | 'completed' | 'kicked' | 'withdrawn';
  mentorshipMentorId?: string;
}

export interface MockRound {
  id: string;
  type: 'technical' | 'behavioural';
  apiType: 'behavioural' | 'leetcode' | 'custom';
  problemId?: string;
  link?: string;
  title: string;
  score: number | null;
  scores: Record<string, number>;
  notes: string;
  reviewed: boolean;
  intervieweeComment: string;
}

export interface MockInterview {
  id: string;
  interviewee: Person;
  interviewer: Person;
  season: string | null;
  occurredAt: string;
  durationMinutes: number;
  notes: string;
  rounds: MockRound[];
  reviewStatus: 'pending' | 'reviewed';
  reviewComments?: string;
  revision: number;
}

export interface PageInfo {
  nextCursor: string | null;
  previousCursor: string | null;
  hasMore: boolean;
}

export interface Page<T> {
  items: T[];
  pageInfo: PageInfo;
  totalCount: number;
}

export interface ProblemDetails {
  type: string;
  title: string;
  status: number;
  detail?: string;
  instance?: string;
  code: string;
  requestId: string;
  errors?: Array<{ field: string; message: string }>;
}
