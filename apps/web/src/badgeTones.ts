/** The colours a badge can take, from statuses to seasons and levels. */
export type BadgeTone =
  | 'neutral'
  | 'muted'
  | 'success'
  | 'warning'
  | 'danger'
  | 'info'
  | 'purple'
  | 'pink'
  | 'orange'
  | 'strongSuccess'
  | 'strongDanger';

const positive = new Set(['active', 'open']);
const pending = new Set(['pending', 'pending_mfa', 'unassigned']);
const negative = new Set([
  'kicked',
  'withdrawn',
  'suspended',
  'deleted',
  'deletion_pending',
  'revoked',
]);

/**
 * The tone for a status: positive ones are green, pending ones yellow,
 * negative ones red, and finished ones (completed, closed) grey.
 */
export function statusTone(status?: string | null): BadgeTone {
  if (!status) return 'muted';
  if (positive.has(status)) return 'success';
  if (pending.has(status)) return 'warning';
  if (negative.has(status)) return 'danger';
  return 'muted';
}
