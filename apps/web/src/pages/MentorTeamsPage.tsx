import { IconArrowRight, IconPlus, IconUsersGroup } from '@tabler/icons-react';
import { Link } from 'react-router-dom';

import { usePeople } from '@/api/queries';
import { PageHeader, PersonIdentity, usePageTitle } from '@/components/Common';
import { FormDialog } from '@/components/Dialogs';
import { InlineNotice, PageSkeleton } from '@/components/StatusViews';
import styles from '@/styles/App.module.css';

export function MentorTeamsPage() {
  usePageTitle('Mentor teams');
  const query = usePeople();
  if (query.isLoading) return <PageSkeleton label="Loading mentor teams" />;
  const mentors = query.data?.items.filter((person) => person.roles.includes('mentor')) ?? [];
  const students = query.data?.items.filter((person) => person.roles.includes('student')) ?? [];
  return (
    <div className={styles.page}>
      <PageHeader
        eyebrow="Season operations"
        title="Mentor teams"
        description="See assignments at a glance, find unassigned students and balance each mentor's team."
        actions={<FormDialog title="Create mentorship" description="Choose one active mentor and one active student in this season." trigger={<><IconPlus size={18} aria-hidden="true" /> Assign student</>}><InlineNotice>Assignment validates season consistency and prevents duplicate active mentorships.</InlineNotice><div className={styles.dialogActions}><Link className={styles.button} to="/admin/mentorships">Open mentorship administration</Link></div></FormDialog>}
      />
      {students.some((student) => student.status === 'unassigned') ? <InlineNotice tone="warning"><IconUsersGroup size={19} aria-hidden="true" /> {students.filter((student) => student.status === 'unassigned').length} student needs a mentor assignment.</InlineNotice> : null}
      <div className={styles.cardGrid}>
        {mentors.map((mentor, index) => (
          <article className={styles.card} key={mentor.id}>
            <PersonIdentity person={mentor} privateView />
            <div className={styles.sectionHeader} style={{ marginTop: '1rem' }}><h2 className={styles.cardTitle}>Assigned students</h2><span className={styles.badge}>{Math.min(2, students.length)}</span></div>
            <ul className={styles.cleanList}>
              {students.filter((_, studentIndex) => studentIndex % Math.max(mentors.length, 1) === index).map((student) => <li className={styles.personRow} key={student.id}><PersonIdentity person={student} /><Link to={`/people/${student.slug}`} aria-label={`Open ${student.name}`}><IconArrowRight size={18} aria-hidden="true" /></Link></li>)}
            </ul>
          </article>
        ))}
      </div>
      <section className={styles.section}><div className={styles.sectionHeader}><h2 className={styles.sectionTitle}>Unassigned students</h2></div><div className={styles.panel}><ul className={styles.cleanList}>{students.filter((student) => student.status === 'unassigned').map((student) => <li className={styles.personRow} key={student.id}><PersonIdentity person={student} privateView /><button className={styles.buttonSecondary} type="button">Assign mentor</button></li>)}</ul></div></section>
    </div>
  );
}
