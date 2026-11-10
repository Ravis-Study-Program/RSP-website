import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  IconDotsVertical,
  IconMail,
  IconSend,
  IconX,
} from '@tabler/icons-react';
import { Field, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useState, type ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import {
  demoMode,
  fetchAllPages,
  usePeople,
  useSeasonEnrollments,
} from '@/api/queries';
import type { Invitation, InvitationCreate } from '@/api/generated/models';
import { LabelHint } from '@/components/Common';
import { FormDialog } from '@/components/Dialogs';
import type { Season } from '@/types';

const formStack = 'grid gap-4';
const notice = 'text-sm text-muted-foreground';
const invitationRoleItems = [
  { label: 'Student', value: 'student' },
  { label: 'Mentor', value: 'mentor' },
  { label: 'Coordinator', value: 'coordinator' },
];

export interface InvitationParts {
  /** The header button that opens the invitation form. */
  invite: ReactNode;
  invitations: Invitation[];
  loading: boolean;
  error: boolean;
  refetch: () => void;
  /** A pending or expired invitation's row menu: resend or cancel. */
  menuFor: (invitation: Invitation) => ReactNode;
  /** The latest invitation outcome, for a status line above the table. */
  status: string;
}

/**
 * A season's invitations, handed to `render` in parts so the page can put the
 * invite button in its header and the invitations in its table. The invite
 * form starts on that season but can invite to any of `seasons`; accepting
 * enrols the person in the invitation's season. A student can be invited
 * together with a mentor from that season.
 */
export function Invitations({
  season,
  seasons,
  render,
}: {
  season: Season;
  /** The seasons the invite form offers. */
  seasons: Season[];
  render: (parts: InvitationParts) => ReactNode;
}) {
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
  const [inviteSeasonId, setInviteSeasonId] = useState(season.id);
  const inviteSeason = seasons.find((item) => item.id === inviteSeasonId);
  // Mentors come from the season the invitation is for.
  const people = usePeople(open);
  const enrollments = useSeasonEnrollments(open ? inviteSeasonId : undefined);
  const mentors = (people.data?.items ?? []).filter((person) =>
    (enrollments.data?.items ?? []).some(
      (enrollment) =>
        enrollment.userId === person.id &&
        enrollment.state === 'active' &&
        (enrollment.role === 'mentor' || enrollment.role === 'coordinator'),
    ),
  );
  const seasonItems = seasons.map((item) => ({
    label: item.name,
    value: item.id,
  }));
  const mentorItems = [
    { label: 'No mentor yet', value: '' },
    ...mentors.map((mentor) => ({ label: mentor.name, value: mentor.id })),
  ];
  async function save(invitation?: Invitation, action?: 'resend' | 'cancel') {
    // A new invitation goes to the season chosen in the form.
    const target = invitation ? season : (inviteSeason ?? season);
    const targetKey = ['invitations', target.id];
    setBusy(true);
    setMessage('');
    // A mentor only goes with a student invitation.
    const body: InvitationCreate = {
      name: form.name,
      email: form.email,
      role: form.role,
      ...(form.role === 'student' && form.mentorUserId
        ? { mentorUserId: form.mentorUserId }
        : {}),
    };
    try {
      const at = new Date();
      const saved: Invitation = demoMode
        ? {
            id: invitation?.id ?? crypto.randomUUID(),
            ...form,
            ...invitation,
            seasonId: target.id,
            seasonName: target.name,
            seasonSlug: target.slug,
            expiresAt: new Date(at.getTime() + 7 * 86400000).toISOString(),
            sentAt: at.toISOString(),
            status: action === 'cancel' ? 'cancelled' : 'pending',
            mentorUserId: invitation?.mentorUserId ?? form.mentorUserId ?? null,
            acceptedBy: null,
          }
        : await apiRequest<Invitation>(
            `/seasons/${target.id}/invitations${invitation ? `/${invitation.id}/${action}` : ''}`,
            {
              method: 'POST',
              ...(invitation ? {} : { body: JSON.stringify(body) }),
            },
          );
      if (demoMode) {
        const others = (
          client.getQueryData<{ items: Invitation[] }>(targetKey)?.items ?? []
        ).filter((i) => i.id !== saved.id);
        client.setQueryData(targetKey, {
          items: [...others, saved],
          totalCount: others.length + 1,
          pageInfo: { hasMore: false, nextCursor: null, previousCursor: null },
        });
      } else await client.invalidateQueries({ queryKey: targetKey });
      setMessage(
        saved.status === 'cancelled'
          ? 'Invitation cancelled.'
          : saved.sentAt
            ? `Invitation email queued for ${target.name}. The recipient has 7 days to accept.`
            : 'Invitation saved, but the email could not be queued. Use Resend to try again.',
      );
      setOpen(false);
      setForm({ name: '', email: '', role: 'student' });
      setInviteSeasonId(season.id);
    } catch (e) {
      setMessage(
        e instanceof Error ? e.message : 'Unable to update this invitation.',
      );
    } finally {
      setBusy(false);
    }
  }
  const menuFor = (invitation: Invitation) =>
    ['pending', 'expired'].includes(invitation.status) &&
    season.status === 'open' ? (
      <DropdownMenu>
        <DropdownMenuTrigger
          render={<Button variant="ghost" size="icon" disabled={busy} />}
          aria-label={`More actions for ${invitation.name}`}
        >
          <IconDotsVertical size={18} aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem onClick={() => void save(invitation, 'resend')}>
            <IconSend size={17} aria-hidden="true" /> Resend
          </DropdownMenuItem>
          <DropdownMenuItem
            variant="destructive"
            onClick={() => void save(invitation, 'cancel')}
          >
            <IconX size={17} aria-hidden="true" /> Cancel Invitation
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    ) : null;
  const invite = (
    <FormDialog
      open={open}
      onOpenChange={(next) => {
        if (
          !next &&
          (form.name || form.email) &&
          !window.confirm('Discard this unsent invitation?')
        )
          return;
        if (next) setInviteSeasonId(season.id);
        setOpen(next);
      }}
      title="Invite a Member"
      trigger={
        season.status === 'open' ? (
          <>
            <IconMail size={17} aria-hidden="true" /> Invite a Member
          </>
        ) : undefined
      }
    >
      <form
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
        className={formStack}
      >
        <Field>
          <FieldLabel htmlFor="invitation-season">Season</FieldLabel>
          <Select
            items={seasonItems}
            value={inviteSeasonId}
            onValueChange={(value) => {
              if (!value) return;
              setInviteSeasonId(value);
              // Mentors differ by season, so the mentor starts over.
              setForm({ ...form, mentorUserId: undefined });
            }}
          >
            <SelectTrigger id="invitation-season" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {seasonItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
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
          <Select
            items={invitationRoleItems}
            value={form.role}
            onValueChange={(value) => {
              if (value)
                setForm({
                  ...form,
                  role: value as InvitationCreate['role'],
                });
            }}
          >
            <SelectTrigger id="invitation-role" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {invitationRoleItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
        {form.role === 'student' ? (
          <Field>
            <FieldLabel htmlFor="invitation-mentor">
              Mentor <LabelHint>optional</LabelHint>
            </FieldLabel>
            <Select
              items={mentorItems}
              value={form.mentorUserId ?? ''}
              onValueChange={(value) =>
                setForm({ ...form, mentorUserId: value || undefined })
              }
            >
              <SelectTrigger id="invitation-mentor" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {mentorItems.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </Field>
        ) : null}
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
          <Button type="submit" disabled={busy || !inviteSeason}>
            {busy ? 'Sending…' : 'Send Invitation'}
          </Button>
        </div>
      </form>
    </FormDialog>
  );
  return render({
    invite,
    invitations: query.data?.items ?? [],
    loading: query.isLoading,
    error: query.isError,
    refetch: () => void query.refetch(),
    menuFor,
    status: open ? '' : message,
  });
}
