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

import styles from '@/styles/App.module.css';

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
      className={styles.field}
      value={dateValue}
      onChange={(date) => onChange(date?.toString() ?? '')}
      isInvalid={Boolean(error)}
      granularity={granularity}
      hourCycle={24}
    >
      <Label className={styles.fieldLabel}>{label}</Label>
      <Group className={styles.dateGroup}>
        <DateInput className={styles.dateInput}>
          {(segment) => (
            <DateSegment className={styles.dateSegment} segment={segment} />
          )}
        </DateInput>
        <Button
          className={styles.iconButton}
          aria-label={`Choose ${label.toLowerCase()}`}
        >
          <IconCalendar size={18} aria-hidden="true" />
        </Button>
      </Group>
      {error ? (
        <p className={styles.fieldError} role="alert">
          {error}
        </p>
      ) : null}
      <Popover className={styles.datePopover}>
        <Dialog>
          <Calendar>
            <div className={styles.calendarHeader}>
              <Button
                slot="previous"
                className={styles.iconButton}
                aria-label="Previous month"
              >
                <IconChevronLeft size={18} aria-hidden="true" />
              </Button>
              <Heading className={styles.calendarHeading} />
              <Button
                slot="next"
                className={styles.iconButton}
                aria-label="Next month"
              >
                <IconChevronRight size={18} aria-hidden="true" />
              </Button>
            </div>
            <CalendarGrid className={styles.calendarGrid}>
              <CalendarGridHeader>
                {(day) => <CalendarHeaderCell>{day}</CalendarHeaderCell>}
              </CalendarGridHeader>
              <CalendarGridBody>
                {(date) => (
                  <CalendarCell className={styles.calendarCell} date={date} />
                )}
              </CalendarGridBody>
            </CalendarGrid>
          </Calendar>
        </Dialog>
      </Popover>
    </DatePicker>
  );
}
