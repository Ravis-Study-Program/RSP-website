import { parseDate, parseDateTime } from '@internationalized/date';
import {
  IconCalendar,
  IconChevronLeft,
  IconChevronRight,
} from '@tabler/icons-react';
import {
  Button,
  Calendar,
  CalendarCell,
  CalendarGrid,
  CalendarGridBody,
  CalendarGridHeader,
  CalendarHeaderCell,
  DateInput,
  DatePicker,
  DateSegment,
  Dialog,
  Group,
  Heading,
  Label,
  Popover,
} from 'react-aria-components';

import { FieldError } from '@/components/ui/field';

const iconButtonClass =
  'inline-flex size-7 items-center justify-center rounded-md text-muted-foreground outline-none hover:bg-muted hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 disabled:opacity-50';

export function AppDatePicker({
  label,
  value,
  onChange,
  error,
  granularity = 'day',
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  error?: string;
  granularity?: 'day' | 'minute';
}) {
  const dateValue = value
    ? granularity === 'minute'
      ? parseDateTime(value)
      : parseDate(value)
    : null;
  return (
    <DatePicker
      className="flex w-full flex-col gap-1.5"
      value={dateValue}
      onChange={(date) => onChange(date?.toString() ?? '')}
      isInvalid={Boolean(error)}
      granularity={granularity}
      hourCycle={24}
    >
      <Label className="text-sm font-medium">{label}</Label>
      <Group className="flex h-9 items-center gap-0.5 rounded-lg border border-input bg-transparent px-1 focus-within:border-ring focus-within:ring-3 focus-within:ring-ring/50 data-invalid:border-destructive">
        <DateInput className="flex min-w-0 flex-1 px-1.5">
          {(segment) => (
            <DateSegment
              className="rounded-sm px-px outline-none data-focused:bg-primary data-focused:text-primary-foreground"
              segment={segment}
            />
          )}
        </DateInput>
        <Button
          className={iconButtonClass}
          aria-label={`Choose ${label.toLowerCase()}`}
        >
          <IconCalendar size={18} aria-hidden="true" />
        </Button>
      </Group>
      {error ? <FieldError>{error}</FieldError> : null}
      <Popover className="z-100 rounded-lg border border-border bg-popover p-3 text-popover-foreground shadow-md">
        <Dialog>
          <Calendar>
            <div className="mb-2 flex items-center justify-between gap-2">
              <Button
                slot="previous"
                className={iconButtonClass}
                aria-label="Previous month"
              >
                <IconChevronLeft size={18} aria-hidden="true" />
              </Button>
              <Heading className="text-sm font-semibold" />
              <Button
                slot="next"
                className={iconButtonClass}
                aria-label="Next month"
              >
                <IconChevronRight size={18} aria-hidden="true" />
              </Button>
            </div>
            <CalendarGrid className="w-full border-collapse [&_td]:p-0.5 [&_th]:p-0.5">
              <CalendarGridHeader>
                {(day) => (
                  <CalendarHeaderCell className="text-xs font-medium text-muted-foreground">
                    {day}
                  </CalendarHeaderCell>
                )}
              </CalendarGridHeader>
              <CalendarGridBody>
                {(date) => (
                  <CalendarCell
                    className="grid size-9 cursor-default place-items-center rounded-md text-sm outline-none data-disabled:text-muted-foreground/50 data-focus-visible:ring-3 data-focus-visible:ring-ring/50 data-hovered:bg-muted data-outside-month:text-muted-foreground data-selected:bg-primary data-selected:text-primary-foreground data-unavailable:line-through"
                    date={date}
                  />
                )}
              </CalendarGridBody>
            </CalendarGrid>
          </Calendar>
        </Dialog>
      </Popover>
    </DatePicker>
  );
}
