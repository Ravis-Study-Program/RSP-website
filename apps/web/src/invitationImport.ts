import { parseDelimited } from '@/csv';
import type { Season } from '@/types';

export { parseDelimited };

/** The required columns of an invitations spreadsheet, to recognise one. */
export const invitationSheetColumns = ['name', 'role', 'email', 'season'];

/** One spreadsheet row, checked and ready to send unless it has a problem. */
export interface ImportRow {
  /** The row's line in the sheet (the header is line 1). */
  line: number;
  name: string;
  email: string;
  role: string;
  mentor: string;
  /** The username to give the person; empty for one made from their name. */
  slug: string;
  seasonText: string;
  season?: Season;
  /** Why this row cannot be sent; empty when it can. */
  problem: string;
}

const roles = ['student', 'mentor', 'coordinator'];
const columns = {
  name: ['name', 'full name'],
  role: ['role'],
  email: ['email', 'email address'],
  mentor: ['mentor', 'mentor email', 'mentor name'],
  season: ['season', 'season name', 'season slug'],
  slug: ['slug', 'username'],
} as const;
type Column = keyof typeof columns;

const normalise = (value: string) => value.trim().toLowerCase();

/**
 * Reads an invitations spreadsheet into checked rows, or explains why the
 * sheet as a whole cannot be read. Seasons are matched by name or slug among
 * `seasons`; mentors and existing members are matched by the API.
 */
export function parseInvitationSheet(
  text: string,
  seasons: Season[],
): ImportRow[] | string {
  const [header, ...body] = parseDelimited(text);
  if (!header) return 'This file is empty.';
  const index = {} as Record<Column, number>;
  for (const column of Object.keys(columns) as Column[]) {
    index[column] = header.findIndex((cell) =>
      (columns[column] as readonly string[]).includes(normalise(cell)),
    );
  }
  const missing = (['name', 'role', 'email', 'season'] as Column[]).filter(
    (column) => index[column] < 0,
  );
  if (missing.length)
    return `The first row must name the columns; missing: ${missing.join(', ')}.`;
  if (!body.length) return 'This file has no rows under its header.';

  // Who first asked for each slug; the same person may repeat a row, but two
  // people cannot share a slug.
  const slugOwners = new Map<string, string>();
  return body.map((cells, offset) => {
    const value = (column: Column) =>
      index[column] < 0 ? '' : (cells[index[column]] ?? '').trim();
    const row: ImportRow = {
      line: offset + 2,
      name: value('name'),
      email: value('email').toLowerCase(),
      role: normalise(value('role')),
      mentor: value('mentor'),
      slug: value('slug').toLowerCase().replace(/^@/, ''),
      seasonText: value('season'),
      problem: '',
    };
    row.season = seasons.find(
      (season) =>
        normalise(season.name) === normalise(row.seasonText) ||
        normalise(season.slug) === normalise(row.seasonText),
    );
    row.problem = !row.name
      ? 'Add a name.'
      : !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(row.email)
        ? 'Add a valid email address.'
        : !roles.includes(row.role)
          ? 'Role must be student, mentor or coordinator.'
          : row.mentor && row.role !== 'student'
            ? 'Only students can be given a mentor.'
            : row.slug && !/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(row.slug)
              ? 'Slugs use lowercase letters, numbers and single hyphens.'
              : row.slug && (row.slug.length < 3 || row.slug.length > 50)
                ? 'Slugs are 3 to 50 characters.'
                : !row.seasonText
                  ? 'Add a season.'
                  : !row.season
                    ? `No open season called "${row.seasonText}".`
                    : row.slug &&
                        slugOwners.has(row.slug) &&
                        slugOwners.get(row.slug) !== row.email
                      ? 'This slug is already used by another row in this file.'
                      : '';
    if (!row.problem && row.slug && !slugOwners.has(row.slug))
      slugOwners.set(row.slug, row.email);
    return row;
  });
}
