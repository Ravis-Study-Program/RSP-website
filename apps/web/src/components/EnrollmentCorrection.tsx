import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select';
import { Button } from '@/components/ui/button';
import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import {
  demoMode,
  seasonEnrollmentsQueryKey,
  useEnrollmentCandidates,
} from '@/api/queries';
import type { Enrollment, EnrollmentPage } from '@/api/generated/models';
import { FormDialog } from '@/components/Dialogs';
import type { Season } from '@/types';
import styles from '@/styles/App.module.css';

export function EnrollmentCorrection({
  enrollment,
  seasons,
  name,
}: {
  enrollment: Enrollment;
  seasons: Season[];
  name: string;
}) {
  const client = useQueryClient();
  const [open, setOpen] = useState(false);
  const [seasonId, setSeasonId] = useState(enrollment.seasonId);
  const [userId, setUserId] = useState(enrollment.userId);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const candidates = useEnrollmentCandidates(seasonId, open);
  const dirty =
    userId !== enrollment.userId || seasonId !== enrollment.seasonId;
  async function save() {
    setPending(true);
    setError('');
    try {
      const saved: Enrollment = demoMode
        ? { ...enrollment, userId, seasonId, member: undefined }
        : await apiRequest<Enrollment>(
            `/seasons/${enrollment.seasonId}/members/${enrollment.id}/correction`,
            { method: 'POST', body: JSON.stringify({ userId, seasonId }) },
          );
      if (demoMode) {
        client.setQueryData<EnrollmentPage>(
          seasonEnrollmentsQueryKey(enrollment.seasonId),
          (current) =>
            current
              ? {
                  ...current,
                  items: current.items.filter((e) => e.id !== enrollment.id),
                  totalCount: current.totalCount - 1,
                }
              : current,
        );
        client.setQueryData<EnrollmentPage>(
          seasonEnrollmentsQueryKey(seasonId),
          (current) =>
            current
              ? {
                  ...current,
                  items: [
                    ...current.items.filter((e) => e.id !== saved.id),
                    saved,
                  ],
                  totalCount:
                    current.items.filter((e) => e.id !== saved.id).length + 1,
                }
              : {
                  items: [saved],
                  totalCount: 1,
                  pageInfo: {
                    hasMore: false,
                    nextCursor: null,
                    previousCursor: null,
                  },
                },
        );
      } else await client.invalidateQueries();
      setOpen(false);
    } catch (e) {
      setError(
        e instanceof Error ? e.message : 'Unable to correct the enrollment.',
      );
    } finally {
      setPending(false);
    }
  }
  return (
    <FormDialog
      title={`Correct enrollment for ${name}`}
      description="Choose the correct person and season. Participation dates are retained and affected mentor assignments are cleared. Recorded practice and mocks stay with their original owners."
      trigger="Correct person or season"
      open={open}
      onOpenChange={(next) => {
        if (pending) return;
        if (
          !next &&
          dirty &&
          !window.confirm('Discard this enrollment correction?')
        )
          return;
        if (next) {
          setSeasonId(enrollment.seasonId);
          setUserId(enrollment.userId);
          setError('');
        }
        setOpen(next);
      }}
    >
      <form
        className={styles.formStack}
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        <label className={styles.field}>
          Correct season
          <NativeSelect
            className="w-full"
            value={seasonId}
            onChange={(e) => setSeasonId(e.target.value)}
          >
            {seasons
              .filter((s) => s.status === 'open')
              .map((s) => (
                <NativeSelectOption value={s.id} key={s.id}>
                  {s.name}
                </NativeSelectOption>
              ))}
          </NativeSelect>
        </label>
        <label className={styles.field}>
          Correct person
          <NativeSelect
            className="w-full"
            value={userId}
            onChange={(e) => setUserId(e.target.value)}
            disabled={candidates.isLoading}
          >
            <NativeSelectOption value={enrollment.userId}>
              {name}
            </NativeSelectOption>
            {candidates.data?.items
              .filter((p) => p.id !== enrollment.userId)
              .map((p) => (
                <NativeSelectOption key={p.id} value={p.id}>
                  {p.name} ({p.slug})
                </NativeSelectOption>
              ))}
          </NativeSelect>
        </label>
        {candidates.isError ? (
          <p role="alert">
            People could not be loaded.{' '}
            <Button type="button" onClick={() => void candidates.refetch()}>
              Retry
            </Button>
          </p>
        ) : null}
        {error ? <p role="alert">{error}</p> : null}
        <Button
          type="submit"
          disabled={
            pending || !dirty || candidates.isLoading || candidates.isError
          }
        >
          {pending ? 'Correcting…' : 'Save correction'}
        </Button>
      </form>
    </FormDialog>
  );
}
