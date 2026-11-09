import { parseDelimited } from '@/csv';
import { zonedDateTimeToUtc } from '@/utils';

/** One week from a season spreadsheet, checked, unless it has a problem. */
export interface SeasonImportRow {
  /** The row's line in the sheet (the header is line 1). */
  line: number;
  number: number;
  /** The week's first and last days, as YYYY-MM-DD. */
  startDate: string;
  endDate: string;
  resourcesHtml: string;
  /** Why this week cannot be imported; empty when it can. */
  problem: string;
}

/** A season from a spreadsheet: its rows grouped by the season column. */
export interface SeasonImportGroup {
  name: string;
  slug: string;
  location: string;
  /** The season's first day, as YYYY-MM-DD. */
  startDate: string;
  weeks: SeasonImportRow[];
  /** Why the season as a whole cannot be created; empty when it can. */
  problem: string;
}

const columns = {
  season: ['season', 'season name', 'name'],
  slug: ['slug', 'season slug'],
  location: ['location'],
  week: ['week', 'week number', 'number'],
  start: ['start', 'start date', 'starts'],
  end: ['end', 'end date', 'ends'],
  resources: ['resources', 'resource', 'notes'],
} as const;
type Column = keyof typeof columns;

/** The required columns of a season spreadsheet, to recognise one. */
export const seasonSheetColumns = ['season', 'week', 'start'];

/**
 * Reads a date as YYYY-MM-DD or D/M/YYYY (day first, as Australian
 * spreadsheets export it). Returns YYYY-MM-DD, or '' for anything else.
 */
export function parseSheetDate(value: string): string {
  const text = value.trim();
  const iso = /^(\d{4})-(\d{1,2})-(\d{1,2})$/.exec(text);
  const dayFirst = /^(\d{1,2})\/(\d{1,2})\/(\d{4})$/.exec(text);
  const [year, month, day] = iso
    ? [iso[1], iso[2], iso[3]]
    : dayFirst
      ? [dayFirst[3], dayFirst[2], dayFirst[1]]
      : [];
  if (!year) return '';
  const date = new Date(Date.UTC(Number(year), Number(month) - 1, Number(day)));
  if (
    date.getUTCFullYear() !== Number(year) ||
    date.getUTCMonth() !== Number(month) - 1 ||
    date.getUTCDate() !== Number(day)
  )
    return '';
  return date.toISOString().slice(0, 10);
}

const escapeHtml = (text: string) =>
  text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');

const linkify = (escaped: string) =>
  escaped.replace(
    /https?:\/\/[^\s<]+[^\s<.,;:!?)]/g,
    (url) => `<a href="${url}">${url}</a>`,
  );

/**
 * Turns a spreadsheet cell into a week's resources page. HTML is kept (the
 * API sanitises it); plain text becomes paragraphs, lines starting with "- "
 * or "* " become a bulleted list, and web addresses become links.
 */
export function resourcesToHtml(text: string): string {
  const trimmed = text.trim();
  if (!trimmed) return '';
  if (/^<[a-z]/i.test(trimmed)) return trimmed;
  const blocks: string[] = [];
  let items: string[] = [];
  const closeList = () => {
    if (items.length) blocks.push(`<ul>${items.join('')}</ul>`);
    items = [];
  };
  for (const raw of trimmed.split(/\r?\n/)) {
    const line = raw.trim();
    const bullet = /^[-*•]\s+(.*)$/.exec(line);
    if (bullet) items.push(`<li><p>${linkify(escapeHtml(bullet[1]))}</p></li>`);
    else {
      closeList();
      if (line) blocks.push(`<p>${linkify(escapeHtml(line))}</p>`);
    }
  }
  closeList();
  return blocks.join('');
}

/** A week's start (00:00 on its first day) and end (23:59:59 on its last). */
export function weekTimes(row: SeasonImportRow) {
  return {
    startAt: zonedDateTimeToUtc(`${row.startDate}T00:00`),
    endAt: zonedDateTimeToUtc(`${row.endDate}T23:59:59`),
  };
}

/**
 * Reads seasons and their weeks from a spreadsheet, one row per week, or
 * explains why the sheet cannot be read. Blank season, slug and location
 * cells copy the row above (slug and location only within one season). A
 * blank start is the day after the previous week's end, and a blank end is
 * the seventh day of the week, so weeks run on back to back; a season's first
 * week needs its start. `existing` are the seasons already created, whose
 * names and slugs are taken.
 */
export function parseSeasonSheet(
  text: string,
  existing: Array<{ name: string; slug: string }> = [],
): SeasonImportGroup[] | string {
  const [header, ...body] = parseDelimited(text);
  if (!header) return 'This file is empty.';
  const index = {} as Record<Column, number>;
  for (const column of Object.keys(columns) as Column[])
    index[column] = header.findIndex((cell) =>
      (columns[column] as readonly string[]).includes(
        cell.trim().toLowerCase(),
      ),
    );
  const missing = (['season', 'week', 'start'] as Column[]).filter(
    (column) => index[column] < 0,
  );
  if (missing.length)
    return `The first row must name the columns; missing: ${missing.join(', ')}.`;
  if (!body.length) return 'This file has no weeks under its header.';

  const groups = new Map<string, SeasonImportGroup>();
  let current: SeasonImportGroup | undefined;
  body.forEach((cells, offset) => {
    const value = (column: Column) =>
      index[column] < 0 ? '' : (cells[index[column]] ?? '').trim();
    // A blank season cell continues the season above.
    const name = value('season') || current?.name || '';
    const key = name.toLowerCase();
    const group = groups.get(key) ?? {
      name,
      slug: '',
      location: '',
      startDate: '',
      weeks: [],
      problem: '',
    };
    groups.set(key, group);
    current = group;
    group.slug ||= value('slug').toLowerCase();
    group.location ||= value('location');

    const numberText = value('week').replace(/^week\s*/i, '');
    const number = /^\d+$/.test(numberText) ? Number(numberText) : 0;
    const previous = group.weeks.at(-1);
    const startText = value('start');
    const endText = value('end');
    const startDate = startText
      ? parseSheetDate(startText)
      : previous?.endDate
        ? addDays(previous.endDate, 1)
        : '';
    const endDate = endText
      ? parseSheetDate(endText)
      : startDate
        ? addDays(startDate, 6)
        : '';
    const row: SeasonImportRow = {
      line: offset + 2,
      number,
      startDate,
      endDate,
      resourcesHtml: resourcesToHtml(value('resources')),
      problem: '',
    };
    row.problem =
      number < 1
        ? 'Week must be a number from 1.'
        : group.weeks.some((week) => week.number === number)
          ? `Week ${number} is listed more than once.`
          : startText && !startDate
            ? `Start "${startText}" must be a date, such as 2027-02-01 or 1/2/2027.`
            : !startDate
              ? 'Give the first week of a season its start date.'
              : endText && !endDate
                ? `End "${endText}" must be a date, such as 2027-02-07 or 7/2/2027.`
                : endDate < startDate
                  ? 'End is before start.'
                  : '';
    group.weeks.push(row);
  });

  const taken = (value: string, field: 'name' | 'slug') =>
    existing.some(
      (season) => season[field].toLowerCase() === value.toLowerCase(),
    );
  for (const group of groups.values()) {
    group.slug ||= seasonSlug(group.name);
    // Weeks may not overlap: compare each with the one starting before it.
    const ordered = group.weeks
      .filter((row) => !row.problem)
      .sort((a, b) => a.startDate.localeCompare(b.startDate));
    for (let i = 1; i < ordered.length; i += 1)
      if (ordered[i].startDate <= ordered[i - 1].endDate)
        ordered[i].problem = `Overlaps week ${ordered[i - 1].number}.`;
    group.startDate = ordered[0]?.startDate ?? '';
    group.problem = !group.name
      ? 'Name the season in the season column.'
      : taken(group.name, 'name')
        ? `A season called "${group.name}" already exists.`
        : !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(group.slug)
          ? 'The slug may only use lowercase letters, numbers and hyphens.'
          : taken(group.slug, 'slug')
            ? `The slug "${group.slug}" is already used.`
            : group.weeks.some((week) => week.problem)
              ? 'Fix the weeks marked below.'
              : '';
  }
  const all = [...groups.values()];
  const slugs = all.map((group) => group.slug);
  for (const group of all)
    if (
      !group.problem &&
      slugs.filter((slug) => slug === group.slug).length > 1
    )
      group.problem = `Two seasons in this file would use the slug "${group.slug}".`;
  return all;
}

/** The day `days` after a YYYY-MM-DD date. */
function addDays(date: string, days: number) {
  const next = new Date(`${date}T00:00:00Z`);
  next.setUTCDate(next.getUTCDate() + days);
  return next.toISOString().slice(0, 10);
}

/** The season's dates: its first week's start to its last week's end. */
export function seasonDates(group: SeasonImportGroup) {
  const starts = group.weeks.map((week) => week.startDate).sort();
  const ends = group.weeks.map((week) => week.endDate).sort();
  return { startDate: starts[0] ?? '', endDate: ends.at(-1) ?? '' };
}

/** A season URL slug from its name, such as summer-2026-27. */
export function seasonSlug(name: string) {
  return name
    .normalize('NFKD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 50)
    .replace(/-+$/, '');
}
