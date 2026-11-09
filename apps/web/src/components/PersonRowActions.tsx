import {
  IconDotsVertical,
  IconEdit,
  IconShieldLock,
  IconUserMinus,
} from '@tabler/icons-react';
import { useState } from 'react';

import type { Enrollment } from '@/api/generated/models';
import { ManagePersonDialog } from '@/components/ManagePersonDialog';
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
  const [open, setOpen] = useState<'details' | 'manage' | 'remove' | null>(
    null,
  );
  const allowed = personActions(viewer, person, season);
  const canCorrect = allowed.correct && Boolean(enrollment);
  if (
    !allowed.editDetails &&
    !canCorrect &&
    !allowed.remove &&
    !allowed.manageAccount
  )
    return null;
  const close = (next: boolean) => {
    if (!next) setOpen(null);
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
              <IconEdit size={17} aria-hidden="true" /> Edit details
            </DropdownMenuItem>
          ) : null}
          {canCorrect || allowed.manageAccount ? (
            <DropdownMenuItem onClick={() => setOpen('manage')}>
              <IconShieldLock size={17} aria-hidden="true" /> Manage account
            </DropdownMenuItem>
          ) : null}
          {allowed.remove ? (
            <DropdownMenuItem
              variant="destructive"
              onClick={() => setOpen('remove')}
            >
              <IconUserMinus size={17} aria-hidden="true" /> Remove from season
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
          onOpenChange={close}
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
          onChanged={onChanged}
          onOpenChange={close}
        />
      ) : null}
      {open === 'remove' ? (
        <RemovalDialog
          seasonId={season?.id}
          person={person}
          onRemoved={onChanged}
          open
          onOpenChange={close}
        />
      ) : null}
    </>
  );
}
