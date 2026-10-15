import { useQueryClient, type QueryClient } from '@tanstack/react-query';
import type { ColumnDef } from '@tanstack/react-table';
import { IconChevronDown, IconChevronUp } from '@tabler/icons-react';
import { useMemo, useState, type FormEvent } from 'react';
import { Link, Navigate, useParams } from 'react-router-dom';

import {
  defaultMondayUpdateRange,
  type MondayUpdateRange,
} from '@/activityDates';
import { apiRequest } from '@/api/client';
import type {
  AttemptOutcome,
  MondayUpdate,
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
  useSeasonMentorships,
  useSeasonPeople,
  useSeasons,
  type MondayUpdateFilters,
} from '@/api/queries';
import { AppDatePicker } from '@/components/AppDatePicker';
import { PageHeader, usePageTitle } from '@/components/Common';
import { DataTable } from '@/components/DataTable';
import { NamedConfirmation } from '@/components/Dialogs';
import { RichTextContent } from '@/components/RichTextContent';
import { RichTextEditor } from '@/components/RichTextEditor';
import {
  EmptyState,
  ErrorState,
  InlineNotice,
  PageSkeleton,
} from '@/components/StatusViews';
import styles from '@/styles/App.module.css';
import { formatDate, formatDateTime, outcomeLabel } from '@/utils';

type Experience = 'student' | 'mentor' | 'reviewer';

const markOptions: { value: MondayUpdateMark; label: string }[] = [
  { value: 'way_ahead', label: 'Way ahead' },
  { value: 'doing_good', label: 'Doing good' },
  { value: 'on_track', label: 'On track' },
  { value: 'a_bit_behind', label: 'A bit behind' },
  { value: 'at_risk', label: 'At risk' },
];

const rangeErrorText = 'The end date must not be before the start date.';

function markLabel(mark: MondayUpdateMark) {
  return markOptions.find((option) => option.value === mark)?.label ?? mark;
}

function updateRangeLabel(update: { startDate: string; endDate: string }) {
  return `${formatDate(update.startDate)} – ${formatDate(update.endDate)}`;
}

function rangeReversed(range: MondayUpdateRange) {
  return Boolean(
    range.startDate && range.endDate && range.endDate < range.startDate,
  );
}

function outcomeClass(outcome: AttemptOutcome) {
  if (outcome === 'independently_solved') return styles.badgeSuccess;
  if (outcome === 'not_solved') return styles.badgeDanger;
  return styles.badgeWarning;
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
      <div className={styles.page}>
        <ErrorState
          onRetry={() => void Promise.all([seasons.refetch(), user.refetch()])}
        />
      </div>
    );
  if (!season)
    return (
      <div className={styles.page}>
        <ErrorState
          title="Season not found"
          message="This season may have moved or you may not have access."
        />
      </div>
    );
  if (!actor || !experience) return <Navigate to="/forbidden" replace />;

  const reversed = rangeReversed(range);
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
    <div className={styles.page}>
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
      <section className={styles.section} aria-labelledby="monday-range-title">
        <div className={styles.sectionHeader}>
          <h2 id="monday-range-title" className={styles.sectionTitle}>
            Date range
          </h2>
          <span className={styles.muted}>
            Updates that overlap this range are shown.
          </span>
        </div>
        <div className={styles.fieldGrid}>
          <AppDatePicker
            label="Start date"
            granularity="day"
            value={range.startDate}
            onChange={(startDate) =>
              setRange((current) => ({ ...current, startDate }))
            }
          />
          <AppDatePicker
            label="End date"
            granularity="day"
            value={range.endDate}
            error={reversed ? rangeErrorText : undefined}
            onChange={(endDate) =>
              setRange((current) => ({ ...current, endDate }))
            }
          />
        </div>
      </section>
      {experience === 'student' ? (
        <>
          <StudentUpdateForm seasonId={season.id} range={range} />
          <section
            className={styles.section}
            aria-labelledby="monday-history-title"
          >
            <div className={styles.sectionHeader}>
              <h2 id="monday-history-title" className={styles.sectionTitle}>
                Your updates
              </h2>
              <span className={styles.muted}>{ownUpdates.length} posted</span>
            </div>
            {updates.isLoading ? (
              <p role="status">Loading your updates…</p>
            ) : updates.isError ? (
              <ErrorState
                title="Your updates could not be loaded"
                onRetry={() => void updates.refetch()}
              />
            ) : ownUpdates.length === 0 ? (
              <EmptyState
                title="No updates yet"
                message="Post your first update for this range so your mentor can see what you worked on."
              />
            ) : (
              <div className={styles.cardGrid}>
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
        <section
          className={styles.section}
          aria-labelledby="monday-mentor-title"
        >
          <div className={styles.sectionHeader}>
            <h2 id="monday-mentor-title" className={styles.sectionTitle}>
              Student updates
            </h2>
            <span className={styles.muted}>
              {selectedStudentIds.length} of {menteeIds.length} students
              selected
            </span>
          </div>
          <fieldset className={styles.unstyledFieldset}>
            <legend className={styles.legend}>Students</legend>
            <p className={styles.helper}>
              Every student you mentor is selected by default. Uncheck a student
              to leave their updates out.
            </p>
            {menteeIds.length === 0 ? (
              <p className={styles.muted}>
                No students are assigned to you in this season.
              </p>
            ) : (
              <div className={styles.fieldGrid}>
                {menteeIds.map((studentId) => (
                  <label className={styles.checkLabel} key={studentId}>
                    <input
                      type="checkbox"
                      checked={!unselectedStudentIds.includes(studentId)}
                      onChange={(event) =>
                        setUnselectedStudentIds((ids) =>
                          event.target.checked
                            ? ids.filter((id) => id !== studentId)
                            : [...ids, studentId],
                        )
                      }
                    />
                    {studentLabel(studentId)}
                  </label>
                ))}
              </div>
            )}
          </fieldset>
          {menteeIds.length === 0 ? (
            <EmptyState
              title="No students assigned"
              message="A coordinator can assign students to your mentor team."
            />
          ) : selectedStudentIds.length === 0 ? (
            <EmptyState
              title="No students selected"
              message="Select at least one student above to review their Monday updates."
            />
          ) : updates.isLoading ? (
            <p role="status">Loading updates for your students…</p>
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
                  className={styles.section}
                  key={studentId}
                  aria-labelledby={`monday-student-${studentId}`}
                >
                  <div className={styles.sectionHeader}>
                    <h3
                      id={`monday-student-${studentId}`}
                      className={styles.sectionTitle}
                    >
                      {studentLabel(studentId)}
                    </h3>
                    {studentUpdates.length ? (
                      <span className={styles.badge}>
                        {studentUpdates.length === 1
                          ? '1 update in this range'
                          : `${studentUpdates.length} updates in this range`}
                      </span>
                    ) : null}
                  </div>
                  {studentUpdates.length === 0 ? (
                    <p className={styles.muted}>No update in this range.</p>
                  ) : (
                    <div className={styles.cardGrid}>
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
        <section
          className={styles.section}
          aria-labelledby="monday-review-title"
        >
          <div className={styles.sectionHeader}>
            <h2 id="monday-review-title" className={styles.sectionTitle}>
              Student review
            </h2>
            <span className={styles.muted}>
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
}: {
  seasonId: string;
  range: MondayUpdateRange;
}) {
  const queryClient = useQueryClient();
  const [content, setContent] = useState('');
  const [pending, setPending] = useState(false);
  const [attempted, setAttempted] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const contentEmpty = !hasContent(content);

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setAttempted(true);
    setError('');
    setMessage('');
    if (rangeReversed(range) || contentEmpty) return;
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
            } satisfies MondayUpdateMutation),
          },
        );
      setContent('');
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
    <section className={styles.section} aria-labelledby="monday-post-title">
      <div className={styles.sectionHeader}>
        <h2 id="monday-post-title" className={styles.sectionTitle}>
          Post an update
        </h2>
        <span className={styles.muted}>
          Your update covers the date range above.
        </span>
      </div>
      <form className={styles.form} onSubmit={(event) => void submit(event)}>
        {error ? (
          <InlineNotice tone="warning">
            <span role="alert">{error}</span>
          </InlineNotice>
        ) : null}
        {message ? (
          <span className={styles.visuallyHidden} role="status">
            {message}
          </span>
        ) : null}
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
        <div className={styles.buttonRow}>
          <button className={styles.button} type="submit" disabled={pending}>
            {pending ? 'Posting…' : 'Post update'}
          </button>
        </div>
      </form>
    </section>
  );
}

function UpdateActivity({ update }: { update: MondayUpdate }) {
  const attempts = update.activity.attempts;
  const mocks = update.activity.mocks;
  return (
    <div className={styles.expandedDetails}>
      <h4 className={styles.cardTitle}>Attempts ({attempts.length})</h4>
      {attempts.length === 0 ? (
        <p className={styles.muted}>No practice attempts in this range.</p>
      ) : (
        <ul className={styles.metaList}>
          {attempts.map((attempt) => (
            <li className={styles.listRow} key={attempt.id}>
              <span>
                <strong>
                  {attempt.problemUrl ? (
                    <a
                      href={attempt.problemUrl}
                      target="_blank"
                      rel="noreferrer"
                    >
                      {attempt.problem}
                    </a>
                  ) : (
                    attempt.problem
                  )}
                </strong>
                <span className={styles.helper}>
                  {' '}
                  · {formatDate(attempt.attemptedAt)}
                </span>
              </span>
              <span className={styles.inline}>
                {attempt.difficulty ? (
                  <span className={styles.badge}>{attempt.difficulty}</span>
                ) : null}
                <span className={outcomeClass(attempt.outcome)}>
                  {outcomeLabel(attempt.outcome)}
                </span>
                <span className={styles.helper}>{attempt.minutes} min</span>
              </span>
            </li>
          ))}
        </ul>
      )}
      <h4 className={`${styles.cardTitle} ${styles.sectionHeaderSpaced}`}>
        Mock interviews ({mocks.length})
      </h4>
      {mocks.length === 0 ? (
        <p className={styles.muted}>No mock interviews in this range.</p>
      ) : (
        <ul className={styles.metaList}>
          {mocks.map((mock) => (
            <li className={styles.listRow} key={mock.id}>
              <span>{formatDateTime(mock.occurredAt)}</span>
              <span className={styles.helper}>{mock.durationMinutes} min</span>
            </li>
          ))}
        </ul>
      )}
    </div>
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
    <article className={styles.card}>
      <div className={styles.sectionHeader}>
        <h3 className={styles.cardTitle}>{label}</h3>
        <span className={styles.helper}>
          Posted {formatDateTime(update.createdAt)}
        </span>
      </div>
      <RichTextContent html={update.contentHtml} empty="No written content." />
      <UpdateActivity update={update} />
      <div className={styles.buttonRow}>
        <NamedConfirmation
          name={label}
          actionLabel="Delete update"
          description="This permanently removes the update. The practice and mock activity it covers stays in your history."
          onConfirm={deleteUpdate}
        />
      </div>
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
    <article className={styles.card}>
      <h3 className={styles.cardTitle}>{label}</h3>
      <RichTextContent html={update.contentHtml} empty="No written content." />
      <UpdateActivity update={update} />
      <div className={styles.fieldGrid}>
        <div className={styles.field}>
          <label htmlFor={`monday-mark-${update.id}`}>Mark for {label}</label>
          <select
            id={`monday-mark-${update.id}`}
            className={styles.select}
            value={mark}
            onChange={(event) =>
              setMark(event.target.value as MondayUpdateMark | '')
            }
          >
            <option value="">No mark yet</option>
            {markOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </div>
      </div>
      <RichTextEditor
        id={`monday-mark-comment-${update.id}`}
        label={`Comment for ${label}`}
        value={comment}
        onChange={setComment}
      />
      {error ? (
        <InlineNotice tone="warning">
          <span role="alert">{error}</span>
        </InlineNotice>
      ) : null}
      {message ? (
        <span className={styles.visuallyHidden} role="status">
          {message}
        </span>
      ) : null}
      <div className={styles.buttonRow}>
        <button
          className={styles.button}
          type="button"
          disabled={pending || !mark}
          onClick={() => void save()}
        >
          {pending ? 'Saving…' : 'Save mark'}
        </button>
      </div>
      {otherMarks.length ? (
        <div className={styles.helperBlock}>
          <h4 className={styles.cardTitle}>Other mentor marks</h4>
          <ul className={styles.metaList}>
            {otherMarks.map((record) => (
              <li key={record.id}>
                <strong>{record.mentor.name}</strong>{' '}
                <span className={styles.badge}>{markLabel(record.mark)}</span>
                <RichTextContent
                  html={record.commentHtml}
                  empty="No comment."
                />
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </article>
  );
}

function ReviewUpdateList({
  updates,
}: {
  updates: MondayUpdateReviewUpdate[];
}) {
  if (updates.length === 0)
    return <p className={styles.muted}>No update in this range.</p>;
  return (
    <ul className={styles.metaList}>
      {updates.map((update) => (
        <li key={update.id}>
          <h4 className={styles.cardTitle}>
            {updateRangeLabel(update)}
            <span className={styles.helper}>
              {' '}
              · {formatDateTime(update.createdAt)}
            </span>
          </h4>
          <RichTextContent
            html={update.contentHtml}
            empty="No written content."
          />
          {update.marks.length === 0 ? (
            <p className={styles.muted}>No mentor mark yet.</p>
          ) : (
            <ul className={styles.metaList}>
              {update.marks.map((record) => (
                <li key={record.id}>
                  <strong>{record.mentor.name}</strong>{' '}
                  <span className={styles.badge}>{markLabel(record.mark)}</span>
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
          <button
            className={styles.iconButton}
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
          </button>
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
            <div className={styles.helper}>{row.original.student.slug}</div>
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
          <div>
            <span className={styles.badge}>
              {row.original.leetcodeCounts.easy} /{' '}
              {row.original.leetcodeCounts.medium} /{' '}
              {row.original.leetcodeCounts.hard}
            </span>
            <div className={styles.helper}>easy / medium / hard</div>
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
        <div className={styles.expandedDetails}>
          <ReviewUpdateList updates={row.original.updates} />
        </div>
      )}
      renderCard={(row) => (
        <div>
          <h3 className={styles.cardTitle}>
            <Link to={`/people/${row.original.student.slug}`}>
              {row.original.student.name}
            </Link>
          </h3>
          <p className={styles.helper}>{row.original.student.slug}</p>
          <div className={styles.inline}>
            <span className={styles.badge}>
              {row.original.leetcodeCounts.easy} /{' '}
              {row.original.leetcodeCounts.medium} /{' '}
              {row.original.leetcodeCounts.hard}
            </span>
            <span className={styles.helper}>easy / medium / hard</span>
          </div>
          <p className={styles.helper}>
            {row.original.mockCount} mock interviews ·{' '}
            {row.original.updateCount} updates
          </p>
          <ReviewUpdateList updates={row.original.updates} />
        </div>
      )}
    />
  );
}
