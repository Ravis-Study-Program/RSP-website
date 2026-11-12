import type {
  JobApplicationStatus,
  JobCategory,
  JobPosition,
  JobPositionInput,
  JobType,
} from '@/api/generated/models';
import type { CurrentUser } from '@/types';

export const jobApplicationStatuses: {
  value: JobApplicationStatus;
  label: string;
}[] = [
  { value: 'active', label: 'Active' },
  { value: 'inactive', label: 'Rejected' },
  { value: 'offer', label: 'Offer' },
];

export const jobCategories: { value: JobCategory; label: string }[] = [
  { value: 'faang', label: 'FAANG' },
  { value: 'big_tech', label: 'Big Tech' },
  { value: 'quant', label: 'HFT/Quant' },
  { value: 'other', label: 'Other Tech' },
];
export const jobTypes: { value: JobType; label: string }[] = [
  { value: 'intern', label: 'Intern' },
  { value: 'grad', label: 'Grad' },
  { value: 'experienced', label: 'Experienced' },
];
export const jobCategoryControlClasses: Record<JobCategory, string> = {
  faang:
    'bg-destructive hover:bg-destructive/90 aria-expanded:bg-destructive dark:hover:bg-destructive/90',
  big_tech:
    'bg-orange hover:bg-orange/90 aria-expanded:bg-orange dark:hover:bg-orange/90',
  quant:
    'bg-warning hover:bg-warning/90 aria-expanded:bg-warning dark:hover:bg-warning/90',
  other: 'bg-pink hover:bg-pink/90 aria-expanded:bg-pink dark:hover:bg-pink/90',
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
