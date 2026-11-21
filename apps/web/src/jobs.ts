import type {
  JobApplicationStatus,
  JobCategory,
  JobPosition,
  JobPositionInput,
  JobType,
} from '@/api/generated/models';
import type { CurrentUser } from '@/types';

export const maxReferralRequests = 5;

export function isDriveFileURL(value: string) {
  const url = value.trim();
  return (
    url.length <= 2048 &&
    /^https:\/\/drive\.google\.com\/(file\/d\/[A-Za-z0-9_-]+(\/(view|preview))?\/?(\?[^#\s\p{Cc}]*)?|open\?id=[A-Za-z0-9_-]+(&[^#\s\p{Cc}]*)?)$/u.test(
      url,
    )
  );
}

export function canAccessJobReferrals(user?: CurrentUser) {
  return Boolean(
    user?.emailVerified &&
    user.accountState === 'active' &&
    (user.alumni ||
      user.seasonRoles.some(
        ({ state }) => state === 'active' || state === 'completed',
      )),
  );
}

export function canRequestJobReferral(user: CurrentUser) {
  return (
    user.alumni ||
    user.seasonRoles.some(
      ({ role, state }) =>
        role === 'student' && (state === 'active' || state === 'completed'),
    )
  );
}

export const jobApplicationStatuses: {
  value: JobApplicationStatus;
  label: string;
}[] = [
  { value: 'active', label: 'Active' },
  { value: 'inactive', label: 'Rejected' },
  { value: 'offer', label: 'Offer' },
];

export const jobCategories: { value: JobCategory; label: string }[] = [
  { value: 'faang', label: 'FAAMG' },
  { value: 'big_tech', label: 'Big Tech' },
  { value: 'quant', label: 'HFT/Quant' },
  { value: 'other', label: 'Other Tech' },
];
export const jobTypes: { value: JobType; label: string }[] = [
  { value: 'intern', label: 'Intern' },
  { value: 'grad', label: 'Grad' },
  { value: 'experienced', label: 'Experienced' },
];
export const jobCategoryBadgeClasses: Record<JobCategory, string> = {
  faang: 'bg-job-faamg text-job-category-foreground',
  big_tech: 'bg-job-big-tech text-job-category-foreground',
  quant: 'bg-job-quant text-job-category-foreground',
  other: 'bg-job-other text-job-category-foreground',
};
export const jobCategoryControlClasses: Record<JobCategory, string> = {
  faang: `${jobCategoryBadgeClasses.faang} hover:bg-job-faamg/90 aria-expanded:bg-job-faamg dark:hover:bg-job-faamg/90`,
  big_tech: `${jobCategoryBadgeClasses.big_tech} hover:bg-job-big-tech/90 aria-expanded:bg-job-big-tech dark:hover:bg-job-big-tech/90`,
  quant: `${jobCategoryBadgeClasses.quant} hover:bg-job-quant/90 aria-expanded:bg-job-quant dark:hover:bg-job-quant/90`,
  other: `${jobCategoryBadgeClasses.other} hover:bg-job-other/90 aria-expanded:bg-job-other dark:hover:bg-job-other/90`,
};
export function canManageJobs(user?: CurrentUser) {
  return Boolean(
    user?.globalRoles.some((r) => r === 'director' || r === 'system_admin') ||
    user?.seasonRoles.some(
      (r) => r.role === 'coordinator' && r.state === 'active',
    ),
  );
}
export function localJobDate() {
  const date = new Date();
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}
export function positionInput(p: JobPosition): JobPositionInput {
  return {
    companyName: p.company.name,
    category: p.company.category,
    title: p.title,
    jobType: p.jobType,
    eligibility: p.eligibility,
    applicationUrl: p.applicationUrl,
    locations: p.locations,
    notes: p.notes,
    status: p.status,
  };
}
