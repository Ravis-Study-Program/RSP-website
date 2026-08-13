import { parseDate } from '@internationalized/date';
import { IconCalendar, IconChevronLeft, IconChevronRight } from '@tabler/icons-react';
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
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  error?: string;
}) {
  return (
    <DatePicker
      className={styles.field}
      value={value ? parseDate(value) : null}
      onChange={(date) => onChange(date?.toString() ?? '')}
      isInvalid={Boolean(error)}
      granularity="day"
    >
      <Label className={styles.fieldLabel}>{label}</Label>
      <Group className={styles.dateGroup}>
        <DateInput className={styles.dateInput}>{(segment) => <DateSegment className={styles.dateSegment} segment={segment} />}</DateInput>
        <Button className={styles.iconButton} aria-label={`Choose ${label.toLowerCase()}`}><IconCalendar size={18} aria-hidden="true" /></Button>
      </Group>
      {error ? <p className={styles.fieldError}>{error}</p> : null}
      <Popover className={styles.datePopover}>
        <Dialog>
          <Calendar>
            <div className={styles.calendarHeader}>
              <Button slot="previous" className={styles.iconButton} aria-label="Previous month"><IconChevronLeft size={18} aria-hidden="true" /></Button>
              <Heading className={styles.calendarHeading} />
              <Button slot="next" className={styles.iconButton} aria-label="Next month"><IconChevronRight size={18} aria-hidden="true" /></Button>
            </div>
            <CalendarGrid className={styles.calendarGrid}>
              <CalendarGridHeader>{(day) => <CalendarHeaderCell>{day}</CalendarHeaderCell>}</CalendarGridHeader>
              <CalendarGridBody>{(date) => <CalendarCell className={styles.calendarCell} date={date} />}</CalendarGridBody>
            </CalendarGrid>
          </Calendar>
        </Dialog>
      </Popover>
    </DatePicker>
  );
}
