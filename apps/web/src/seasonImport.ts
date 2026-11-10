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
  /** The season's resources link; blank keeps an existing season's. */
  resourcesUrl: string;
  /** The season's first day, as YYYY-MM-DD. */
  startDate: string;
  weeks: SeasonImportRow[];
  /** The open season this sheet updates, when its slug is already used. */
  existing?: ExistingSeason;
  /** Why the season as a whole cannot be imported; empty when it can. */
  problem: string;
}

/** A season that already exists, whose name and slug are taken. */
export interface ExistingSeason {
  id?: string;
  name: string;
  slug: string;
  status?: string;
}

/** The most weeks one season can have. */
export const maxSeasonWeeks = 52;

const columns = {
  season: ['season', 'season name', 'name'],
  slug: ['slug', 'season slug'],
  seasonResources: [
    'season_resources',
    'season resources',
    'season resources url',
    'resources url',
  ],
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

const withScheme = (url: string) =>
  /^https?:\/\//i.test(url) ? url : `https://${url}`;

/**
 * Inline Markdown and links in already-escaped text: `code`, [text](address),
 * bare web addresses (and ones starting www.), **bold**, *italic* or _italic_,
 * and ~~strikethrough~~. Code and links are set aside first, so nothing inside
 * them (an underscore in an address) is read as emphasis.
 */
function inline(escaped: string): string {
  const kept: string[] = [];
  const keep = (html: string) => `\uE000${kept.push(html) - 1}\uE000`;
  let text = escaped
    .replace(/`([^`]+)`/g, (_, code: string) => keep(`<code>${code}</code>`))
    .replace(
      /\[([^\]]+)\]\(\s*((?:https?:\/\/|www\.)[^\s)]+)\s*\)/gi,
      (_, label: string, url: string) =>
        keep(`<a href="${withScheme(url)}">${label}</a>`),
    )
    .replace(
      /(?:https?:\/\/|\bwww\.)[^\s<\uE000]+[^\s<.,;:!?)\uE000]/gi,
      (url) => keep(`<a href="${withScheme(url)}">${url}</a>`),
    );
  text = text
    .replace(/\*\*([^\s*](?:[^*]*[^\s*])?)\*\*/g, '<strong>$1</strong>')
    .replace(
      /(^|[\s(])__([^\s_](?:[^_]*[^\s_])?)__(?=$|[\s).,;:!?])/g,
      '$1<strong>$2</strong>',
    )
    .replace(/~~([^\s~](?:[^~]*[^\s~])?)~~/g, '<s>$1</s>')
    .replace(/\*([^\s*](?:[^*]*[^\s*])?)\*/g, '<em>$1</em>')
    .replace(
      /(^|[\s(])_([^\s_](?:[^_]*[^\s_])?)_(?=$|[\s).,;:!?])/g,
      '$1<em>$2</em>',
    );
  return text.replace(/\uE000(\d+)\uE000/g, (_, n: string) => kept[Number(n)]);
}

/**
 * Turns a spreadsheet cell into a week's resources page. HTML is kept (the
 * API sanitises it). Otherwise it is read as Markdown: headings (#), bullet
 * (- * +) and numbered (1.) lists, > quotes, ``` code blocks, --- rules,
 * **bold**, *italic*, ~~strikethrough~~, `code` and [text](address) links;
 * bare web addresses become links, and any other line is a paragraph.
 */
export function resourcesToHtml(text: string): string {
  const trimmed = text.trim();
  if (!trimmed) return '';
  if (/^<[a-z]/i.test(trimmed)) return trimmed;
  const blocks: string[] = [];
  let list: { tag: 'ul' | 'ol'; items: string[] } | undefined;
  let quote: string[] = [];
  let code: string[] | undefined;
  const closeList = () => {
    if (list) blocks.push(`<${list.tag}>${list.items.join('')}</${list.tag}>`);
    list = undefined;
  };
  const closeQuote = () => {
    if (quote.length)
      blocks.push(
        `<blockquote>${quote.map((line) => `<p>${line}</p>`).join('')}</blockquote>`,
      );
    quote = [];
  };
  for (const raw of trimmed.split(/\r?\n/)) {
    if (code) {
      if (/^\s*```/.test(raw)) {
        blocks.push(`<pre><code>${code.join('\n')}</code></pre>`);
        code = undefined;
      } else code.push(escapeHtml(raw));
      continue;
    }
    const line = raw.trim();
    if (/^```/.test(line)) {
      closeList();
      closeQuote();
      code = [];
      continue;
    }
    const quoted = /^>\s?(.*)$/.exec(line);
    if (quoted) {
      closeList();
      if (quoted[1]) quote.push(inline(escapeHtml(quoted[1])));
      continue;
    }
    closeQuote();
    const heading = /^(#{1,6})\s+(.*?)\s*#*$/.exec(line);
    const bullet = /^[-*+•]\s+(.*)$/.exec(line);
    const numbered = /^\d+[.)]\s+(.*)$/.exec(line);
    const item = bullet ?? numbered;
    if (item) {
      const tag = bullet ? 'ul' : 'ol';
      if (list?.tag !== tag) {
        closeList();
        list = { tag, items: [] };
      }
      list.items.push(`<li><p>${inline(escapeHtml(item[1]))}</p></li>`);
      continue;
    }
    closeList();
    if (heading)
      blocks.push(
        `<h${Math.min(heading[1].length, 4)}>${inline(escapeHtml(heading[2]))}</h${Math.min(heading[1].length, 4)}>`,
      );
    else if (/^(?:-{3,}|\*{3,}|_{3,})$/.test(line)) blocks.push('<hr>');
    else if (line) blocks.push(`<p>${inline(escapeHtml(line))}</p>`);
  }
  if (code) blocks.push(`<pre><code>${code.join('\n')}</code></pre>`);
  closeList();
  closeQuote();
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
 * explains why the sheet cannot be read. Blank season and slug
 * cells copy the row above (the slug only within one season). A
 * blank start is the day after the previous week's end, and a blank end is
 * the seventh day of the week, so weeks run on back to back; a season's first
 * week needs its start. A season whose slug (or, with no slug, name) is that
 * of an `existing` open season updates it, weeks matched by number; a season
 * with at most 52 weeks. `scope` limits the sheet to one season, as when
 * importing from that season's own page.
 */
export function parseSeasonSheet(
  text: string,
  existing: ExistingSeason[] = [],
  scope?: { name: string; slug: string },
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
      resourcesUrl: '',
      startDate: '',
      weeks: [],
      problem: '',
    };
    groups.set(key, group);
    current = group;
    group.slug ||= value('slug').toLowerCase();
    group.resourcesUrl ||= value('seasonResources');

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

  const find = (field: 'name' | 'slug', value: string) =>
    existing.find(
      (season) => season[field].toLowerCase() === value.toLowerCase(),
    );
  for (const group of groups.values()) {
    // With no slug given, a name that is already a season's name picks that season.
    if (!group.slug && group.name) {
      const named = find('name', group.name);
      group.slug = named ? named.slug : seasonSlug(group.name);
    }
    // Weeks run in order of number, each starting after the one before ends.
    const ordered = group.weeks
      .filter((row) => !row.problem)
      .sort((a, b) => a.number - b.number);
    for (let i = 1; i < ordered.length; i += 1) {
      const [before, row] = [ordered[i - 1], ordered[i]];
      if (row.startDate <= before.endDate)
        row.problem =
          row.endDate >= before.startDate
            ? `Overlaps week ${before.number}.`
            : `Must come after week ${before.number}.`;
    }
    group.startDate = seasonDates(group).startDate;
    const match = find('slug', group.slug);
    const named = find('name', group.name);
    if (match) group.existing = match;
    group.problem = !group.name
      ? 'Name the season in the season column.'
      : !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(group.slug)
        ? 'The slug may only use lowercase letters, numbers and hyphens.'
        : group.resourcesUrl && !/^https:\/\/\S+$/i.test(group.resourcesUrl)
          ? 'Season resources must be a link starting with https://.'
          : scope && group.slug !== scope.slug
            ? `This sheet is for "${scope.name}", not "${group.name}". Import it from the Seasons page.`
            : match?.id?.startsWith('new:')
              ? `The slug "${group.slug}" is already used.`
              : match && match.status !== 'open'
                ? `"${match.name}" is closed, so it cannot be changed.`
                : named && named !== match
                  ? `A season called "${group.name}" already exists.`
                  : group.weeks.length > maxSeasonWeeks
                    ? `A season can have at most ${maxSeasonWeeks} weeks.`
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
