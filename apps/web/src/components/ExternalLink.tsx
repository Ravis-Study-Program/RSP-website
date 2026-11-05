import { IconExternalLink } from '@tabler/icons-react';
import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

/** A link that opens in a new tab, marked with an external-link icon. */
export function ExternalLink({
  href,
  className,
  children,
}: {
  href: string;
  className?: string;
  children: ReactNode;
}) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className={cn('inline-flex items-center gap-1', className)}
    >
      {children}
      <IconExternalLink
        size={14}
        aria-hidden="true"
        className="shrink-0 text-muted-foreground"
      />
    </a>
  );
}
