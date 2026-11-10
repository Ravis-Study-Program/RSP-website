import type { Enrollment } from '@/api/generated/models';
import { FormDialog } from '@/components/Dialogs';
import { EnrollmentCorrectionForm } from '@/components/EnrollmentCorrection';
import { Separator } from '@/components/ui/separator';
import { UserAdministrationDialog } from '@/pages/AdminPage';
import type { Person, Season } from '@/types';

/**
 * Everything an administrator manages about one person, opened from a table
 * row's "Manage Account": correcting their enrollment's person or season,
 * and, for System Admins, their account itself.
 */
export function ManagePersonDialog({
  person,
  name,
  currentUserId,
  canManageAccount,
  enrollment,
  seasons,
  onRemove,
  onChanged,
  onOpenChange,
}: {
  person?: Person;
  name: string;
  currentUserId?: string;
  canManageAccount: boolean;
  /** The enrollment to correct; omit when it cannot be corrected. */
  enrollment?: Enrollment;
  seasons: Season[];
  /** Starts removing them from the season; omit when not allowed. */
  onRemove?: () => void;
  onChanged?: () => void;
  onOpenChange: (open: boolean) => void;
}) {
  const correction = enrollment ? (
    <EnrollmentCorrectionForm
      enrollment={enrollment}
      seasons={seasons}
      name={name}
      onRemove={onRemove}
      onSaved={() => {
        onChanged?.();
        onOpenChange(false);
      }}
    />
  ) : null;
  const seasonSections = correction;
  if (canManageAccount && person)
    return (
      <UserAdministrationDialog
        person={person}
        currentUserId={currentUserId}
        open
        onOpenChange={onOpenChange}
        before={
          seasonSections ? (
            <>
              {seasonSections}
              <Separator />
            </>
          ) : undefined
        }
      />
    );
  return (
    <FormDialog
      title={`Manage ${name}`}
      open
      onOpenChange={onOpenChange}
      contentClassName="sm:max-w-3xl"
    >
      <div className="flex flex-col gap-4">{seasonSections}</div>
    </FormDialog>
  );
}
