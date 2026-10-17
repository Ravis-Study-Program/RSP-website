import { format, parse } from 'date-fns';
import { CalendarIcon } from 'lucide-react';
import { useId, useState } from 'react';
import type { DateRange } from 'react-day-picker';

import { Button } from '@/components/ui/button';
import { Calendar } from '@/components/ui/calendar';
import { Field, FieldLabel } from '@/components/ui/field';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';

export interface CalendarDateRange {
  startDate: string; // YYYY-MM-DD
  endDate: string; // YYYY-MM-DD
}

const dateKeyFormat = 'yyyy-MM-dd';

/**
 * shadcn range picker over calendar-date keys. With a range already chosen, a
 * click starts a new range; the second click completes it and closes the
 * popover. Only complete ranges reach `onChange`, so the value is never
 * reversed or half-set.
 */
export function DateRangePicker({
  label,
  value,
  onChange,
}: {
  label: string;
  value: CalendarDateRange;
  onChange: (value: CalendarDateRange) => void;
}) {
  const id = useId();
  const [open, setOpen] = useState(false);
  const committed: DateRange = {
    from: parse(value.startDate, dateKeyFormat, new Date()),
    to: parse(value.endDate, dateKeyFormat, new Date()),
  };
  // A one-day draft while the second click is pending.
  const [draft, setDraft] = useState<DateRange | undefined>();
  const display = (date: Date) => format(date, 'd MMM yyyy');

  return (
    <Field className="w-full max-w-80">
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <Popover
        open={open}
        onOpenChange={(next) => {
          setOpen(next);
          setDraft(undefined);
        }}
      >
        <PopoverTrigger
          render={
            <Button
              variant="outline"
              id={id}
              className="justify-start px-2.5 font-normal"
            />
          }
        >
          <CalendarIcon data-icon="inline-start" />
          {display(committed.from!)} – {display(committed.to!)}
        </PopoverTrigger>
        <PopoverContent className="w-auto p-0" align="start">
          <Calendar
            mode="range"
            resetOnSelect
            defaultMonth={committed.from}
            selected={draft ?? committed}
            weekStartsOn={1}
            numberOfMonths={2}
            onSelect={(range) => {
              if (!range?.from) return;
              if (!range.to) {
                setDraft(range);
                return;
              }
              setDraft(undefined);
              setOpen(false);
              onChange({
                startDate: format(range.from, dateKeyFormat),
                endDate: format(range.to, dateKeyFormat),
              });
            }}
          />
        </PopoverContent>
      </Popover>
    </Field>
  );
}
