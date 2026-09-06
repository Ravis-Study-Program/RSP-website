import { IconArrowUp } from '@tabler/icons-react';
import { useQueryClient } from '@tanstack/react-query';
import { useId, useState } from 'react';

import { apiRequest } from '@/api/client';
import type { Enrollment, StudentLevelUpdate } from '@/api/generated/models';
import { demoMode } from '@/api/queries';
import { FormDialog } from '@/components/Dialogs';
import { studentLevels } from '@/studentLevels';
import styles from '@/styles/App.module.css';
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
        <p className={styles.fieldError} role="alert">
          {error}
        </p>
      ) : null}
      <div className={styles.field}>
        <label htmlFor={fieldId}>Student level</label>
        <select
          id={fieldId}
          className={styles.select}
          value={level}
          disabled={pending}
          onChange={(event) =>
            setLevel(event.target.value as keyof typeof studentLevels)
          }
        >
          {Object.entries(studentLevels).map(([value, label]) => (
            <option key={value} value={value}>
              {label}
            </option>
          ))}
        </select>
      </div>
      <div className={styles.dialogFooter}>
        <button
          type="button"
          className={styles.button}
          disabled={
            pending || !person.enrollmentId || level === person.studentLevel
          }
          onClick={() => void save()}
        >
          {pending ? 'Saving…' : 'Save level'}
        </button>
      </div>
    </FormDialog>
  );
}
