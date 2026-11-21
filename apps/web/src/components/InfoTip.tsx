import { IconInfoCircle } from '@tabler/icons-react';
import type { ReactNode } from 'react';

import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import { cn } from '@/lib/utils';

/**
 * A light grey (i) beside a label that explains it. It opens on hover and
 * stays open on click or tap, so the text is reachable without a mouse.
 */
export function InfoTip({
  label,
  children,
  size = 15,
  contentClassName,
}: {
  /** The button's accessible name, such as "What to include". */
  label: string;
  /** The explanation; defaults to the label itself. */
  children?: ReactNode;
  size?: number;
  contentClassName?: string;
}) {
  return (
    <Popover>
      <PopoverTrigger
        openOnHover
        delay={150}
        aria-label={label}
        className="inline-flex rounded-sm text-muted-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        <IconInfoCircle size={size} aria-hidden="true" />
      </PopoverTrigger>
      <PopoverContent
        align="start"
        className={cn('w-72 text-sm text-muted-foreground', contentClassName)}
      >
        {children ?? label}
      </PopoverContent>
    </Popover>
  );
}
