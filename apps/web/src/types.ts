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
}

export interface Attempt {
  seasonId?: string | null;
  id: string;
  problemId: string;
  problem: string;
  problemUrl?: string;
  difficulty: Difficulty | null;
  category: string | null;
  categories?: string[];
  weekId?: string | null;
  outcome: AttemptOutcome;
  confidence: number | null;
  minutes: number | null;
  attemptedAt: string;
  notes: string;
}

export interface Person {
  id: string;
  name: string;
  slug: string;
  initials: string;
  avatarUrl: string | null;
  roles: Role[];
  season: string | null;
  status: 'active' | 'completed' | 'unassigned' | 'kicked' | 'withdrawn' | null;
  attempts: number | null;
  interviews: number | null;
  mocksReceived?: number;
  mocksConducted?: number;
  email?: string;
  lastActiveAt: string | null;
  enrollmentId?: string;
  seasonRole?: SeasonRole;
  studentLevel?:
    'novice' | 'beginner' | 'intermediate' | 'advanced' | 'not_applicable';
  enrollmentState?: 'active' | 'completed' | 'kicked' | 'withdrawn';
  mentorshipMentorId?: string;
  previousMentorIds?: string[];
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
  seasonId?: string | null;
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
