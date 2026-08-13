export type GlobalRole = 'director' | 'system_admin';
export type SeasonRole = 'student' | 'mentor' | 'coordinator';
export type Role = GlobalRole | SeasonRole | 'graduate';
export type SeasonStatus = 'open' | 'closed';
export type Difficulty = 'Easy' | 'Medium' | 'Hard';
export type AttemptOutcome =
  | 'independently_solved'
  | 'solved_with_hints'
  | 'not_solved'
  | 'unknown';

export interface CurrentUser {
  id: string;
  name: string;
  slug: string;
  email: string;
  timezone: string;
  emailVerified: boolean;
  mfaVerified: boolean;
  globalRoles: GlobalRole[];
  seasonRoles: Array<{ seasonId: string; seasonSlug: string; role: SeasonRole }>;
  alumni: boolean;
  revision: number;
}

export interface Season {
  id: string;
  slug: string;
  name: string;
  status: SeasonStatus;
  startsAt: string;
  endsAt: string;
  memberCount: number;
  weekCount: number;
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
  problem: string;
  difficulty: Difficulty;
  category: string;
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
  roles: Role[];
  season: string;
  status: 'active' | 'completed' | 'unassigned';
  attempts: number;
  interviews: number;
  email?: string;
  lastActiveAt: string;
}

export interface MockRound {
  id: string;
  type: 'technical' | 'behavioural';
  title: string;
  score: number;
  notes: string;
}

export interface MockInterview {
  id: string;
  interviewee: Person;
  interviewer: Person;
  season: string;
  occurredAt: string;
  durationMinutes: number;
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
