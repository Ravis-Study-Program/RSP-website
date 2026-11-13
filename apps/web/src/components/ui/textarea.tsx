import * as React from 'react';
import { cn } from '@/lib/utils';

function Textarea({
  className,
  disabled,
  onPointerDown,
  ...props
}: React.ComponentProps<'textarea'>) {
  return (
    <textarea
      disabled={disabled}
      data-slot="textarea"
      className={cn(
        'flex field-sizing-content min-h-16 w-full rounded-lg border border-input bg-transparent px-2.5 py-2 text-base transition-shadow outline-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 read-only:cursor-not-allowed disabled:cursor-not-allowed disabled:select-none disabled:bg-input/50 disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-3 aria-invalid:ring-destructive/20 md:text-sm dark:bg-input/30 dark:disabled:bg-input/80 dark:aria-invalid:border-destructive/50 dark:aria-invalid:ring-destructive/40',
        className,
      )}
      onPointerDown={(event) => {
        if (disabled) event.preventDefault();
        onPointerDown?.(event);
      }}
      {...props}
    />
  );
}

export { Textarea };
