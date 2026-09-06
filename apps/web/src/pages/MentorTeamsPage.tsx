import { IconArrowRight, IconPlus, IconUsersGroup } from '@tabler/icons-react';
import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';

import { apiRequest } from '@/api/client';
import type { Mentorship, MentorshipCreate } from '@/api/generated/models';
import { demoMode, useSeasons, useSeasonPeople } from '@/api/queries';
import { PageHeader, PersonIdentity, usePageTitle } from '@/components/Common';
import { FormDialog } from '@/components/Dialogs';
import { InlineNotice, PageSkeleton } from '@/components/StatusViews';
import styles from '@/styles/App.module.css';

export function MentorTeamsPage() {
  usePageTitle('Mentor teams');
  const { slug } = useParams();
  const seasons = useSeasons();
  const season = seasons.data?.items.find((item) => item.slug === slug);
  const query = useSeasonPeople(season?.id, season?.name);
  if (query.isLoading) return <PageSkeleton label="Loading mentor teams" />;
  const mentors =
    query.data?.items.filter((person) => person.roles.includes('mentor')) ?? [];
  const students =
    query.data?.items.filter((person) => person.roles.includes('student')) ??
    [];
  return (
    <div className={styles.page}>
      <PageHeader
        eyebrow="Season operations"
        title="Mentor teams"
        description="See assignments at a glance, find unassigned students and balance each mentor's team."
        actions={
          <MentorshipDialog
            seasonId={season?.id}
            mentors={mentors}
            students={students}
            onCreated={() => void query.refetch()}
          />
        }
      />
      {students.some((student) => student.status === 'unassigned') ? (
        <InlineNotice tone="warning">
          <IconUsersGroup size={19} aria-hidden="true" />{' '}
          {students.filter((student) => student.status === 'unassigned').length}{' '}
          student needs a mentor assignment.
        </InlineNotice>
      ) : null}
      <div className={styles.cardGrid}>
        {mentors.map((mentor) => (
          <article className={styles.card} key={mentor.id}>
            <PersonIdentity person={mentor} seasonId={season?.id} privateView />
            <div
              className={`${styles.sectionHeader} ${styles.sectionHeaderSpaced}`}
            >
              <h2 className={styles.cardTitle}>Assigned students</h2>
              <span className={styles.badge}>
                {
                  students.filter(
                    (student) => student.mentorshipMentorId === mentor.id,
                  ).length
                }
              </span>
            </div>
            <ul className={styles.cleanList}>
              {students
                .filter((student) => student.mentorshipMentorId === mentor.id)
                .map((student) => (
                  <li className={styles.personRow} key={student.id}>
                    <PersonIdentity person={student} seasonId={season?.id} />
                    <Link
                      to={`/people/${student.slug}?seasonId=${encodeURIComponent(season!.id)}`}
                      aria-label={`Open ${student.name}`}
                    >
                      <IconArrowRight size={18} aria-hidden="true" />
                    </Link>
                  </li>
                ))}
            </ul>
          </article>
        ))}
      </div>
      <section className={styles.section}>
        <div className={styles.sectionHeader}>
          <h2 className={styles.sectionTitle}>Unassigned students</h2>
        </div>
        <div className={styles.panel}>
          <ul className={styles.cleanList}>
            {students
              .filter((student) => student.status === 'unassigned')
              .map((student) => (
                <li className={styles.personRow} key={student.id}>
                  <PersonIdentity
                    person={student}
                    seasonId={season?.id}
                    privateView
                  />
                  <MentorshipDialog
                    seasonId={season?.id}
                    mentors={mentors}
                    students={students}
                    initialStudentId={student.id}
                    onCreated={() => void query.refetch()}
                  />
                </li>
              ))}
          </ul>
        </div>
      </section>
    </div>
  );
}

function MentorshipDialog({
  seasonId,
  mentors,
  students,
  initialStudentId = '',
  onCreated,
}: {
  seasonId?: string;
  mentors: Array<{ id: string; name: string }>;
  students: Array<{ id: string; name: string; status: string | null }>;
  initialStudentId?: string;
  onCreated: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [mentorId, setMentorId] = useState('');
  const [studentId, setStudentId] = useState(initialStudentId);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const dirty = mentorId !== '' || studentId !== initialStudentId;
  const requestOpenChange = (next: boolean) => {
    if (
      !next &&
      dirty &&
      !window.confirm('Discard unsaved mentorship changes?')
    )
      return;
    if (next) {
      setMentorId('');
      setStudentId(initialStudentId);
      setError('');
    }
    setOpen(next);
  };
  const create = async () => {
    if (!seasonId || !mentorId || !studentId) return;
    setPending(true);
    setError('');
    try {
      if (!demoMode) {
        const body: MentorshipCreate = {
          mentorUserId: mentorId,
          studentUserId: studentId,
        };
        await apiRequest<Mentorship>(
          `/seasons/${encodeURIComponent(seasonId)}/mentorships`,
          { method: 'POST', body: JSON.stringify(body) },
        );
      }
      onCreated();
      setMentorId('');
      setStudentId(initialStudentId);
      setOpen(false);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to create this mentorship.',
      );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      title="Create mentorship"
      description="Choose one active mentor and one active student in this season."
      open={open}
      onOpenChange={requestOpenChange}
      trigger={
        <>
          <IconPlus size={18} aria-hidden="true" />{' '}
          {initialStudentId ? 'Assign mentor' : 'Assign student'}
        </>
      }
    >
      <div className={styles.form}>
        <InlineNotice>
          Assignment validates season consistency and prevents duplicate active
          mentorships.
        </InlineNotice>
        {error ? (
          <p className={styles.fieldError} role="alert">
            {error}
          </p>
        ) : null}
        <div className={styles.field}>
          <label htmlFor={`mentor-${initialStudentId || 'new'}`}>Mentor</label>
          <select
            id={`mentor-${initialStudentId || 'new'}`}
            className={styles.select}
            value={mentorId}
            onChange={(event) => setMentorId(event.target.value)}
          >
            <option value="">Choose a mentor</option>
            {mentors.map((mentor) => (
              <option key={mentor.id} value={mentor.id}>
                {mentor.name}
              </option>
            ))}
          </select>
        </div>
        <div className={styles.field}>
          <label htmlFor={`student-${initialStudentId || 'new'}`}>
            Student
          </label>
          <select
            id={`student-${initialStudentId || 'new'}`}
            className={styles.select}
            value={studentId}
            disabled={Boolean(initialStudentId)}
            onChange={(event) => setStudentId(event.target.value)}
          >
            <option value="">Choose a student</option>
            {students
              .filter((student) => student.status === 'unassigned')
              .map((student) => (
                <option key={student.id} value={student.id}>
                  {student.name}
                </option>
              ))}
          </select>
        </div>
        <div className={styles.dialogActions}>
          <button
            className={styles.buttonSecondary}
            type="button"
            disabled={pending}
            onClick={() => requestOpenChange(false)}
          >
            Cancel
          </button>
          <button
            className={styles.button}
            type="button"
            disabled={
              pending || !mentorId || !studentId || (!demoMode && !seasonId)
            }
            onClick={() => void create()}
          >
            {pending ? 'Assigning…' : 'Create mentorship'}
          </button>
        </div>
      </div>
    </FormDialog>
  );
}
