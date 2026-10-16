import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Field, FieldDescription, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import { demoMode } from '@/api/queries';
import type { AdminProfile, UserPrivate } from '@/api/generated/models';

const notice = 'text-sm text-muted-foreground';
const formStack = 'grid gap-4';

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
  if (query.isLoading)
    return (
      <p role="status" className={notice}>
        Loading profile details…
      </p>
    );
  if (!query.data)
    return (
      <Alert variant="destructive">
        <AlertDescription className="flex flex-wrap items-center gap-3 text-destructive">
          Profile details could not be loaded.
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => void query.refetch()}
          >
            Retry
          </Button>
        </AlertDescription>
      </Alert>
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
    <section
      aria-label="Edit profile and contact details"
      className="grid gap-6"
    >
      <h3 className="text-base font-semibold">Profile and contact details</h3>
      <form
        className={formStack}
        onSubmit={(e) => {
          e.preventDefault();
          void save(false);
        }}
      >
        <Field>
          <FieldLabel htmlFor="admin-profile-name">Name</FieldLabel>
          <Input
            id="admin-profile-name"
            value={form.name}
            required
            maxLength={100}
            onChange={(e) => setForm({ ...form, name: e.target.value })}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="admin-profile-slug">Profile slug</FieldLabel>
          <Input
            id="admin-profile-slug"
            value={form.slug}
            required
            minLength={3}
            maxLength={50}
            onChange={(e) => setForm({ ...form, slug: e.target.value })}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="admin-profile-avatar">Avatar URL</FieldLabel>
          <Input
            id="admin-profile-avatar"
            type="url"
            value={form.avatarUrl ?? ''}
            placeholder="https://…"
            onChange={(e) => setForm({ ...form, avatarUrl: e.target.value })}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="admin-profile-discord">Discord</FieldLabel>
          <Input
            id="admin-profile-discord"
            value={form.discordId ?? ''}
            maxLength={100}
            onChange={(e) => setForm({ ...form, discordId: e.target.value })}
          />
        </Field>
        <div>
          <Button type="submit" disabled={pending}>
            Save profile
          </Button>
        </div>
      </form>
      <form
        className={`${formStack} border-t pt-6`}
        onSubmit={(e) => {
          e.preventDefault();
          void save(true);
        }}
      >
        <p className={notice}>Current email: {profile.email}</p>
        <Field>
          <FieldLabel htmlFor="admin-profile-new-email">
            New email address
          </FieldLabel>
          <Input
            id="admin-profile-new-email"
            type="email"
            required
            value={email}
            aria-describedby="admin-profile-new-email-help"
            onChange={(e) => setEmail(e.target.value)}
          />
          <FieldDescription id="admin-profile-new-email-help">
            The recipient must verify the new address before their sign-in email
            changes.
          </FieldDescription>
        </Field>
        <div>
          <Button variant="outline" type="submit" disabled={pending}>
            Send email verification
          </Button>
        </div>
      </form>
      {error ? (
        <Alert variant="destructive">
          <AlertDescription className="text-destructive">
            {error}
          </AlertDescription>
        </Alert>
      ) : null}
      {message ? (
        <p role="status" className={notice}>
          {message}
        </p>
      ) : null}
    </section>
  );
}
