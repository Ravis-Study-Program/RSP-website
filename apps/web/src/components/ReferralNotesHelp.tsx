import { useState } from 'react';
import { Tooltip } from '@base-ui/react/tooltip';
import { IconInfoCircle } from '@tabler/icons-react';

export function ReferralNotesHelp() {
  const [open, setOpen] = useState(false);
  return (
    <Tooltip.Provider>
      <Tooltip.Root open={open} onOpenChange={setOpen}>
        <Tooltip.Trigger
          type="button"
          aria-label="What to include in referral notes"
          closeOnClick={false}
          onClick={() => setOpen(true)}
          className="inline-flex size-7 shrink-0 items-center justify-center rounded-sm text-muted-foreground hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
        >
          <IconInfoCircle size={16} aria-hidden="true" />
        </Tooltip.Trigger>
        <Tooltip.Portal>
          <Tooltip.Positioner sideOffset={6} className="z-50">
            <Tooltip.Popup
              role="tooltip"
              className="max-w-72 rounded-lg border bg-popover p-3 text-sm leading-relaxed text-popover-foreground"
            >
              Add requirements such as specific roles, GPA thresholds, or
              instructions like “Message me your resume”. Students must meet all
              your requirements before contacting you.
            </Tooltip.Popup>
          </Tooltip.Positioner>
        </Tooltip.Portal>
      </Tooltip.Root>
    </Tooltip.Provider>
  );
}
