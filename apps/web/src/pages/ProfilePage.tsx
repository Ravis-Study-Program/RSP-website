import { zodResolver } from '@hookform/resolvers/zod';
import { IconEdit, IconLock, IconMail, IconWorld } from '@tabler/icons-react';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { useParams } from 'react-router-dom';
import { z } from 'zod';

import { useCurrentUser, usePeople } from '@/api/queries';
import { MetricCard, RoleBadge, usePageTitle } from '@/components/Common';
import { FormDialog } from '@/components/Dialogs';
import { ErrorState, InlineNotice, PageSkeleton } from '@/components/StatusViews';
import { currentPerson } from '@/data/demo';
import styles from '@/styles/App.module.css';
import { formatDateTime, initials } from '@/utils';

const profileSchema = z.object({
  name: z.string().trim().min(2, 'Enter your name.').max(120),
  slug: z.string().regex(/^[a-z0-9]+(?:-[a-z0-9]+)*$/, 'Use lower-case letters, numbers and hyphens.'),
  timezone: z.string().min(1),
});
type ProfileValues = z.infer<typeof profileSchema>;

export function ProfilePage() {
  const { slug } = useParams();
  const people = usePeople();
  const user = useCurrentUser();
  const isSelf = !slug || slug === user.data?.slug;
  const person = isSelf ? currentPerson : people.data?.items.find((item) => item.slug === slug);
  const [savedName, setSavedName] = useState<string | null>(null);
  usePageTitle(person?.name ?? 'Profile');

  if (people.isLoading || user.isLoading) return <PageSkeleton label="Loading profile" />;
  if (people.isError || user.isError) return <div className={styles.page}><ErrorState onRetry={() => void Promise.all([people.refetch(), user.refetch()])} /></div>;
  if (!person) return <div className={styles.page}><ErrorState title="Profile not found" message="This person may no longer be visible to your account." /></div>;

  const displayName = savedName ?? person.name;
  return (
    <div className={styles.page}>
      <div className={styles.profileBanner}>
        <div className={styles.profileHeader}>
          <span className={styles.avatar}>{initials(displayName)}</span>
          <div><h1>{displayName}</h1><p>@{person.slug} · {person.season}</p></div>
        </div>
      </div>
      <div className={styles.buttonRow} style={{ justifyContent: 'space-between', marginTop: '1rem' }}>
        <div className={styles.inline}>{person.roles.map((role) => <RoleBadge role={role} key={role} />)}<span className={person.status === 'active' ? styles.badgeSuccess : styles.badgeNeutral}>{person.status}</span></div>
        {isSelf && user.data ? <EditProfileDialog user={user.data} onSaved={(values) => setSavedName(values.name)} /> : null}
      </div>
      {isSelf ? <InlineNotice tone="success"><IconLock size={19} aria-hidden="true" /> Private account details below are only visible to you and authorised programme administrators.</InlineNotice> : null}
      <div className={styles.metricGrid}>
        <MetricCard label="Practice attempts" value={person.attempts} detail="Public programme activity count" />
        <MetricCard label="Mock interviews" value={person.interviews} detail="Received and conducted" />
        <MetricCard label="Last active" value={formatDateTime(person.lastActiveAt).split(',')[0]} detail="Shown to approved members" />
      </div>
      <div className={styles.grid2}>
        <section className={styles.section}><div className={styles.sectionHeader}><h2 className={styles.sectionTitle}>Programme participation</h2></div><div className={styles.panel}><ul className={styles.cleanList}><li className={styles.listRow}><span>Current season</span><strong>{person.season}</strong></li><li className={styles.listRow}><span>Role badges</span><strong>{person.roles.join(', ')}</strong></li><li className={styles.listRow}><span>Activity visibility</span><strong>Members and alumni</strong></li></ul></div></section>
        {isSelf ? <section className={styles.section}><div className={styles.sectionHeader}><h2 className={styles.sectionTitle}>Private account details</h2></div><div className={styles.panel}><ul className={styles.cleanList}><li className={styles.listRow}><span className={styles.inline}><IconMail size={18} aria-hidden="true" /> Email</span><strong>{user.data?.email}</strong></li><li className={styles.listRow}><span className={styles.inline}><IconWorld size={18} aria-hidden="true" /> Timezone</span><strong>{user.data?.timezone}</strong></li><li className={styles.listRow}><span>Email verification</span><span className={styles.badgeSuccess}>Verified</span></li></ul></div></section> : null}
      </div>
    </div>
  );
}

function EditProfileDialog({ user, onSaved }: { user: { name: string; slug: string; timezone: string }; onSaved: (values: ProfileValues) => void }) {
  const [open, setOpen] = useState(false);
  const { register, handleSubmit, reset, formState: { errors, isDirty, isSubmitting } } = useForm<ProfileValues>({ resolver: zodResolver(profileSchema), defaultValues: user });
  const submit = handleSubmit(async (values) => {
    await Promise.resolve();
    onSaved(values);
    reset(values);
    setOpen(false);
  });
  const onOpenChange = (next: boolean) => {
    if (!next && isDirty && !window.confirm('Discard your profile changes?')) return;
    setOpen(next);
  };
  return (
    <FormDialog title="Edit profile" description="Your name and slug are visible to approved active members and alumni." open={open} onOpenChange={onOpenChange} trigger={<><IconEdit size={17} aria-hidden="true" /> Edit profile</>}>
      <form className={styles.form} noValidate onSubmit={submit}>
        <div className={styles.field}><label htmlFor="profile-name">Name</label><input id="profile-name" className={styles.input} aria-invalid={Boolean(errors.name)} {...register('name')} />{errors.name ? <p className={styles.fieldError}>{errors.name.message}</p> : null}</div>
        <div className={styles.field}><label htmlFor="profile-slug">Profile slug</label><input id="profile-slug" className={styles.input} aria-invalid={Boolean(errors.slug)} {...register('slug')} />{errors.slug ? <p className={styles.fieldError}>{errors.slug.message}</p> : null}</div>
        <div className={styles.field}><label htmlFor="profile-timezone">Timezone</label><select id="profile-timezone" className={styles.select} {...register('timezone')}><option>Australia/Adelaide</option><option>Australia/Sydney</option><option>Asia/Hong_Kong</option><option>UTC</option></select></div>
        <div className={styles.dialogActions}><button className={styles.buttonSecondary} type="button" onClick={() => onOpenChange(false)}>Cancel</button><button className={styles.button} type="submit" disabled={isSubmitting}>{isSubmitting ? 'Saving…' : 'Save profile'}</button></div>
      </form>
    </FormDialog>
  );
}
