import { Alert } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { IconAlertTriangle, IconInbox, IconRefresh } from '@tabler/icons-react';

import { cn } from '@/lib/utils';

const stateClass =
  'mx-auto grid min-h-60 w-full place-items-center p-8 text-center';
const statusContentClass = 'grid max-w-[34rem] justify-items-center gap-2';

export function PageSkeleton({ label = 'Loading page' }: { label?: string }) {
  return (
    <div
      className="mx-auto w-full max-w-[92rem]"
      aria-busy="true"
      aria-label={label}
    >
      <Skeleton className="h-3.5 w-32 text-transparent">Loading</Skeleton>
      <Skeleton className="mt-3 h-10 w-[min(28rem,85%)] text-transparent">
        Loading
      </Skeleton>
      <div className="mt-4 grid grid-cols-1 gap-4 min-[56rem]:grid-cols-2 min-[70rem]:grid-cols-3">
        {[1, 2, 3].map((item) => (
          <Skeleton key={item} className="h-32 text-transparent">
            Loading
          </Skeleton>
        ))}
      </div>
      <Skeleton className="mt-4 h-72 text-transparent">Loading</Skeleton>
      <span className="sr-only" role="status">
        {label}
      </span>
    </div>
  );
}

export function EmptyState({
  title,
  message,
  filtered = false,
  action,
}: {
  title: string;
  message: string;
  filtered?: boolean;
  action?: React.ReactNode;
}) {
  return (
    <div className={stateClass}>
      <div className={statusContentClass}>
        <IconInbox
          size={30}
          aria-hidden="true"
          className="text-muted-foreground"
        />
        <h2 className="text-xl font-semibold tracking-tight">
          {filtered ? `No matches for ${title.toLowerCase()}` : title}
        </h2>
        <p className="text-sm text-muted-foreground">{message}</p>
        {action}
      </div>
    </div>
  );
}

export function ErrorState({
  title = 'We could not load this section',
  message = 'The rest of your workspace is still available. Try this request again.',
  onRetry,
}: {
  title?: string;
  message?: string;
  onRetry?: () => void;
}) {
  return (
    <div className={stateClass} role="alert">
      <div className={statusContentClass}>
        <IconAlertTriangle
          size={30}
          aria-hidden="true"
          className="text-muted-foreground"
        />
        <h2 className="text-xl font-semibold tracking-tight">{title}</h2>
        <p className="text-sm text-muted-foreground">{message}</p>
        {onRetry ? (
          <Button variant="outline" type="button" onClick={onRetry}>
            <IconRefresh size={18} aria-hidden="true" /> Retry
          </Button>
        ) : null}
      </div>
    </div>
  );
}

const noticeToneClass: Record<
  'info' | 'warning' | 'danger' | 'success',
  string
> = {
  info: 'bg-muted text-foreground',
  warning: 'bg-warning-soft text-warning',
  danger: 'bg-destructive-soft text-destructive',
  success: 'bg-success-soft text-success',
};

export function InlineNotice({
  children,
  tone = 'info',
}: {
  children: React.ReactNode;
  tone?: 'info' | 'warning' | 'danger' | 'success';
}) {
  return (
    <Alert
      className={cn('text-sm', noticeToneClass[tone])}
      role={tone === 'danger' ? 'alert' : 'status'}
    >
      {children}
    </Alert>
  );
}
