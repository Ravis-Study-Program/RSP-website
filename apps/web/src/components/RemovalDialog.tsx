import { Button } from '@/components/ui/button';
import { Field, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { useState } from 'react';

import { apiRequest } from '@/api/client';
import type { Enrollment } from '@/api/generated/models';
import { demoMode } from '@/api/queries';
import { FormDialog, DialogCancel } from '@/components/Dialogs';
import type { Person } from '@/types';

/** Removes a member from the season, opened from their row menu. */
export function RemovalDialog({
  seasonId,
  person,
  onRemoved,
  open,
  onOpenChange,
}: {
  seasonId?: string;
  person: Person;
  onRemoved: () => void;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const [reason, setReason] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const remove = async () => {
    if (demoMode) {
      onRemoved();
      onOpenChange(false);
      return;
    }
    if (!seasonId || !person.enrollmentId) return;
    setPending(true);
    setError('');
    try {
      if (!demoMode) {
        const body = {
          reason: reason.trim(),
        };
        await apiRequest<Enrollment>(
          `/seasons/${encodeURIComponent(seasonId)}/members/${encodeURIComponent(person.enrollmentId)}/remove`,
          { method: 'POST', body: JSON.stringify(body) },
        );
      }
      onRemoved();
      onOpenChange(false);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to remove this member.',
      );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      title={`Remove ${person.name}?`}
      description="They immediately lose access to season resources and the directory. Your name and the removal reason are recorded."
      open={open}
      onOpenChange={onOpenChange}
    >
      <Field>
        <FieldLabel htmlFor={`reason-${person.id}`}>Removal Reason</FieldLabel>
        <Textarea
          id={`reason-${person.id}`}
          required
          value={reason}
          onChange={(event) => setReason(event.target.value)}
        />
      </Field>
      <Field>
        <FieldLabel htmlFor={`name-${person.id}`}>
          Type <strong>{person.name}</strong> to confirm
        </FieldLabel>
        <Input
          id={`name-${person.id}`}
          value={confirmation}
          onChange={(event) => setConfirmation(event.target.value)}
        />
      </Field>
      {error ? (
        <p className="text-sm text-destructive" role="alert">
          {error}
        </p>
      ) : null}
      <div className="button-group">
        <DialogCancel />
        <Button
          variant="destructive"
          type="button"
          disabled={
            pending ||
            reason.trim().length < 5 ||
            confirmation !== person.name ||
            (!demoMode && (!seasonId || !person.enrollmentId))
          }
          onClick={() => void remove()}
        >
          {pending ? 'Removing…' : 'Remove Member'}
        </Button>
      </div>
    </FormDialog>
  );
}
