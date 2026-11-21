import { IconExternalLink } from '@tabler/icons-react';
import type { ComponentProps } from 'react';
import { cn } from '@/lib/utils';

/** A link that opens in a new tab, marked with an external-link icon. */
export function ExternalLink({
  href,
  className,
  children,
  ...props
}: Omit<ComponentProps<'a'>, 'href' | 'target' | 'rel'> & { href: string }) {
  return (
    <a
      {...props}
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className={cn(
        'inline-flex max-w-full items-center gap-1 underline decoration-[0.08em] underline-offset-[0.2em] hover:decoration-[0.14em]',
        className,
      )}
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
