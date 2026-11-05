import type { FilterFn } from '@tanstack/react-table';

/** Filter choices for a date column; each value is a number of days back. */
export const recentDateFilterOptions = [
  { label: 'Past 7 days', value: '7' },
  { label: 'Past 30 days', value: '30' },
  { label: 'Past 90 days', value: '90' },
  { label: 'Past year', value: '365' },
];

/** Keeps rows whose date column falls within the chosen number of days. */
export const withinRecentDays: FilterFn<any> = (row, columnId, days) =>
  Date.now() - new Date(row.getValue<string>(columnId)).getTime() <=
  Number(days) * 24 * 60 * 60 * 1000;
