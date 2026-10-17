import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useQueryClient, type QueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import {
  IconArrowRight,
  IconChevronDown,
  IconChevronUp,
} from '@tabler/icons-react';
import { useMemo, useState, type FormEvent } from 'react';
import { Link, Navigate, useParams } from 'react-router-dom';

import {
  defaultMondayUpdateRange,
  type MondayUpdateRange,
} from '@/activityDates';
import { apiRequest } from '@/api/client';
import type {
  MondayUpdate,
  MondayUpdateActivity,
  MondayUpdateMark,
  MondayUpdateMarkMutation,
  MondayUpdateMarkRecord,
  MondayUpdateMutation,
  MondayUpdateReviewRow,
  MondayUpdateReviewUpdate,
} from '@/api/generated/models';
import {
  demoMode,
  useCurrentUser,
  useMondayUpdateReview,
  useMondayUpdates,
  useMondayUpdateRangeActivity,
  useSeasonMentorships,
  useSeasonPeople,
  useSeasons,
  type MondayUpdateFilters,
} from '@/api/queries';
import { DateRangePicker } from '@/components/DateRangePicker';
import { PageHeader, usePageTitle } from '@/components/Common';
import { DataTable } from '@/components/DataTable';
import { NamedConfirmation } from '@/components/Dialogs';
import { RichTextContent } from '@/components/RichTextContent';
import { RichTextEditor } from '@/components/RichTextEditor';
import {
  ErrorState,
  InlineNotice,
  PageSkeleton,
} from '@/components/StatusViews';
import { formatDate, formatDateTime } from '@/utils';

type Experience = 'student' | 'mentor' | 'reviewer';

const markOptions: { value: MondayUpdateMark; label: string }[] = [
  { value: 'way_ahead', label: 'Way ahead' },
  { value: 'doing_good', label: 'Doing good' },
  { value: 'on_track', label: 'On track' },
  { value: 'a_bit_behind', label: 'A bit behind' },
  { value: 'at_risk', label: 'At risk' },
];

const pageClass = 'mx-auto w-full max-w-[92rem]';
const sectionClass = 'mt-6';
const sectionHeaderClass =
  'mb-3 flex flex-wrap items-end justify-between gap-x-4 gap-y-1';
const cardHeaderClass =
  'flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1';
const sectionTitleClass = 'text-xl font-semibold tracking-tight';
const cardTitleClass = 'text-base font-semibold tracking-tight';
const subTitleClass = 'text-sm font-semibold';
const mutedClass = 'text-sm text-muted-foreground';
const rowClass = 'flex flex-wrap items-center gap-2';
const cleanListClass = 'm-0 grid list-none gap-3 p-0';
const metaListClass =
  'm-0 grid list-none gap-2 p-0 text-sm text-muted-foreground';
const fieldGridClass = 'grid gap-3.5 min-[56rem]:grid-cols-2';
const formClass = 'grid gap-4';
const cardClass = 'gap-4 p-5';

const defaultHours = '0';

/** Hours are 0–168 in half-hour steps; returns null when the input is invalid. */
function parseHours(value: string): number | null {
  if (value.trim() === '') return null;
  const hours = Number(value);
  if (!Number.isFinite(hours) || hours < 0 || hours > 168) return null;
  return Number.isInteger(hours * 2) ? hours : null;
}

function hoursLabel(hours: number) {
  return `${hours} ${hours === 1 ? 'hour' : 'hours'}`;
}

const markSelectItems: { label: string; value: MondayUpdateMark | '' }[] = [
  { label: 'No mark yet', value: '' },
  ...markOptions,
];

function markLabel(mark: MondayUpdateMark) {
  return markOptions.find((option) => option.value === mark)?.label ?? mark;
}

/** One colour per mark, from strongest (solid green) to weakest (solid red). */
const markTone: Record<MondayUpdateMark, { badge: string; dot: string }> = {
  way_ahead: { badge: 'bg-success text-background', dot: 'bg-success' },
  doing_good: { badge: 'bg-success-soft text-success', dot: 'bg-success/60' },
  on_track: { badge: 'bg-info-soft text-info', dot: 'bg-info' },
  a_bit_behind: { badge: 'bg-warning-soft text-warning', dot: 'bg-warning' },
  at_risk: { badge: 'bg-destructive text-background', dot: 'bg-destructive' },
};

function MarkBadge({ mark }: { mark: MondayUpdateMark }) {
  return <Badge className={markTone[mark].badge}>{markLabel(mark)}</Badge>;
}

function updateRangeLabel(update: { startDate: string; endDate: string }) {
  return `${formatDate(update.startDate)} – ${formatDate(update.endDate)}`;
}

/** Rich text counts as empty when it carries no text, only markup. */
function hasContent(html: string) {
  return (
    html
      .replace(/<[^>]*>/g, ' ')
      .replace(/&nbsp;/gi, ' ')
      .trim().length > 0
  );
}

function invalidateMondayUpdates(client: QueryClient) {
  return Promise.all([
    client.invalidateQueries({ queryKey: ['monday-updates'] }),
    client.invalidateQueries({ queryKey: ['monday-update-review'] }),
  ]);
}

export function MondayUpdatesPage() {
  const { slug } = useParams();
  const seasons = useSeasons();
  const user = useCurrentUser();
  const season = seasons.data?.items.find((item) => item.slug === slug);
  const [range, setRange] = useState<MondayUpdateRange>(() =>
    defaultMondayUpdateRange(),
  );
  const [unselectedStudentIds, setUnselectedStudentIds] = useState<string[]>(
    [],
  );

  const actor = user.data;
  const membership = actor?.seasonRoles.find(
    (item) => item.seasonId === season?.id,
  );
  const isGlobalManager = Boolean(
    actor?.globalRoles.some(
      (role) => role === 'director' || role === 'system_admin',
    ),
  );
  const experience: Experience | null =
    isGlobalManager ||
    (membership?.role === 'coordinator' && membership.state === 'active')
      ? 'reviewer'
      : membership?.role === 'mentor' &&
          (membership.state === 'active' || membership.state === 'completed')
        ? 'mentor'
        : membership?.role === 'student'
          ? 'student'
          : null;

  const mentorships = useSeasonMentorships(
    experience === 'mentor' ? season?.id : undefined,
  );
  const people = useSeasonPeople(
    experience === 'mentor' ? season?.id : undefined,
  );
  const menteeIds = useMemo(() => {
    const mentorId = actor?.id;
    if (!mentorId) return [];
    return [
      ...new Set(
        (mentorships.data?.items ?? [])
          .filter((mentorship) => mentorship.mentorUserId === mentorId)
          .map((mentorship) => mentorship.studentUserId),
      ),
    ];
  }, [mentorships.data, actor?.id]);
  // Every mentee is selected by default; narrowing it unchecks a student.
  const selectedStudentIds = useMemo(
    () => menteeIds.filter((id) => !unselectedStudentIds.includes(id)),
    [menteeIds, unselectedStudentIds],
  );
  const filters = useMemo<MondayUpdateFilters>(
    () => ({
      startDate: range.startDate,
      endDate: range.endDate,
      studentIds: selectedStudentIds,
    }),
    [range.startDate, range.endDate, selectedStudentIds],
  );
  const updates = useMondayUpdates(
    experience === 'student' ||
      (experience === 'mentor' && selectedStudentIds.length > 0),
    season?.id,
    filters,
  );
  const review = useMondayUpdateReview(
    experience === 'reviewer' ? season?.id : undefined,
    range.startDate,
    range.endDate,
  );
  usePageTitle(season ? `Monday updates · ${season.name}` : 'Monday updates');

  if (seasons.isLoading || user.isLoading)
    return <PageSkeleton label="Loading Monday updates" />;
  if (seasons.isError || user.isError)
    return (
      <div className={pageClass}>
        <ErrorState
          onRetry={() => void Promise.all([seasons.refetch(), user.refetch()])}
        />
      </div>
    );
  if (!season)
    return (
      <div className={pageClass}>
        <ErrorState
          title="Season not found"
          message="This season may have moved or you may not have access."
        />
      </div>
    );
  if (!actor || !experience) return <Navigate to="/forbidden" replace />;

  // The list is the actor's own updates; the filter keeps the demo fixtures to
  // the signed-in student as well.
  const ownUpdates = (updates.data?.items ?? []).filter(
    (update) => update.student.id === actor.id,
  );
  const updatesByStudent = new Map<string, MondayUpdate[]>();
  for (const update of updates.data?.items ?? []) {
    const current = updatesByStudent.get(update.student.id);
    if (current) current.push(update);
    else updatesByStudent.set(update.student.id, [update]);
  }
  const studentNames = new Map<string, string>();
  const studentSlugs = new Map<string, string>();
  for (const person of people.data?.items ?? []) {
    studentNames.set(person.id, person.name);
    studentSlugs.set(person.id, person.slug);
  }
  for (const update of updates.data?.items ?? []) {
    if (!studentNames.has(update.student.id))
      studentNames.set(update.student.id, update.student.name);
    if (!studentSlugs.has(update.student.id))
      studentSlugs.set(update.student.id, update.student.slug);
  }
  const studentLabel = (studentId: string) =>
    studentNames.get(studentId) ?? studentSlugs.get(studentId) ?? studentId;

  return (
    <div className={pageClass}>
      <PageHeader
        eyebrow={season.name}
        title="Monday updates"
        description={
          experience === 'student'
            ? 'Post what you worked on and see the practice and mock activity inside each date range.'
            : experience === 'mentor'
              ? "Review your students' written updates and record your own mark and comment for each one."
              : 'One row per student with their solved LeetCode counts, mock interviews and every update in the range.'
        }
      />
      {experience === 'student' ? null : (
        <div className={sectionClass}>
          <DateRangePicker label="Dates" value={range} onChange={setRange} />
        </div>
      )}
      {experience === 'student' ? (
        <>
          <StudentUpdateForm
            seasonId={season.id}
            range={range}
            onRangeChange={setRange}
          />
          <section
            className={sectionClass}
            aria-labelledby="monday-history-title"
          >
            <div className={sectionHeaderClass}>
              <h2 id="monday-history-title" className={sectionTitleClass}>
                Your updates
              </h2>
              <span className={mutedClass}>{ownUpdates.length} posted</span>
            </div>
            {updates.isLoading ? (
              <p role="status" className={mutedClass}>
                Loading your updates…
              </p>
            ) : updates.isError ? (
              <ErrorState
                title="Your updates could not be loaded"
                onRetry={() => void updates.refetch()}
              />
            ) : ownUpdates.length === 0 ? (
              <Card className={cardClass}>
                <h3 className={cardTitleClass}>No updates yet</h3>
                <p className={mutedClass}>
                  Post your first update for this range so your mentor can see
                  what you worked on.
                </p>
              </Card>
            ) : (
              <div className={cleanListClass}>
                {ownUpdates.map((update) => (
                  <StudentUpdateCard
                    key={update.id}
                    seasonId={season.id}
                    update={update}
                  />
                ))}
              </div>
            )}
          </section>
        </>
      ) : experience === 'mentor' ? (
        <section className={sectionClass} aria-labelledby="monday-mentor-title">
          <div className={sectionHeaderClass}>
            <h2 id="monday-mentor-title" className={sectionTitleClass}>
              Student updates
            </h2>
            <span className={mutedClass}>
              {selectedStudentIds.length} of {menteeIds.length} students
              selected
            </span>
          </div>
          <FieldSet className="min-w-0">
            <FieldLegend variant="label" className="font-semibold">
              Students
            </FieldLegend>
            <FieldDescription>
              Every student you mentor is selected by default. Uncheck a student
              to leave their updates out.
            </FieldDescription>
            <div className={fieldGridClass}>
              {menteeIds.map((studentId) => (
                <label
                  className="flex min-h-6 items-center gap-2 text-sm"
                  key={studentId}
                >
                  <Checkbox
                    checked={!unselectedStudentIds.includes(studentId)}
                    onCheckedChange={(checked) =>
                      setUnselectedStudentIds((ids) =>
                        checked
                          ? ids.filter((id) => id !== studentId)
                          : [...ids, studentId],
                      )
                    }
                  />
                  {studentLabel(studentId)}
                </label>
              ))}
            </div>
          </FieldSet>
          {menteeIds.length === 0 ? (
            <Card className={cardClass}>
              <h3 className={cardTitleClass}>No students assigned</h3>
              <p className={mutedClass}>
                A coordinator can assign students to your mentor team.
              </p>
            </Card>
          ) : selectedStudentIds.length === 0 ? (
            <Card className={cardClass}>
              <h3 className={cardTitleClass}>No students selected</h3>
              <p className={mutedClass}>
                Select at least one student above to review their Monday
                updates.
              </p>
            </Card>
          ) : updates.isLoading ? (
            <p role="status" className={mutedClass}>
              Loading updates for your students…
            </p>
          ) : updates.isError ? (
            <ErrorState
              title="Updates could not be loaded"
              onRetry={() => void updates.refetch()}
            />
          ) : (
            selectedStudentIds.map((studentId) => {
              const studentUpdates = updatesByStudent.get(studentId) ?? [];
              return (
                <section
                  className={sectionClass}
                  key={studentId}
                  aria-labelledby={`monday-student-${studentId}`}
                >
                  <div className={sectionHeaderClass}>
                    <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                      <h3
                        id={`monday-student-${studentId}`}
                        className={cardTitleClass}
                      >
                        {studentLabel(studentId)}
                      </h3>
                      {studentUpdates.length ? (
                        <Badge variant="secondary">
                          {studentUpdates.length === 1
                            ? '1 update in this range'
                            : `${studentUpdates.length} updates in this range`}
                        </Badge>
                      ) : null}
                    </div>
                    {studentSlugs.get(studentId) ? (
                      <Link
                        to={`/people/${studentSlugs.get(studentId)}`}
                        className="inline-flex items-center gap-1 rounded-sm text-sm font-medium text-muted-foreground underline-offset-4 hover:text-foreground hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
                      >
                        View {studentLabel(studentId)}’s profile
                        <IconArrowRight className="size-4" aria-hidden />
                      </Link>
                    ) : null}
                  </div>
                  {studentUpdates.length === 0 ? (
                    <p className={mutedClass}>No update in this range.</p>
                  ) : (
                    <div className={cleanListClass}>
                      {studentUpdates.map((update) => (
                        <MentorUpdateCard
                          key={update.id}
                          seasonId={season.id}
                          update={update}
                          mentorId={actor.id}
                        />
                      ))}
                    </div>
                  )}
                </section>
              );
            })
          )}
        </section>
      ) : (
        <section className={sectionClass} aria-labelledby="monday-review-title">
          <div className={sectionHeaderClass}>
            <h2 id="monday-review-title" className={sectionTitleClass}>
              Student review
            </h2>
            <span className={mutedClass}>
              Expand a row for every update and its mentor marks.
            </span>
          </div>
          <ReviewerTable
            rows={review.data?.items ?? []}
            loading={review.isLoading}
            error={review.isError}
            onRetry={() => void review.refetch()}
          />
        </section>
      )}
    </div>
  );
}

function StudentUpdateForm({
  seasonId,
  range,
  onRangeChange,
}: {
  seasonId: string;
  range: MondayUpdateRange;
  onRangeChange: (range: MondayUpdateRange) => void;
}) {
  const queryClient = useQueryClient();
  const [content, setContent] = useState('');
  const [hoursInput, setHoursInput] = useState(defaultHours);
  const [pending, setPending] = useState(false);
  const [attempted, setAttempted] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const contentEmpty = !hasContent(content);
  const hours = parseHours(hoursInput);
  // Mentors see this range's activity with the update; show it while writing.
  const rangeActivity = useMondayUpdateRangeActivity(
    true,
    seasonId,
    range.startDate,
    range.endDate,
  );

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setAttempted(true);
    setError('');
    setMessage('');
    if (contentEmpty || hours === null) return;
    setPending(true);
    try {
      if (!demoMode)
        await apiRequest<MondayUpdate>(
          `/seasons/${encodeURIComponent(seasonId)}/monday-updates`,
          {
            method: 'POST',
            body: JSON.stringify({
              startDate: range.startDate,
              endDate: range.endDate,
              contentHtml: content,
              hours,
            } satisfies MondayUpdateMutation),
          },
        );
      setContent('');
      setHoursInput(defaultHours);
      setAttempted(false);
      setMessage('Your update was posted.');
      await invalidateMondayUpdates(queryClient);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to post your update.',
      );
    } finally {
      setPending(false);
    }
  };

  return (
    <section className={sectionClass} aria-labelledby="monday-post-title">
      <div className={sectionHeaderClass}>
        <h2 id="monday-post-title" className={sectionTitleClass}>
          Post an update
        </h2>
      </div>
      <form className={formClass} onSubmit={(event) => void submit(event)}>
        {error ? (
          <InlineNotice tone="warning">
            <span role="alert">{error}</span>
          </InlineNotice>
        ) : null}
        {message ? (
          <span className="sr-only" role="status">
            {message}
          </span>
        ) : null}
        <div className="grid items-start gap-4 min-[40rem]:grid-cols-2">
          <DateRangePicker
            label="Dates"
            value={range}
            onChange={onRangeChange}
          />
          <Field className="max-w-sm">
            <FieldLabel htmlFor="monday-update-hours">
              Hours spent (Expected: ~30h/week)
            </FieldLabel>
            <Input
              id="monday-update-hours"
              type="number"
              inputMode="decimal"
              min="0"
              max="168"
              step="0.5"
              required
              value={hoursInput}
              onChange={(event) => setHoursInput(event.target.value)}
              aria-invalid={attempted && hours === null}
            />
            {attempted && hours === null ? (
              <FieldError>
                Enter hours between 0 and 168 in steps of 0.5.
              </FieldError>
            ) : null}
          </Field>
        </div>
        <section aria-labelledby="monday-range-activity-title">
          <h3 id="monday-range-activity-title" className={subTitleClass}>
            Activity in this range
          </h3>
          {rangeActivity.isLoading ? (
            <p role="status" className={`${mutedClass} py-2.5`}>
              Loading your LeetCode and mock interviews…
            </p>
          ) : rangeActivity.isError ? (
            <ErrorState
              title="Your activity could not be loaded"
              onRetry={() => void rangeActivity.refetch()}
            />
          ) : rangeActivity.data ? (
            <UpdateActivitySummary activity={rangeActivity.data} />
          ) : null}
        </section>
        <RichTextEditor
          id="monday-update-content"
          label="Your update"
          value={content}
          onChange={setContent}
          error={
            attempted && contentEmpty
              ? 'Write something before posting your update.'
              : undefined
          }
        />
        <div className={rowClass}>
          <Button type="submit" disabled={pending}>
            {pending ? 'Posting…' : 'Post update'}
          </Button>
        </div>
      </form>
    </section>
  );
}

/**
 * Hours, LeetCode solved by difficulty and mock interviews for one date range,
 * as mentors see them. Hours are omitted where the student is still entering
 * them.
 */
function UpdateActivitySummary({
  activity,
  hours,
}: {
  activity: MondayUpdateActivity;
  hours?: number;
}) {
  const counts = { easy: 0, medium: 0, hard: 0 };
  for (const attempt of activity.attempts) {
    const solved =
      attempt.outcome === 'independently_solved' ||
      attempt.outcome === 'solved_with_hints';
    if (solved && attempt.difficulty in counts)
      counts[attempt.difficulty as keyof typeof counts] += 1;
  }
  const total = counts.easy + counts.medium + counts.hard;
  const mockCount = activity.mocks.length;
  const mockMinutes = activity.mocks.reduce(
    (sum, mock) => sum + mock.durationMinutes,
    0,
  );
  const difficulties = [
    { key: 'easy', label: 'Easy', count: counts.easy, dot: 'bg-success' },
    { key: 'medium', label: 'Medium', count: counts.medium, dot: 'bg-warning' },
    { key: 'hard', label: 'Hard', count: counts.hard, dot: 'bg-destructive' },
  ] as const;
  const rowClass =
    'grid grid-cols-[9rem_1fr] items-baseline gap-x-4 gap-y-1 border-t border-border py-2.5 first:border-t-0';
  return (
    <dl className="m-0 grid border-b border-border tabular-nums">
      {hours === undefined ? null : (
        <div className={rowClass}>
          <dt className="text-sm font-semibold">Hours spent</dt>
          <dd className="m-0 text-sm">
            <strong className="text-base font-semibold">{hours}</strong>{' '}
            {hours === 1 ? 'hour' : 'hours'}
          </dd>
        </div>
      )}
      <div className={rowClass}>
        <dt className="text-sm font-semibold">LeetCode</dt>
        <dd className="m-0 flex flex-wrap items-baseline gap-x-4 gap-y-1">
          <span className="text-sm">
            <strong className="text-base font-semibold">{total}</strong> solved
          </span>
          {difficulties.map((item) => (
            <span
              key={item.key}
              className="flex items-center gap-1.5 text-sm text-muted-foreground"
            >
              <span className={`size-2 rounded-full ${item.dot}`} aria-hidden />
              <span className="font-medium text-foreground">{item.count}</span>
              {item.label}
            </span>
          ))}
        </dd>
      </div>
      <div className={rowClass}>
        <dt className="text-sm font-semibold">Mock interviews</dt>
        <dd className="m-0 flex flex-wrap items-baseline gap-x-4 text-sm">
          <span>
            <strong className="text-base font-semibold">{mockCount}</strong>{' '}
            {mockCount === 1 ? 'interview' : 'interviews'}
          </span>
          {mockCount ? (
            <span className="text-muted-foreground">
              {mockMinutes} min total
            </span>
          ) : null}
        </dd>
      </div>
    </dl>
  );
}

function StudentUpdateCard({
  seasonId,
  update,
}: {
  seasonId: string;
  update: MondayUpdate;
}) {
  const queryClient = useQueryClient();
  const label = updateRangeLabel(update);
  const deleteUpdate = async () => {
    if (!demoMode)
      await apiRequest<void>(
        `/seasons/${encodeURIComponent(seasonId)}/monday-updates/${encodeURIComponent(update.id)}`,
        { method: 'DELETE' },
      );
    await invalidateMondayUpdates(queryClient);
  };
  return (
    <article>
      <Card className={cardClass}>
        <div className={cardHeaderClass}>
          <h3 className={cardTitleClass}>{label}</h3>
          <span className={mutedClass}>
            Posted {formatDateTime(update.createdAt)}
          </span>
        </div>
        <UpdateActivitySummary
          activity={update.activity}
          hours={update.hours}
        />
        <RichTextContent
          html={update.contentHtml}
          empty="No written content."
        />
        <div className={rowClass}>
          <NamedConfirmation
            name={label}
            actionLabel="Delete update"
            description="This permanently removes the update. The practice and mock activity it covers stays in your history."
            onConfirm={deleteUpdate}
          />
        </div>
      </Card>
    </article>
  );
}

function MentorUpdateCard({
  seasonId,
  update,
  mentorId,
}: {
  seasonId: string;
  update: MondayUpdate;
  mentorId: string;
}) {
  const queryClient = useQueryClient();
  const ownMark = update.marks.find((record) => record.mentor.id === mentorId);
  const otherMarks = update.marks.filter(
    (record) => record.mentor.id !== mentorId,
  );
  const [mark, setMark] = useState<MondayUpdateMark | ''>(ownMark?.mark ?? '');
  const [comment, setComment] = useState(ownMark?.commentHtml ?? '');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const label = updateRangeLabel(update);

  const save = async () => {
    if (!mark) return;
    setPending(true);
    setError('');
    setMessage('');
    try {
      if (!demoMode)
        await apiRequest<MondayUpdateMarkRecord>(
          `/seasons/${encodeURIComponent(seasonId)}/monday-updates/${encodeURIComponent(update.id)}/mark`,
          {
            method: 'PUT',
            body: JSON.stringify({
              mark,
              commentHtml: comment,
            } satisfies MondayUpdateMarkMutation),
          },
        );
      setMessage(`Your mark for ${label} was saved.`);
      await invalidateMondayUpdates(queryClient);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to save your mark.',
      );
    } finally {
      setPending(false);
    }
  };

  return (
    <article>
      <Card className={cardClass}>
        <div className={cardHeaderClass}>
          <h3 className={cardTitleClass}>{label}</h3>
          <span className={mutedClass}>
            Posted {formatDateTime(update.createdAt)}
          </span>
        </div>
        <UpdateActivitySummary
          activity={update.activity}
          hours={update.hours}
        />
        <RichTextContent
          html={update.contentHtml}
          empty="No written content."
        />
        <details className="group border-t border-border pt-3">
          <summary className="flex cursor-pointer list-none items-center justify-between gap-3 rounded-md py-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden">
            <span className="flex items-center gap-2">
              <span className="text-sm font-semibold">Your mark</span>
              {ownMark ? (
                <MarkBadge mark={ownMark.mark} />
              ) : (
                <span className={mutedClass}>Not marked yet</span>
              )}
            </span>
            <span className="flex items-center gap-1 text-sm text-muted-foreground group-hover:text-foreground">
              <span className="group-open:hidden">
                {ownMark ? 'Edit mark' : 'Mark update'}
              </span>
              <span className="hidden group-open:inline">Hide</span>
              <IconChevronDown
                className="size-4 transition-transform duration-200 ease-out group-open:rotate-180"
                aria-hidden
              />
            </span>
          </summary>
          <div className="mt-3 grid gap-4">
            <div className={fieldGridClass}>
              <Field>
                <FieldLabel htmlFor={`monday-mark-${update.id}`}>
                  Mark for {update.student.name}
                </FieldLabel>
                <Select
                  items={markSelectItems}
                  value={mark}
                  onValueChange={(value) =>
                    setMark((value ?? '') as MondayUpdateMark | '')
                  }
                >
                  <SelectTrigger
                    id={`monday-mark-${update.id}`}
                    className="w-full"
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {markSelectItems.map((item) => (
                        <SelectItem key={item.value} value={item.value}>
                          <span
                            className={`size-2 shrink-0 rounded-full ${
                              item.value
                                ? markTone[item.value].dot
                                : 'border border-border-strong'
                            }`}
                            aria-hidden
                          />
                          {item.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
            </div>
            <RichTextEditor
              id={`monday-mark-comment-${update.id}`}
              label={`Comment for ${update.student.name}`}
              value={comment}
              onChange={setComment}
            />
            {error ? (
              <InlineNotice tone="warning">
                <span role="alert">{error}</span>
              </InlineNotice>
            ) : null}
            {message ? (
              <span className="sr-only" role="status">
                {message}
              </span>
            ) : null}
            <div className={rowClass}>
              <Button
                type="button"
                disabled={pending || !mark}
                onClick={() => void save()}
              >
                {pending ? 'Saving…' : 'Save mark'}
              </Button>
            </div>
          </div>
        </details>
        {otherMarks.length ? (
          <div className="grid gap-2 border-t border-border pt-4">
            <h4 className={subTitleClass}>Other mentor marks</h4>
            <ul className={metaListClass}>
              {otherMarks.map((record) => (
                <li key={record.id}>
                  <strong className="font-semibold text-foreground">
                    {record.mentor.name}
                  </strong>{' '}
                  <MarkBadge mark={record.mark} />
                  <RichTextContent
                    html={record.commentHtml}
                    empty="No comment."
                  />
                </li>
              ))}
            </ul>
          </div>
        ) : null}
      </Card>
    </article>
  );
}

function ReviewUpdateList({
  updates,
}: {
  updates: MondayUpdateReviewUpdate[];
}) {
  if (updates.length === 0)
    return <p className={mutedClass}>No update in this range.</p>;
  return (
    <ul className={metaListClass}>
      {updates.map((update) => (
        <li key={update.id} className="grid gap-2">
          <h3 className={subTitleClass}>
            {updateRangeLabel(update)}
            <span className={mutedClass}>
              {' '}
              · {hoursLabel(update.hours)} · {formatDateTime(update.createdAt)}
            </span>
          </h3>
          <RichTextContent
            html={update.contentHtml}
            empty="No written content."
          />
          {update.marks.length === 0 ? (
            <p className={mutedClass}>No mentor mark yet.</p>
          ) : (
            <ul className={metaListClass}>
              {update.marks.map((record) => (
                <li key={record.id}>
                  <strong className="font-semibold text-foreground">
                    {record.mentor.name}
                  </strong>{' '}
                  <MarkBadge mark={record.mark} />
                  <RichTextContent
                    html={record.commentHtml}
                    empty="No comment."
                  />
                </li>
              ))}
            </ul>
          )}
        </li>
      ))}
    </ul>
  );
}

function ReviewerTable({
  rows,
  loading,
  error,
  onRetry,
}: {
  rows: MondayUpdateReviewRow[];
  loading: boolean;
  error: boolean;
  onRetry: () => void;
}) {
  const columns = useMemo<ColumnDef<MondayUpdateReviewRow, any>[]>(
    () => [
      {
        id: 'expand',
        header: 'Details',
        enableSorting: false,
        enableHiding: false,
        cell: ({ row }) => (
          <Button
            variant="ghost"
            size="icon"
            type="button"
            aria-label={`${row.getIsExpanded() ? 'Hide' : 'Show'} details for ${row.original.student.name}`}
            aria-expanded={row.getIsExpanded()}
            onClick={row.getToggleExpandedHandler()}
          >
            {row.getIsExpanded() ? (
              <IconChevronUp size={18} aria-hidden="true" />
            ) : (
              <IconChevronDown size={18} aria-hidden="true" />
            )}
          </Button>
        ),
      },
      {
        accessorKey: 'student.name',
        id: 'student',
        header: 'Student',
        cell: ({ row }) => (
          <div>
            <Link to={`/people/${row.original.student.slug}`}>
              {row.original.student.name}
            </Link>
            <div className={mutedClass}>{row.original.student.slug}</div>
          </div>
        ),
      },
      {
        id: 'leetcode',
        header: 'LeetCode',
        accessorFn: (row) =>
          row.leetcodeCounts.easy +
          row.leetcodeCounts.medium +
          row.leetcodeCounts.hard,
        cell: ({ row }) => (
          <div className="grid gap-1">
            <span>
              <Badge variant="secondary">
                {row.original.leetcodeCounts.easy} /{' '}
                {row.original.leetcodeCounts.medium} /{' '}
                {row.original.leetcodeCounts.hard}
              </Badge>
            </span>
            <div className={mutedClass}>easy / medium / hard</div>
          </div>
        ),
      },
      {
        accessorKey: 'mockCount',
        header: 'Mocks',
        cell: ({ getValue }) => `${getValue()} in range`,
      },
      {
        accessorKey: 'updateCount',
        header: 'Updates',
        cell: ({ getValue }) => `${getValue()} in range`,
      },
    ],
    [],
  );

  return (
    <DataTable
      ariaLabel="Monday update review"
      data={rows}
      columns={columns}
      loading={loading}
      error={error}
      onRetry={onRetry}
      emptyTitle="No students to review"
      emptyMessage="No enrolled student has activity in this date range yet."
      getRowId={(row) => row.student.id}
      renderExpanded={(row) => (
        <div className="mt-3">
          <ReviewUpdateList updates={row.original.updates} />
        </div>
      )}
      renderCard={(row) => (
        <div className="grid gap-2">
          <h3 className={cardTitleClass}>
            <Link to={`/people/${row.original.student.slug}`}>
              {row.original.student.name}
            </Link>
          </h3>
          <p className={mutedClass}>{row.original.student.slug}</p>
          <div className={rowClass}>
            <Badge variant="secondary">
              {row.original.leetcodeCounts.easy} /{' '}
              {row.original.leetcodeCounts.medium} /{' '}
              {row.original.leetcodeCounts.hard}
            </Badge>
            <span className={mutedClass}>easy / medium / hard</span>
          </div>
          <p className={mutedClass}>
            {row.original.mockCount} mock interviews ·{' '}
            {row.original.updateCount} updates
          </p>
          <ReviewUpdateList updates={row.original.updates} />
        </div>
      )}
    />
  );
}
