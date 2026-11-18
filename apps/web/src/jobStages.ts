import type { JobApplication } from '@/api/generated/models';

export const recommendedJobStages = [
  'Recruiter screen (confirm info)',
  'Recruiter screen (behavioural)',
  'Hiring manager screen',
  'Online assessment',
  'Take-home assignment',
  'Technical (LeetCode style)',
  'Technical (OOP design)',
  'Technical (system design)',
  'Technical (live coding)',
  'Technical (debugging)',
  'Technical (project discussion)',
  'Behavioural interview',
  'Panel interview',
  'Final interview',
  'Team matching',
];

export const jobStageKey = (title: string) =>
  title.trim().replace(/\s+/g, ' ').toLowerCase();

const reservedStageNames = new Set([
  '',
  'applied',
  'applied (merged listing)',
  'active',
  'rejected',
  'offer',
]);

export const isInterviewStage = (title: string) =>
  !reservedStageNames.has(jobStageKey(title));

export function currentJobStage(application: JobApplication) {
  if (application.status === 'inactive') return 'Rejected';
  if (application.status === 'offer') return 'Offer';
  for (let i = application.stages.length - 1; i >= 0; i--) {
    const stage = application.stages[i];
    if (isInterviewStage(stage.title)) return stage.title;
  }
  return 'Applied';
}
