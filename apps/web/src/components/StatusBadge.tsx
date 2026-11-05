import type { ComponentProps } from 'react';
import { Badge } from '@/components/ui/badge';
import type { BadgeTone } from '@/badgeTones';
import { cn } from '@/lib/utils';

const toneClass: Record<BadgeTone, string> = {
  neutral: 'bg-secondary text-secondary-foreground',
  muted: 'bg-muted text-muted-foreground',
  success: 'bg-success-soft text-success',
  warning: 'bg-warning-soft text-warning',
  danger: 'bg-destructive-soft text-destructive',
  info: 'bg-info-soft text-info',
  purple: 'bg-purple-soft text-purple',
  pink: 'bg-pink-soft text-pink',
  orange: 'bg-orange-soft text-orange',
  strongSuccess: 'bg-success text-background',
  strongDanger: 'bg-destructive text-background',
};

/**
 * Every inline badge in the app: a small, square-cornered, bold label in a
 * tone, as on the LeetCode and Mock Interviews tables.
 */
export function StatusBadge({
  tone = 'neutral',
  className,
  ...props
}: Omit<ComponentProps<typeof Badge>, 'variant'> & { tone?: BadgeTone }) {
  return (
    <Badge
      variant="outline"
      className={cn(
        'rounded-sm border-transparent font-semibold',
        toneClass[tone],
        className,
      )}
      {...props}
    />
  );
}
