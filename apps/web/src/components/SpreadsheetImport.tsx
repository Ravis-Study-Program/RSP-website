import { IconDownload, IconFileSpreadsheet } from '@tabler/icons-react';
import { useQueryClient } from '@tanstack/react-query';
import { useState, type ReactNode } from 'react';

import { apiRequest } from '@/api/client';
import type {
  ImportChange,
  InvitationBatchCreate,
  InvitationBatchResult,
  InvitationBatchResults,
  SeasonImport,
} from '@/api/generated/models';
import { demoMode } from '@/api/queries';
import { CsvColumns } from '@/components/CsvColumns';
import { FormDialog, DialogCancel } from '@/components/Dialogs';
import { StatusBadge } from '@/components/StatusBadge';
import { Button } from '@/components/ui/button';
import { Field, FieldLabel } from '@/components/ui/field';
import {
  invitationSheetColumns,
  parseInvitationSheet,
  type ImportRow,
} from '@/invitationImport';
import {
  parseSeasonSheet,
  seasonSlug,
  seasonSheetColumns,
  weekTimes,
  type SeasonImportGroup,
} from '@/seasonImport';
import type { Season } from '@/types';
import { formatDate } from '@/utils';
import { parseDelimited } from '@/csv';

export type ImportKind = 'seasons' | 'invitations';

// The API takes at most this many invitation rows per request.
const batchSize = 200;

const templates: Record<ImportKind, { file: string; text: string }> = {
  seasons: {
    file: 'seasons-template.csv',
    text: [
      'season,slug,season_resources,week,start,end,resources',
      'Summer 2026/27,summer-2026-27,https://example.com/summer-2026-27,1,2027-02-01,,"Arrays and hashing',
      '- https://leetcode.com/problems/two-sum/"',
      ',,,2,,,Two pointers',
      ',,,3,,,',
    ].join('\n'),
  },
  invitations: {
    file: 'invitations-template.csv',
    text: [
      'name,username,email,role,mentor,season',
      'Alex Chen,,alex@example.com,student,priya-raman,Summer 2025/26',
      'Sam Lee,sam-lee,sam@example.com,mentor,,summer-2025-26',
    ].join('\n'),
  },
};

const references: Record<
  ImportKind,
  { columns: Array<{ name: string; description: ReactNode }> }
> = {
  invitations: {
    columns: [
      { name: 'name', description: 'The person’s full name.' },
      {
        name: 'username',
        description: 'Optional: the username they get with a new account.',
      },
      {
        name: 'email',
        description: 'Where the invitation goes; matches existing people.',
      },
      { name: 'role', description: 'student, mentor or coordinator.' },
      {
        name: 'mentor',
        description:
          'Optional, students only: their mentor’s email, username or name.',
      },
      { name: 'season', description: 'The season’s name or slug.' },
    ],
  },
  seasons: {
    columns: [
      {
        name: 'season',
        description: 'The season’s name; blank copies the row above.',
      },
      {
        name: 'slug',
        description:
          'Optional: the season’s URL slug; made from the name, and copied down. An open season with this slug is updated, its weeks matched by number.',
      },
      {
        name: 'season_resources',
        description:
          'Optional: a link (https://…) to the season’s resources, in the season’s first row only. Blank keeps an existing season’s link.',
      },
      { name: 'week', description: 'The week’s number, 1 to 52.' },
      {
        name: 'start',
        description:
          'The week’s first day: 2027-02-01 or 1/2/2027. Needed for a season’s first week; blank is the day after the week above ends.',
      },
      {
        name: 'end',
        description: 'Optional: the week’s last day; blank is its 7th day.',
      },
      {
        name: 'resources',
        description:
          'Optional: the week’s resources. Markdown: # headings, - and 1. lists, > quotes, **bold**, *italic*, `code`, [text](https://…) links; web addresses become links; other lines are paragraphs.',
      },
    ],
  },
};

type SheetFile =
  | { name: string; kind: 'seasons'; groups: SeasonImportGroup[] }
  | { name: string; kind: 'invitations'; rows: ImportRow[] }
  | { name: string; kind: 'error'; error: string };

/** The id a season about to be created stands in under, before it exists. */
const pendingId = (slug: string) => `new:${slug}`;

const fieldLabels: Record<string, string> = {
  role: 'Role',
  mentor: 'Mentor',
  name: 'Name',
  slug: 'Slug',
};

const changeText = (change: ImportChange) =>
  `${fieldLabels[change.field] ?? change.field}: ${change.from || 'none'} → ${change.to || 'none'}`;

/** A result badge with optional grey detail lines under it. */
function Outcome({
  tone,
  label,
  details = [],
}: {
  tone: 'success' | 'info' | 'warning' | 'muted' | 'danger';
  label: string;
  details?: string[];
}) {
  // An error reads on one line, flowing on from its badge: "[Error]: reason".
  if (tone === 'danger' && details.length)
    return (
      <span className="text-sm text-muted-foreground">
        <StatusBadge tone={tone}>{label}</StatusBadge>
        <span aria-hidden="true">:</span> {details.join(' ')}
      </span>
    );
  return (
    <span className="grid justify-items-start gap-1">
      <StatusBadge tone={tone}>{label}</StatusBadge>
      {details.map((detail) => (
        <span key={detail} className="text-xs text-muted-foreground">
          {detail}
        </span>
      ))}
    </span>
  );
}

/**
 * A problem as a badge: someone already invited (yellow), or anything else
 * as an error (red) with its reason.
 */
function ProblemOutcome({ problem }: { problem: string }) {
  if (/^already (invited|a member)/i.test(problem))
    return <Outcome tone="warning" label="Already Invited" />;
  return <Outcome tone="danger" label="Error" details={[problem]} />;
}

/**
 * Imports spreadsheets: seasons with their weeks, invitations, or (for a mass
 * import) any number of both at once. Each file is recognised by its columns.
 * Everything is checked first, including what would change for people already
 * in a season; seasons are created before invitations, so an invitation can
 * name a season from the same import.
 */
export function SpreadsheetImport({
  kinds,
  title,
  trigger,
  seasons,
  scope,
}: {
  kinds: ImportKind[];
  title: string;
  trigger: ReactNode;
  /** Every season, to recognise names and slugs; invitations go to open ones. */
  seasons: Season[];
  /** Limits a season sheet to this season's weeks, as on its own page. */
  scope?: Season;
}) {
  const client = useQueryClient();
  const [open, setOpen] = useState(false);
  const [files, setFiles] = useState<SheetFile[]>([]);
  const [plans, setPlans] = useState(
    new Map<ImportRow, InvitationBatchResult>(),
  );
  const [checking, setChecking] = useState(false);
  const [running, setRunning] = useState(false);
  const [done, setDone] = useState(false);
  const [seasonResults, setSeasonResults] = useState(
    new Map<SeasonImportGroup, string>(),
  );
  const [rowResults, setRowResults] = useState(
    new Map<ImportRow, InvitationBatchResult>(),
  );
  const [requestError, setRequestError] = useState('');
  const [chosenNames, setChosenNames] = useState('');
  const mass = kinds.length > 1;

  const groups = files.flatMap((file) =>
    file.kind === 'seasons' ? file.groups : [],
  );
  const importable = groups.filter((group) => !group.problem);
  const creating = importable.filter((group) => !group.existing);
  const updating = importable.filter((group) => group.existing);
  const rows = files.flatMap((file) =>
    file.kind === 'invitations' ? file.rows : [],
  );
  // A row can be sent when the sheet has no problem with it and, for an
  // existing season, the dry run found nothing wrong.
  const sendable = rows.filter(
    (row) =>
      !row.problem &&
      plans.get(row)?.outcome !== 'failed' &&
      plans.get(row)?.outcome !== 'unchanged',
  );
  const nothingToDo = !importable.length && !sendable.length;
  // Nothing is imported while anything shows an error; "already invited" is a
  // warning, not an error.
  const isWarning = (problem?: string) =>
    !!problem && /^already (invited|a member)/i.test(problem);
  const hasErrors =
    files.some((file) => file.kind === 'error') ||
    groups.some(
      (group) => !!group.problem || group.weeks.some((week) => !!week.problem),
    ) ||
    rows.some((row) => {
      if (row.problem) return !isWarning(row.problem);
      const plan = plans.get(row);
      return plan?.outcome === 'failed' && !isWarning(plan.error);
    });

  const reset = () => {
    setFiles([]);
    setPlans(new Map());
    setSeasonResults(new Map());
    setRowResults(new Map());
    setRequestError('');
    setDone(false);
  };

  // Asks the API what each invitation row would do, season by season.
  const preview = async (sheetRows: ImportRow[]) => {
    const next = new Map<ImportRow, InvitationBatchResult>();
    const bySeason = groupBySeason(
      sheetRows.filter(
        (row) => !row.problem && !row.season!.id.startsWith('new:'),
      ),
    );
    for (const [seasonId, group] of bySeason)
      for (const chunk of chunks(group)) {
        const response = demoMode
          ? demoResults(chunk, true)
          : await apiRequest<InvitationBatchResults>(
              `/seasons/${encodeURIComponent(seasonId)}/invitations/batch/preview`,
              { method: 'POST', body: JSON.stringify(batchBody(chunk)) },
            );
        for (const result of response.results)
          next.set(chunk[result.row], result);
      }
    return next;
  };

  const readFiles = async (list: FileList | null) => {
    reset();
    if (!list?.length) return;
    // Season sheets first, so invitations can name the seasons they create.
    const read = await Promise.all(
      [...list].map(async (file) => ({
        name: file.name,
        text: await file.text(),
      })),
    );
    const kindOf = (text: string): ImportKind | undefined => {
      const header = (parseDelimited(text)[0] ?? []).map((cell) =>
        cell.trim().toLowerCase(),
      );
      const has = (columns: string[]) =>
        columns.every((column) => header.includes(column));
      if (has(seasonSheetColumns)) return 'seasons';
      if (has(invitationSheetColumns)) return 'invitations';
    };
    const sheets: SheetFile[] = [];
    const created: Season[] = [];
    for (const file of read.filter((f) => kindOf(f.text) === 'seasons')) {
      if (!kinds.includes('seasons')) continue;
      const parsed = parseSeasonSheet(
        file.text,
        [...seasons, ...created],
        scope,
      );
      if (typeof parsed === 'string')
        sheets.push({ name: file.name, kind: 'error', error: parsed });
      else {
        sheets.push({ name: file.name, kind: 'seasons', groups: parsed });
        for (const group of parsed.filter((g) => !g.problem))
          created.push({
            id: pendingId(group.slug),
            name: group.name,
            slug: group.slug,
            status: 'open',
          } as Season);
      }
    }
    const openSeasons = [
      ...seasons.filter((season) => season.status === 'open'),
      ...created,
    ];
    for (const file of read) {
      const kind = kindOf(file.text);
      if (kind === 'seasons') continue;
      if (kind !== 'invitations' || !kinds.includes('invitations')) {
        sheets.push({
          name: file.name,
          kind: 'error',
          error: `Not recognised: the first row must name the columns ${kinds
            .map((k) =>
              (k === 'seasons'
                ? seasonSheetColumns
                : invitationSheetColumns
              ).join(','),
            )
            .join(' or ')}.`,
        });
        continue;
      }
      const parsed = parseInvitationSheet(file.text, openSeasons);
      sheets.push(
        typeof parsed === 'string'
          ? { name: file.name, kind: 'error', error: parsed }
          : { name: file.name, kind: 'invitations', rows: parsed },
      );
    }
    setFiles(sheets);
    const sheetRows = sheets.flatMap((sheet) =>
      sheet.kind === 'invitations' ? sheet.rows : [],
    );
    if (!sheetRows.length) return;
    setChecking(true);
    try {
      setPlans(await preview(sheetRows));
    } catch (error) {
      setRequestError(
        error instanceof Error ? error.message : 'Could not check the rows.',
      );
    } finally {
      setChecking(false);
    }
  };

  const run = async () => {
    setRunning(true);
    setRequestError('');
    const seasonOutcomes = new Map<SeasonImportGroup, string>();
    const outcomes = new Map<ImportRow, InvitationBatchResult>();
    const createdIds = new Map<string, string>();
    let rejected = false;
    try {
      for (const group of importable) {
        const body: SeasonImport = {
          name: group.name,
          slug: group.slug,
          location: '',
          imageUrl: '',
          resourcesUrl: group.resourcesUrl,
          weeks: group.weeks.map((week) => ({
            number: week.number,
            ...weekTimes(week),
            resourcesHtml: week.resourcesHtml,
          })),
        };
        try {
          const season = demoMode
            ? { id: crypto.randomUUID() }
            : await apiRequest<{ id: string }>('/seasons/import', {
                method: 'POST',
                body: JSON.stringify(body),
              });
          if (!group.existing) createdIds.set(pendingId(group.slug), season.id);
          seasonOutcomes.set(group, group.existing ? 'updated' : 'created');
        } catch (error) {
          seasonOutcomes.set(
            group,
            error instanceof Error ? error.message : 'Not created',
          );
        }
        setSeasonResults(new Map(seasonOutcomes));
      }
      const bySeason = groupBySeason(sendable);
      for (const [seasonId, group] of bySeason) {
        const targetId = seasonId.startsWith('new:')
          ? createdIds.get(seasonId)
          : seasonId;
        if (!targetId) {
          for (const row of group)
            outcomes.set(row, {
              row: 0,
              outcome: 'failed',
              error: 'Its season was not created.',
            });
          continue;
        }
        for (const chunk of chunks(group)) {
          const response = demoMode
            ? demoResults(chunk, false)
            : await apiRequest<InvitationBatchResults>(
                `/seasons/${encodeURIComponent(targetId)}/invitations/batch`,
                { method: 'POST', body: JSON.stringify(batchBody(chunk)) },
              );
          for (const result of response.results)
            outcomes.set(chunk[result.row], result);
          setRowResults(new Map(outcomes));
          // A batch with any failing row saves nothing; stop there.
          if (!demoMode && response.applied === false) {
            rejected = true;
            break;
          }
        }
        if (rejected) break;
      }
      if (rejected) {
        // Take back the seasons this run created, so nothing is left half done.
        for (const season of createdIds.values())
          await apiRequest(`/seasons/${encodeURIComponent(season)}`, {
            method: 'DELETE',
          }).catch(() => undefined);
        for (const [group, outcome] of seasonOutcomes)
          if (outcome === 'created') seasonOutcomes.set(group, 'Not created');
        setRequestError(
          'Nothing was imported: a row failed, so the whole import was cancelled. Fix the rows marked Error and try again.',
        );
      }
    } catch (error) {
      setRequestError(
        error instanceof Error ? error.message : 'The import stopped early.',
      );
    } finally {
      setSeasonResults(new Map(seasonOutcomes));
      setRowResults(new Map(outcomes));
      setRunning(false);
      setDone(true);
      await Promise.all(
        ['seasons', 'invitations', 'season-enrollments', 'season-weeks'].map(
          (key) => client.invalidateQueries({ queryKey: [key] }),
        ),
      );
    }
  };

  const counts = {
    seasons: [...seasonResults.values()].filter((v) => v === 'created').length,
    seasonsUpdated: [...seasonResults.values()].filter((v) => v === 'updated')
      .length,
    invited: [...rowResults.values()].filter((r) => r.outcome === 'invited')
      .length,
    updated: [...rowResults.values()].filter((r) => r.outcome === 'updated')
      .length,
    failed:
      [...seasonResults.values()].filter(
        (v) => v !== 'created' && v !== 'updated',
      ).length +
      [...rowResults.values()].filter((r) => r.outcome === 'failed').length,
  };
  const summary = [
    counts.seasons
      ? `${counts.seasons} season${counts.seasons === 1 ? '' : 's'} created`
      : '',
    counts.seasonsUpdated
      ? `${counts.seasonsUpdated} season${counts.seasonsUpdated === 1 ? '' : 's'} updated`
      : '',
    counts.invited ? `${counts.invited} invited` : '',
    counts.updated ? `${counts.updated} updated` : '',
    counts.failed ? `${counts.failed} failed` : '',
  ]
    .filter(Boolean)
    .join(', ');

  const plural = (count: number, noun: string) =>
    `${count} ${noun}${count === 1 ? '' : 's'}`;
  const action = [
    scope && updating.length
      ? 'Update Weeks'
      : [
          creating.length ? `Create ${plural(creating.length, 'Season')}` : '',
          updating.length ? `Update ${plural(updating.length, 'Season')}` : '',
        ]
          .filter(Boolean)
          .join(' and '),
    sendable.length
      ? `${importable.length ? 'and ' : ''}Import ${sendable.length} ${sendable.length === 1 ? 'Person' : 'People'}`
      : '',
  ]
    .filter(Boolean)
    .join(' ');

  return (
    <FormDialog
      open={open}
      onOpenChange={(next) => {
        if (
          !next &&
          files.length &&
          !done &&
          !window.confirm('Discard this unsent import?')
        )
          return;
        if (!next) reset();
        setOpen(next);
      }}
      title={title}
      contentClassName="sm:max-w-7xl"
      triggerVariant="outline"
      trigger={
        <>
          <IconFileSpreadsheet size={17} aria-hidden="true" /> {trigger}
        </>
      }
    >
      <div className="grid max-h-[75vh] gap-5 overflow-y-auto">
        {kinds.map((kind) => (
          <div key={kind} className="grid gap-2">
            {mass ? (
              <h3 className="text-sm font-semibold">
                {kind === 'seasons'
                  ? 'Season Spreadsheets'
                  : 'Invitation Spreadsheets'}
              </h3>
            ) : null}
            <CsvColumns columns={references[kind].columns} />
            <a
              className="inline-flex w-fit items-center gap-1.5 text-sm font-medium underline-offset-4 hover:underline"
              href={`data:text/csv;charset=utf-8,${encodeURIComponent(templates[kind].text)}`}
              download={templates[kind].file}
            >
              <IconDownload size={16} aria-hidden="true" />{' '}
              {kind === 'seasons' ? 'Season template' : 'Invitations template'}
            </a>
          </div>
        ))}
        <Field>
          <FieldLabel htmlFor={`import-files-${kinds.join('-')}`}>
            {mass ? 'Spreadsheets' : 'Spreadsheet'}
          </FieldLabel>
          {/* The browser's own file control cannot show "Choose File: name",
              so it is hidden and this label stands in for it. */}
          <div className="relative">
            <input
              id={`import-files-${kinds.join('-')}`}
              type="file"
              multiple={mass}
              accept=".csv,.tsv,text/csv,text/tab-separated-values"
              disabled={running}
              className="peer absolute inset-0 cursor-pointer opacity-0 disabled:cursor-not-allowed"
              onChange={(event) => {
                setChosenNames(
                  [...(event.target.files ?? [])]
                    .map((file) => file.name)
                    .join(', '),
                );
                void readFiles(event.target.files);
              }}
            />
            <div
              aria-hidden="true"
              className="flex h-9 items-center rounded-lg border border-input bg-transparent px-2.5 text-sm peer-focus-visible:border-ring peer-focus-visible:ring-3 peer-focus-visible:ring-ring/50 peer-disabled:opacity-50"
            >
              <span className="font-medium">Choose File</span>
              <span className="whitespace-pre">{': '}</span>
              <span className="truncate">
                {chosenNames || 'No file chosen'}
              </span>
            </div>
          </div>
        </Field>
        {files.map((file) => (
          <section
            key={file.name}
            className="grid gap-2"
            aria-label={file.name}
          >
            {mass || files.length > 1 ? (
              <h3 className="text-sm font-semibold">{file.name}</h3>
            ) : null}
            {file.kind === 'error' ? (
              <p role="alert" className="text-sm text-destructive">
                {file.error}
              </p>
            ) : file.kind === 'seasons' ? (
              <SeasonTable
                name={file.name}
                groups={file.groups}
                results={seasonResults}
              />
            ) : (
              <InvitationTable
                name={file.name}
                rows={file.rows}
                plans={plans}
                results={rowResults}
                checking={checking}
              />
            )}
          </section>
        ))}
        {requestError ? (
          <p role="alert" className="text-sm text-destructive">
            {requestError}
          </p>
        ) : null}
        {done ? (
          <p role="status" className="text-sm text-muted-foreground">
            {summary ? `${summary}.` : 'Nothing was imported.'}
          </p>
        ) : null}
        <div className="button-group">
          {done ? null : <DialogCancel />}
          {done ? (
            <Button type="button" onClick={() => setOpen(false)}>
              Done
            </Button>
          ) : (
            <Button
              type="button"
              disabled={running || checking || nothingToDo || hasErrors}
              onClick={() => void run()}
            >
              {running
                ? 'Importing…'
                : checking
                  ? 'Checking…'
                  : action || 'Import'}
            </Button>
          )}
        </div>
      </div>
    </FormDialog>
  );
}

function groupBySeason(rows: ImportRow[]) {
  const bySeason = new Map<string, ImportRow[]>();
  for (const row of rows)
    bySeason.set(row.season!.id, [
      ...(bySeason.get(row.season!.id) ?? []),
      row,
    ]);
  // Mentors and coordinators go first, so students named as their mentees
  // can follow them in a later request.
  const leads = (row: ImportRow) =>
    row.role === 'mentor' || row.role === 'coordinator' ? 0 : 1;
  for (const [season, list] of bySeason)
    bySeason.set(
      season,
      [...list].sort((a, b) => leads(a) - leads(b)),
    );
  return bySeason;
}

function chunks(rows: ImportRow[]) {
  const out: ImportRow[][] = [];
  for (let start = 0; start < rows.length; start += batchSize)
    out.push(rows.slice(start, start + batchSize));
  return out;
}

function batchBody(rows: ImportRow[]): InvitationBatchCreate {
  return {
    invitations: rows.map((row) => ({
      name: row.name,
      email: row.email,
      role: row.role,
      ...(row.mentor ? { mentor: row.mentor } : {}),
      ...(row.slug ? { slug: row.slug } : {}),
    })),
  };
}

function demoResults(
  rows: ImportRow[],
  dryRun: boolean,
): InvitationBatchResults {
  return {
    results: rows.map((row, index) => ({
      row: index,
      outcome: dryRun ? 'invite' : 'invited',
      person: 'new',
      role: row.role,
    })),
  };
}

const cell = 'px-3 py-2 align-top';

/**
 * A preview table with fixed column widths (in percent), so every file of a
 * kind lines up the same whatever its contents; long text wraps.
 */
function Table({
  label,
  columns,
  children,
}: {
  label: string;
  columns: Array<[heading: string, width: number]>;
  children: ReactNode;
}) {
  return (
    <div className="max-h-96 overflow-auto rounded-lg border">
      <table
        className="w-full table-fixed text-left text-sm [overflow-wrap:anywhere]"
        aria-label={label}
      >
        <colgroup>
          {columns.map(([heading, width]) => (
            <col key={heading} style={{ width: `${width}%` }} />
          ))}
        </colgroup>
        <thead className="sticky top-0 bg-card text-xs font-semibold text-muted-foreground">
          <tr>
            {columns.map(([heading]) => (
              <th key={heading} className="border-b px-3 py-2">
                {heading}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>{children}</tbody>
      </table>
    </div>
  );
}

// Both kinds of table share one grid: the first two columns (a name and a
// slug) and the Result column are the same width, and the four between them
// add up to the same width, so tables from different files line up.
const nameWidth = 15;
const slugWidth = 13;
const resultWidth = 19;

/** A week's resources as one line of plain text, for the preview. */
function resourcesText(html: string) {
  if (!html) return '';
  const element = document.createElement('div');
  // A space after each block, so paragraphs and list items do not run on.
  element.innerHTML = html.replace(/<\/(p|li|h[1-6]|blockquote)>/gi, '$& ');
  return (element.textContent ?? '').replace(/\s+/g, ' ').trim();
}

const weekDate = (date: string) =>
  date ? formatDate(`${date}T12:00:00`) : '—';

/** A season sheet, one row per week, with each season named once. */
function SeasonTable({
  name,
  groups,
  results,
}: {
  name: string;
  groups: SeasonImportGroup[];
  results: Map<SeasonImportGroup, string>;
}) {
  return (
    <Table
      label={`Seasons in ${name}`}
      columns={[
        ['Season', nameWidth],
        ['Slug', slugWidth],
        ['Season Resources', 12],
        ['Week', 6],
        ['Start', 10],
        ['End', 10],
        ['Resources', 15],
        ['Result', resultWidth],
      ]}
    >
      {groups.flatMap((group) => {
        const result = results.get(group);
        // A problem with the season itself, as opposed to one of its weeks.
        const seasonProblem =
          group.problem && !group.weeks.some((week) => week.problem)
            ? group.problem
            : '';
        // A week listed twice is a problem with the season, so it reads at the
        // top of the season's rows rather than beside the repeat.
        const repeated = (problem: string) =>
          /listed more than once/.test(problem);
        const repeats = [
          ...new Set(
            group.weeks.flatMap((week) =>
              week.problem && repeated(week.problem) ? [week.problem] : [],
            ),
          ),
        ];
        return group.weeks.map((week, index) => {
          const problems = [
            ...(week.problem && !repeated(week.problem) ? [week.problem] : []),
            ...(index === 0 ? repeats : []),
          ];
          return (
            <tr
              key={`${group.slug}:${week.line}`}
              // Only the last week of a season gets a divider, so no line stops
              // short of the season and slug cells that span its weeks.
              className={
                index === group.weeks.length - 1
                  ? 'border-b last:border-b-0'
                  : undefined
              }
            >
              {index === 0 ? (
                <>
                  <td className={cell} rowSpan={group.weeks.length}>
                    {group.name || '—'}
                  </td>
                  <td
                    className={`${cell} font-mono text-[0.8125rem]`}
                    rowSpan={group.weeks.length}
                  >
                    {group.slug || '—'}
                  </td>
                  <td className={cell} rowSpan={group.weeks.length}>
                    <span
                      className="line-clamp-2 break-all"
                      title={group.resourcesUrl}
                    >
                      {group.resourcesUrl || '—'}
                    </span>
                  </td>
                </>
              ) : null}
              <td className={cell}>{week.number || '—'}</td>
              <td className={cell}>{weekDate(week.startDate)}</td>
              <td className={cell}>{weekDate(week.endDate)}</td>
              <td className={cell}>
                <span
                  className="line-clamp-2"
                  title={resourcesText(week.resourcesHtml)}
                >
                  {resourcesText(week.resourcesHtml) || '—'}
                </span>
              </td>
              <td className={cell}>
                {problems.length ? (
                  <Outcome tone="danger" label="Error" details={problems} />
                ) : week.problem ? null : seasonProblem ? (
                  <Outcome
                    tone="danger"
                    label="Error"
                    details={index === 0 ? [seasonProblem] : []}
                  />
                ) : result === 'created' ? (
                  <Outcome tone="success" label="Created" />
                ) : result === 'updated' ? (
                  <Outcome tone="info" label="Updated" />
                ) : result ? (
                  <Outcome
                    tone="danger"
                    label="Error"
                    details={index === 0 ? [result] : []}
                  />
                ) : group.existing ? (
                  <Outcome tone="info" label="Will Update" />
                ) : (
                  <Outcome tone="success" label="New" />
                )}
              </td>
            </tr>
          );
        });
      })}
    </Table>
  );
}

function InvitationTable({
  name,
  rows,
  plans,
  results,
  checking,
}: {
  name: string;
  rows: ImportRow[];
  plans: Map<ImportRow, InvitationBatchResult>;
  results: Map<ImportRow, InvitationBatchResult>;
  checking: boolean;
}) {
  // The username each person gets: the one written, the one the server made
  // from their name, or (for a season not created yet) a guess the same way,
  // numbered (maddie-frew2) when taken in this file.
  const taken = new Set(rows.map((row) => row.slug).filter(Boolean));
  const guessed = new Map<string, string>();
  const slugOf = (row: ImportRow) => {
    if (row.slug) return { slug: row.slug, made: false };
    const server = (results.get(row) ?? plans.get(row))?.slug;
    if (server) return { slug: server, made: true };
    if (row.problem || !row.name) return { slug: '', made: false };
    const key = `${row.email}|${row.name.toLowerCase()}`;
    let slug = guessed.get(key);
    if (!slug) {
      const base = seasonSlug(row.name) || 'member';
      for (let n = 1; ; n += 1) {
        slug = n === 1 ? base : `${base}${n}`;
        if (!taken.has(slug)) break;
      }
      taken.add(slug);
      guessed.set(key, slug);
    }
    return { slug, made: true };
  };
  return (
    <Table
      label={`Rows in ${name}`}
      columns={[
        ['Name', nameWidth],
        ['Username', slugWidth],
        ['Email', 18],
        ['Role', 9],
        ['Mentor', 12],
        ['Season', 14],
        ['Result', resultWidth],
      ]}
    >
      {rows.map((row) => {
        const plan = plans.get(row);
        const result = results.get(row);
        const shown = result ?? plan;
        const newSeason = row.season?.id.startsWith('new:');
        return (
          <tr key={row.line} className="border-b last:border-b-0">
            <td className={cell}>{row.name || '—'}</td>
            <td
              className={`${cell} font-mono text-[0.8125rem]`}
              title={slugOf(row).made ? 'Made from their name' : undefined}
            >
              {slugOf(row).slug || '—'}
            </td>
            <td className={cell}>{row.email || '—'}</td>
            <td className={`${cell} capitalize`}>{row.role || '—'}</td>
            <td className={cell}>{row.mentor || '—'}</td>
            <td className={cell}>
              {row.season?.name ?? (row.seasonText || '—')}
              {newSeason ? (
                <span className="block text-muted-foreground">New season</span>
              ) : null}
            </td>
            <td className={cell}>
              <RowResult
                problem={row.problem}
                shown={shown}
                checking={checking && !plan && !newSeason}
              />
            </td>
          </tr>
        );
      })}
    </Table>
  );
}

function RowResult({
  problem,
  shown,
  checking,
}: {
  problem: string;
  shown?: InvitationBatchResult;
  checking: boolean;
}) {
  if (problem) return <ProblemOutcome problem={problem} />;
  if (checking) return <span className="text-muted-foreground">Checking…</span>;
  if (!shown) return <Outcome tone="success" label="New" />;
  if (shown.outcome === 'failed')
    return <ProblemOutcome problem={shown.error ?? 'Not imported.'} />;
  const details = [
    ...(shown.person === 'account' && shown.outcome === 'invite'
      ? ['Has an account; joins on accepting.']
      : []),
    ...(shown.changes ?? []).map(changeText),
    ...(shown.skipped ?? []).map(
      (change) =>
        `${fieldLabels[change.field] ?? change.field} not changed (System Admin only): ${change.from} → ${change.to}`,
    ),
  ];
  const [tone, label] = {
    invite: ['success', 'New'],
    invited: ['success', 'Invited'],
    update: ['info', 'Will Update'],
    updated: ['info', 'Updated'],
    unchanged: ['muted', 'No Changes'],
  }[shown.outcome] as ['success' | 'info' | 'muted', string];
  return <Outcome tone={tone} label={label} details={details} />;
}
