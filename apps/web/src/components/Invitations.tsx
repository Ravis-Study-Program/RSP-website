import { Button } from '@/components/ui/button';
import { Field, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select';
import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import { demoMode, fetchAllPages } from '@/api/queries';
import type { Invitation, InvitationCreate } from '@/api/generated/models';
import { FormDialog } from '@/components/Dialogs';
import { DataTable } from '@/components/DataTable';
import type { Season } from '@/types';
import { formatDateTime } from '@/utils';

const formStack = 'grid gap-4';
const notice = 'text-sm text-muted-foreground';

export function Invitations({ season }: { season: Season }) {
  const client = useQueryClient();
  const key = ['invitations', season.id];
  const query = useQuery({
    queryKey: key,
    queryFn: () =>
      demoMode
        ? Promise.resolve({
            items: [] as Invitation[],
            totalCount: 0,
            pageInfo: {
              hasMore: false,
              nextCursor: null,
              previousCursor: null,
            },
          })
        : fetchAllPages<Invitation>(
            `/seasons/${season.id}/invitations?limit=100`,
          ),
  });
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState<InvitationCreate>({
    name: '',
    email: '',
    role: 'student',
  });
  async function save(invitation?: Invitation, action?: 'resend' | 'cancel') {
    setBusy(true);
    setMessage('');
    try {
      const at = new Date();
      const saved: Invitation = demoMode
        ? {
            id: invitation?.id ?? crypto.randomUUID(),
            ...form,
            ...invitation,
            seasonId: season.id,
            seasonName: season.name,
            seasonSlug: season.slug,
            expiresAt: new Date(at.getTime() + 7 * 86400000).toISOString(),
            sentAt: at.toISOString(),
            status: action === 'cancel' ? 'cancelled' : 'pending',
          }
        : await apiRequest<Invitation>(
            `/seasons/${season.id}/invitations${invitation ? `/${invitation.id}/${action}` : ''}`,
            {
              method: 'POST',
              ...(invitation ? {} : { body: JSON.stringify(form) }),
            },
          );
      if (demoMode)
        client.setQueryData(key, {
          items: [
            ...(query.data?.items ?? []).filter((i) => i.id !== saved.id),
            saved,
          ],
          totalCount:
            (query.data?.items ?? []).filter((i) => i.id !== saved.id).length +
            1,
          pageInfo: { hasMore: false, nextCursor: null, previousCursor: null },
        });
      else await client.invalidateQueries({ queryKey: key });
      setMessage(
        saved.status === 'cancelled'
          ? 'Invitation cancelled.'
          : saved.sentAt
            ? 'Invitation email queued. The recipient has 7 days to accept.'
            : 'Invitation saved, but the email could not be queued. Use Resend to try again.',
      );
      setOpen(false);
      setForm({ name: '', email: '', role: 'student' });
    } catch (e) {
      setMessage(
        e instanceof Error ? e.message : 'Unable to update this invitation.',
      );
    } finally {
      setBusy(false);
    }
  }
  const actions = (invitation: Invitation) =>
    ['pending', 'expired'].includes(invitation.status) &&
    season.status === 'open' ? (
      <div className="flex flex-wrap items-center gap-2.5">
        <Button
          variant="outline"
          size="sm"
          type="submit"
          disabled={busy}
          onClick={() => void save(invitation, 'resend')}
        >
          Resend
        </Button>
        <Button
          variant="destructive"
          size="sm"
          type="submit"
          disabled={busy}
          onClick={() => void save(invitation, 'cancel')}
        >
          Cancel invitation
        </Button>
      </div>
    ) : null;
  return (
    <section className="mt-6" aria-label="Invitations">
      <div className="mb-3 flex items-end justify-between gap-4">
        <h2 className="text-xl font-semibold tracking-tight">Invitations</h2>
        <FormDialog
          open={open}
          onOpenChange={(next) => {
            if (
              !next &&
              (form.name || form.email) &&
              !window.confirm('Discard this unsent invitation?')
            )
              return;
            setOpen(next);
          }}
          title="Invite a member"
          description="The recipient joins this season after accepting with the matching verified email address."
          trigger={season.status === 'open' ? 'Invite a member' : undefined}
        >
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void save();
            }}
            className={formStack}
          >
            <Field>
              <FieldLabel htmlFor="invitation-name">Name</FieldLabel>
              <Input
                id="invitation-name"
                required
                maxLength={100}
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="invitation-email">Email</FieldLabel>
              <Input
                id="invitation-email"
                type="email"
                required
                maxLength={254}
                value={form.email}
                onChange={(e) => setForm({ ...form, email: e.target.value })}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="invitation-role">Role</FieldLabel>
              <NativeSelect
                id="invitation-role"
                className="w-full"
                value={form.role}
                onChange={(e) =>
                  setForm({
                    ...form,
                    role: e.target.value as InvitationCreate['role'],
                  })
                }
              >
                <NativeSelectOption value="student">Student</NativeSelectOption>
                <NativeSelectOption value="mentor">Mentor</NativeSelectOption>
                <NativeSelectOption value="coordinator">
                  Coordinator
                </NativeSelectOption>
              </NativeSelect>
            </Field>
            {form.role === 'coordinator' ? (
              <p className={notice}>
                Coordinator access becomes active after MFA is configured.
              </p>
            ) : null}
            {/* Gate on open: the dialog stays mounted through its close
                animation, which would briefly duplicate the section status. */}
            {message && open ? (
              <p role="status" className={notice}>
                {message}
              </p>
            ) : null}
            <div>
              <Button type="submit" disabled={busy}>
                {busy ? 'Sending…' : 'Send invitation'}
              </Button>
            </div>
          </form>
        </FormDialog>
      </div>
      {message && !open ? (
        <p role="status" className={`${notice} mb-3`}>
          {message}
        </p>
      ) : null}
      <DataTable
        ariaLabel="Season invitations"
        data={query.data?.items ?? []}
        columns={[
          { accessorKey: 'name', header: 'Name' },
          { accessorKey: 'email', header: 'Email' },
          { accessorKey: 'role', header: 'Role' },
          {
            accessorKey: 'status',
            header: 'Status',
            cell: ({ row }) =>
              row.original.status === 'pending' && !row.original.sentAt
                ? 'Pending · email not queued'
                : statusLabel(row.original.status),
          },
          {
            accessorKey: 'expiresAt',
            header: 'Expires',
            cell: ({ getValue }) => formatDateTime(String(getValue())),
          },
          {
            id: 'actions',
            header: 'Actions',
            cell: ({ row }) => actions(row.original),
          },
        ]}
        loading={query.isLoading}
        error={query.isError}
        onRetry={() => void query.refetch()}
        emptyTitle="No invitations"
        emptyMessage="Invite someone using their name and email address."
        getRowId={(i) => i.id}
        renderCard={({ original: i }) => (
          <div>
            <strong>{i.name}</strong>
            <p className={notice}>
              {i.email} · {i.role} · {statusLabel(i.status)}
              {!i.sentAt && i.status === 'pending' ? ' · email not queued' : ''}
            </p>
            {actions(i)}
          </div>
        )}
      />
    </section>
  );
}

function statusLabel(status: string) {
  const text = status.replace(/_/g, ' ');
  return text.charAt(0).toUpperCase() + text.slice(1);
}
