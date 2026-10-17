import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Button } from '@/components/ui/button';
import { Field, FieldLabel } from '@/components/ui/field';
import { IconArrowUp } from '@tabler/icons-react';
import { useQueryClient } from '@tanstack/react-query';
import { useId, useState } from 'react';

import { apiRequest } from '@/api/client';
import type { Enrollment, StudentLevelUpdate } from '@/api/generated/models';
import { demoMode } from '@/api/queries';
import { FormDialog } from '@/components/Dialogs';
import { studentLevels } from '@/studentLevels';
import type { Page, Person } from '@/types';

export function StudentLevelDialog({
  seasonId,
  person,
  onChanged,
}: {
  seasonId: string;
  person: Person;
  onChanged: () => void;
}) {
  const queryClient = useQueryClient();
  const fieldId = useId();
  const levelItems = Object.entries(studentLevels).map(([value, label]) => ({
    label,
    value,
  }));
  const [open, setOpen] = useState(false);
  const [level, setLevel] = useState<keyof typeof studentLevels>('novice');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const save = async () => {
    if (!person.enrollmentId) return;
    setPending(true);
    setError('');
    try {
      if (!demoMode) {
        const body: StudentLevelUpdate = { studentLevel: level };
        await apiRequest<Enrollment>(
          `/seasons/${encodeURIComponent(seasonId)}/members/${encodeURIComponent(person.enrollmentId)}/student-level`,
          { method: 'PATCH', body: JSON.stringify(body) },
        );
      }
      queryClient.setQueryData<Page<Person>>(
        ['season-people', seasonId],
        (current) =>
          current
            ? {
                ...current,
                items: current.items.map((item) =>
                  item.enrollmentId === person.enrollmentId
                    ? { ...item, studentLevel: level }
                    : item,
                ),
              }
            : current,
      );
      if (!demoMode) {
        onChanged();
        void queryClient.invalidateQueries({
          queryKey: ['season-enrollments', seasonId],
        });
      }
      setOpen(false);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to update the student level.',
      );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      triggerVariant="outline"
      triggerSize="sm"
      title={`Set ${person.name}'s level`}
      description="Choose this student's level for the season."
      trigger={
        <>
          <IconArrowUp size={17} aria-hidden="true" /> Set level
        </>
      }
      open={open}
      onOpenChange={(next) => {
        if (pending) return;
        setOpen(next);
        if (next) {
          setLevel(
            person.studentLevel && person.studentLevel !== 'not_applicable'
              ? person.studentLevel
              : 'novice',
          );
          setError('');
        }
      }}
    >
      {error ? (
        <p className="text-sm text-destructive" role="alert">
          {error}
        </p>
      ) : null}
      <Field>
        <FieldLabel htmlFor={fieldId}>Student level</FieldLabel>
        <Select
          items={levelItems}
          value={level}
          disabled={pending}
          onValueChange={(value) => {
            if (value) setLevel(value as keyof typeof studentLevels);
          }}
        >
          <SelectTrigger id={fieldId} className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              {levelItems.map((item) => (
                <SelectItem key={item.value} value={item.value}>
                  {item.label}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </Field>
      <div className="flex flex-wrap justify-end gap-2">
        <Button
          type="button"
          disabled={
            pending || !person.enrollmentId || level === person.studentLevel
          }
          onClick={() => void save()}
        >
          {pending ? 'Saving…' : 'Save level'}
        </Button>
      </div>
    </FormDialog>
  );
}
