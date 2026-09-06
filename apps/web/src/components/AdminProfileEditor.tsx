import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import { demoMode } from '@/api/queries';
import type { AdminProfile, UserPrivate } from '@/api/generated/models';
import styles from '@/styles/App.module.css';

export function AdminProfileEditor({
  user,
  onSaved,
}: {
  user: UserPrivate;
  onSaved: (value: AdminProfile) => void;
}) {
  const query = useQuery({
    queryKey: ['admin-profile', user.id],
    queryFn: () =>
      demoMode
        ? Promise.resolve({
            id: user.id,
            name: user.name,
            slug: user.slug,
            avatarUrl: user.avatarUrl ?? null,
            email: user.email,
            discordId: null,
          } satisfies AdminProfile)
        : apiRequest<AdminProfile>(`/admin/users/${user.id}/profile`),
  });
  if (query.isLoading) return <p role="status">Loading profile details…</p>;
  if (!query.data)
    return (
      <p role="alert">
        Profile details could not be loaded.{' '}
        <button onClick={() => void query.refetch()}>Retry</button>
      </p>
    );
  return <ProfileForm key={user.id} profile={query.data} onSaved={onSaved} />;
}
function ProfileForm({
  profile,
  onSaved,
}: {
  profile: AdminProfile;
  onSaved: (value: AdminProfile) => void;
}) {
  const client = useQueryClient();
  const [form, setForm] = useState(profile);
  const [email, setEmail] = useState('');
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  async function save(emailOnly: boolean) {
    setPending(true);
    setMessage('');
    setError('');
    try {
      if (emailOnly) {
        if (!demoMode)
          await apiRequest(`/admin/users/${profile.id}/email-change`, {
            method: 'POST',
            body: JSON.stringify({ email }),
          });
        setMessage(
          'Verification sent to the new address. The current email remains until it is verified.',
        );
        setEmail('');
      } else {
        const updated = demoMode
          ? form
          : await apiRequest<AdminProfile>(
              `/admin/users/${profile.id}/profile`,
              {
                method: 'PATCH',
                body: JSON.stringify({
                  name: form.name,
                  slug: form.slug,
                  avatarUrl: form.avatarUrl || null,
                  discordId: form.discordId || null,
                }),
              },
            );
        client.setQueryData(['admin-profile', profile.id], updated);
        onSaved(updated);
        await Promise.all([
          client.invalidateQueries({ queryKey: ['admin-users'] }),
          client.invalidateQueries({ queryKey: ['people'] }),
          client.invalidateQueries({ queryKey: ['users'] }),
          client.invalidateQueries({ queryKey: ['me'] }),
          client.invalidateQueries({ queryKey: ['season-people'] }),
          client.invalidateQueries({ queryKey: ['season-enrollments'] }),
          client.invalidateQueries({
            queryKey: ['mock-interview-participants'],
          }),
        ]);
        setMessage('Profile updated.');
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Unable to save the profile.');
    } finally {
      setPending(false);
    }
  }
  return (
    <section aria-label="Edit profile and contact details">
      <h3>Profile and contact details</h3>
      <form
        className={styles.formStack}
        onSubmit={(e) => {
          e.preventDefault();
          void save(false);
        }}
      >
        <label className={styles.field}>
          Name
          <input
            className={styles.input}
            value={form.name}
            required
            maxLength={100}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
          />
        </label>
        <label className={styles.field}>
          Profile slug
          <input
            className={styles.input}
            value={form.slug}
            required
            minLength={3}
            maxLength={50}
            onChange={(e) => setForm({ ...form, slug: e.target.value })}
          />
        </label>
        <label className={styles.field}>
          Avatar URL
          <input
            className={styles.input}
            type="url"
            value={form.avatarUrl ?? ''}
            placeholder="https://…"
            onChange={(e) => setForm({ ...form, avatarUrl: e.target.value })}
          />
        </label>
        <label className={styles.field}>
          Discord
          <input
            className={styles.input}
            value={form.discordId ?? ''}
            maxLength={100}
            onChange={(e) => setForm({ ...form, discordId: e.target.value })}
          />
        </label>
        <button className={styles.buttonPrimary} disabled={pending}>
          Save profile
        </button>
      </form>
      <form
        className={styles.formStack}
        onSubmit={(e) => {
          e.preventDefault();
          void save(true);
        }}
      >
        <p>Current email: {profile.email}</p>
        <label className={styles.field}>
          New email address
          <input
            className={styles.input}
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </label>
        <p className={styles.helper}>
          The recipient must verify the new address before their sign-in email
          changes.
        </p>
        <button className={styles.buttonSecondary} disabled={pending}>
          Send email verification
        </button>
      </form>
      {error ? <p role="alert">{error}</p> : null}
      {message ? <p role="status">{message}</p> : null}
    </section>
  );
}
