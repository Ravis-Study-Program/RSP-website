import { isValidEmail } from '@/utils';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { useDraftSection, type DraftSection } from '@/components/draftSections';
import { Field, FieldLabel } from '@/components/ui/field';
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
  register,
}: {
  user: UserPrivate;
  onSaved: (value: AdminProfile) => void;
  /** Hands the edits to the dialog, which commits them on Confirm. */
  register?: (section: DraftSection | null) => void;
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
  return (
    <ProfileForm
      key={user.id}
      profile={query.data}
      onSaved={onSaved}
      register={register}
    />
  );
}
function ProfileForm({
  profile,
  onSaved,
  register,
}: {
  profile: AdminProfile;
  onSaved: (value: AdminProfile) => void;
  register?: (section: DraftSection | null) => void;
}) {
  const client = useQueryClient();
  const [form, setForm] = useState(profile);
  // The email starts as the current one; changing it asks the person to
  // verify the new address.
  const [email, setEmail] = useState(profile.email);
  const newEmail = email.trim().toLowerCase();
  const dirty =
    form.name !== profile.name ||
    form.slug !== profile.slug ||
    (newEmail !== '' && newEmail !== profile.email.toLowerCase());
  async function save() {
    const messages: string[] = [];
    if (form.name !== profile.name || form.slug !== profile.slug) {
      const updated = demoMode
        ? form
        : await apiRequest<AdminProfile>(`/admin/users/${profile.id}/profile`, {
            method: 'PATCH',
            body: JSON.stringify({ name: form.name, slug: form.slug }),
          });
      client.setQueryData(['admin-profile', profile.id], updated);
      onSaved(updated);
      await Promise.all(
        [
          ['admin-users'],
          ['people'],
          ['users'],
          ['me'],
          ['season-people'],
          ['season-enrollments'],
          ['mock-interview-participants'],
        ].map((queryKey) => client.invalidateQueries({ queryKey })),
      );
    }
    if (newEmail && newEmail !== profile.email.toLowerCase()) {
      if (!isValidEmail(newEmail))
        throw new Error(
          'Enter a valid email address, such as name@example.com.',
        );
      if (!demoMode)
        await apiRequest(`/admin/users/${profile.id}/email-change`, {
          method: 'POST',
          body: JSON.stringify({ email: newEmail }),
        });
      messages.push(
        'Verification sent to the new address. The current email stays until it is verified.',
      );
    }
    return messages.join(' ') || undefined;
  }
  useDraftSection(dirty, save, register);
  return (
    <section
      aria-label="Edit profile and contact details"
      className="grid gap-4"
    >
      <form
        id="admin-profile-form"
        className={formStack}
        onSubmit={(e) => {
          e.preventDefault();
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
          <FieldLabel htmlFor="admin-profile-slug">Username</FieldLabel>
          <Input
            id="admin-profile-slug"
            value={form.slug}
            required
            minLength={2}
            maxLength={50}
            onChange={(e) => setForm({ ...form, slug: e.target.value })}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="admin-profile-email">Email Address</FieldLabel>
          <Input
            id="admin-profile-email"
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </Field>
      </form>
    </section>
  );
}
