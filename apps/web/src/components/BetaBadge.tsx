import { StatusBadge } from '@/components/StatusBadge';

export function BetaBadge({ compact = false }: { compact?: boolean }) {
  return (
    <StatusBadge
      tone="muted"
      className={compact ? 'h-4 px-1 text-[0.625rem]' : undefined}
    >
      Beta
    </StatusBadge>
  );
}
