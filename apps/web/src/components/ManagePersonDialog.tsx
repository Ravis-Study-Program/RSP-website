import type { Enrollment } from '@/api/generated/models';
import { useState } from 'react';
import { DialogCancel, FormDialog } from '@/components/Dialogs';
import { Button } from '@/components/ui/button';
import { FieldError } from '@/components/ui/field';
import {
  useDraftSections,
  type DraftSection,
} from '@/components/draftSections';
import { EnrollmentCorrectionForm } from '@/components/EnrollmentCorrection';
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
  const { register, dirty, saveAll } = useDraftSections();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const correction = (
    register: (key: string) => (section: DraftSection | null) => void,
  ) =>
    enrollment ? (
      <EnrollmentCorrectionForm
        enrollment={enrollment}
        seasons={seasons}
        onSaved={() => onChanged?.()}
        register={register('corrections')}
      />
    ) : null;
  if (canManageAccount && person)
    return (
      <UserAdministrationDialog
        person={person}
        currentUserId={currentUserId}
        open
        onOpenChange={onOpenChange}
        onSaved={onChanged}
        onRemoveFromSeason={onRemove}
        before={correction}
      />
    );
  const confirm = async () => {
    setPending(true);
    setError('');
    try {
      await saveAll(['corrections']);
      onOpenChange(false);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to save the changes.',
      );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      title={`Manage ${name}`}
      open
      onOpenChange={onOpenChange}
      contentClassName="sm:max-w-3xl"
    >
      <div className="flex flex-col gap-4">
        {correction(register)}
        {error ? <FieldError>{error}</FieldError> : null}
        <div className="button-group">
          <DialogCancel />
          <Button
            type="button"
            disabled={pending || !dirty}
            onClick={() => void confirm()}
          >
            {pending ? 'Saving…' : 'Confirm'}
          </Button>
        </div>
      </div>
    </FormDialog>
  );
}
