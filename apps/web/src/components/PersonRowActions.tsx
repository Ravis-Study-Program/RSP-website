import {
  IconDotsVertical,
  IconEdit,
  IconShieldLock,
  IconTrash,
  IconUserMinus,
} from '@tabler/icons-react';
import { useState } from 'react';

import type { Enrollment } from '@/api/generated/models';
import { ManagePersonDialog } from '@/components/ManagePersonDialog';
import { DeleteAccountDialog } from '@/pages/AdminPage';
import { RemovalDialog } from '@/components/RemovalDialog';
import { StudentDetailsDialog } from '@/components/StudentDetailsDialog';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { canSetStudentLevel } from '@/studentLevels';
import type { CurrentUser, Person, Season } from '@/types';

/**
 * What the viewer may do to one person in the People table. Season actions
 * need a season in view: a mentor manages their current and past students,
 * coordinators and administrators manage everyone. System Admins can also
 * manage the account itself, in any view.
 */
function personActions(
  viewer: CurrentUser | undefined,
  person: Person,
  season?: Season,
) {
  const isManager = Boolean(viewer?.globalRoles.length);
  const isSystemAdmin = Boolean(viewer?.globalRoles.includes('system_admin'));
  const membership = viewer?.seasonRoles.find(
    (entry) => entry.seasonId === season?.id && entry.state === 'active',
  );
  const isCoordinator = membership?.role === 'coordinator';
  const ownStudent =
    membership?.role === 'mentor' &&
    (person.mentorshipMentorId === viewer?.id ||
      (person.previousMentorIds ?? []).includes(viewer?.id ?? ''));
  const inSeason = Boolean(season && person.enrollmentId);
  const open = season?.status === 'open';
  const active = person.enrollmentState === 'active';
  const canAdminister =
    inSeason && open && (isManager || isCoordinator || ownStudent);
  return {
    editDetails:
      canAdminister &&
      active &&
      person.seasonRole === 'student' &&
      canSetStudentLevel(viewer, season),
    correct: inSeason && open && (isManager || isCoordinator),
    remove: canAdminister && active,
    manageAccount: isSystemAdmin,
  };
}

/** The People table's row menu; draws nothing when no action is allowed. */
export function PersonRowActions({
  viewer,
  person,
  season,
  seasons,
  enrollment,
  onChanged,
}: {
  viewer?: CurrentUser;
  person: Person;
  season?: Season;
  seasons: Season[];
  /** The person's enrollment in `season`, needed to correct it. */
  enrollment?: Enrollment;
  onChanged: () => void;
}) {
  const [open, setOpen] = useState<
    'details' | 'manage' | 'remove' | 'delete' | null
  >(null);
  const allowed = personActions(viewer, person, season);
  const canCorrect = allowed.correct && Boolean(enrollment);
  if (
    !allowed.editDetails &&
    !canCorrect &&
    !allowed.remove &&
    !allowed.manageAccount
  )
    return null;
  // A dialog only closes itself, so one that has just handed over to another
  // (the manage dialog starting a removal) does not close that one.
  const close = (kind: NonNullable<typeof open>) => (next: boolean) => {
    if (!next) setOpen((current) => (current === kind ? null : current));
  };
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={<Button variant="ghost" size="icon" />}
          aria-label={`More actions for ${person.name}`}
        >
          <IconDotsVertical size={18} aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          {allowed.editDetails ? (
            <DropdownMenuItem onClick={() => setOpen('details')}>
              <IconEdit size={17} aria-hidden="true" /> Edit Details
            </DropdownMenuItem>
          ) : null}
          {canCorrect || allowed.manageAccount ? (
            <DropdownMenuItem onClick={() => setOpen('manage')}>
              <IconShieldLock size={17} aria-hidden="true" /> Manage Account
            </DropdownMenuItem>
          ) : null}
          {/* Without corrections (a mentor and their student), removal is
              here; otherwise it is "(No Season)" in Manage Account. */}
          {allowed.remove && !canCorrect ? (
            <DropdownMenuItem
              variant="destructive"
              onClick={() => setOpen('remove')}
            >
              <IconUserMinus size={17} aria-hidden="true" /> Remove From Season
            </DropdownMenuItem>
          ) : null}
          {allowed.manageAccount && person.id !== viewer?.id ? (
            <DropdownMenuItem
              variant="destructive"
              onClick={() => setOpen('delete')}
            >
              <IconTrash size={17} aria-hidden="true" /> Delete
            </DropdownMenuItem>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
      {open === 'details' && season ? (
        <StudentDetailsDialog
          seasonId={season.id}
          person={person}
          onChanged={onChanged}
          open
          onOpenChange={close('details')}
        />
      ) : null}
      {open === 'manage' ? (
        <ManagePersonDialog
          person={person}
          name={person.name}
          currentUserId={viewer?.id}
          canManageAccount={allowed.manageAccount}
          enrollment={canCorrect ? enrollment : undefined}
          seasons={seasons}
          onRemove={allowed.remove ? () => setOpen('remove') : undefined}
          onChanged={onChanged}
          onOpenChange={close('manage')}
        />
      ) : null}
      {open === 'delete' ? (
        <DeleteAccountDialog
          person={person}
          onOpenChange={close('delete')}
          onDeleted={onChanged}
        />
      ) : null}
      {open === 'remove' ? (
        <RemovalDialog
          seasonId={season?.id}
          person={person}
          onRemoved={onChanged}
          open
          onOpenChange={close('remove')}
        />
      ) : null}
    </>
  );
}
