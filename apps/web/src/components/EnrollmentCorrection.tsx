import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Field, FieldLabel } from '@/components/ui/field';
import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import {
  demoMode,
  seasonEnrollmentsQueryKey,
  useEnrollmentCandidates,
} from '@/api/queries';
import type { Enrollment, EnrollmentPage } from '@/api/generated/models';
import type { Season } from '@/types';

/**
 * Correcting which person or season an enrollment belongs to. Shown as a
 * section of the manage dialog opened from a table's row menu.
 */
export function EnrollmentCorrectionForm({
  enrollment,
  seasons,
  name,
  onSaved,
}: {
  enrollment: Enrollment;
  seasons: Season[];
  name: string;
  onSaved?: () => void;
}) {
  const client = useQueryClient();
  const [seasonId, setSeasonId] = useState(enrollment.seasonId);
  const [userId, setUserId] = useState(enrollment.userId);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const candidates = useEnrollmentCandidates(seasonId, true);
  const dirty =
    userId !== enrollment.userId || seasonId !== enrollment.seasonId;
  const seasonItems = seasons
    .filter((s) => s.status === 'open')
    .map((s) => ({ label: s.name, value: s.id }));
  const personItems = [
    { label: name, value: enrollment.userId },
    ...(candidates.data?.items
      .filter((p) => p.id !== enrollment.userId)
      .map((p) => ({ label: `${p.name} (${p.slug})`, value: p.id })) ?? []),
  ];
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
      onSaved?.();
    } catch (e) {
      setError(
        e instanceof Error ? e.message : 'Unable to correct the enrollment.',
      );
    } finally {
      setPending(false);
    }
  }
  return (
    <section
      aria-labelledby={`enrollment-correction-${enrollment.id}`}
      className="grid gap-3"
    >
      <h3
        id={`enrollment-correction-${enrollment.id}`}
        className="text-base font-semibold"
      >
        Correct Person or Season
      </h3>
      <p className="text-sm text-muted-foreground">
        Participation dates are kept and affected mentor assignments are
        cleared. Recorded practice and mocks stay with their original owners.
      </p>
      <form
        className="grid gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        <Field>
          <FieldLabel htmlFor="enrollment-correction-season">
            Correct season
          </FieldLabel>
          <Select
            items={seasonItems}
            value={seasonId}
            onValueChange={(value) => setSeasonId(value ?? '')}
          >
            <SelectTrigger id="enrollment-correction-season" className="w-full">
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
          <FieldLabel htmlFor="enrollment-correction-person">
            Correct person
          </FieldLabel>
          <Select
            items={personItems}
            value={userId}
            onValueChange={(value) => setUserId(value ?? '')}
            disabled={candidates.isLoading}
          >
            <SelectTrigger id="enrollment-correction-person" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {personItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
        {candidates.isError ? (
          <Alert variant="destructive">
            <AlertDescription className="flex flex-wrap items-center gap-3 text-destructive">
              People could not be loaded.
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => void candidates.refetch()}
              >
                Retry
              </Button>
            </AlertDescription>
          </Alert>
        ) : null}
        {error ? (
          <Alert variant="destructive">
            <AlertDescription className="text-destructive">
              {error}
            </AlertDescription>
          </Alert>
        ) : null}
        <div>
          <Button
            type="submit"
            disabled={
              pending || !dirty || candidates.isLoading || candidates.isError
            }
          >
            {pending ? 'Correcting…' : 'Save correction'}
          </Button>
        </div>
      </form>
    </section>
  );
}
