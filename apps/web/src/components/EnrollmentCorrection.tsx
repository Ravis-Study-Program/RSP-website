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
import { useDraftSection, type DraftSection } from '@/components/draftSections';
import { Field, FieldLabel } from '@/components/ui/field';
import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import {
  demoMode,
  seasonEnrollmentsQueryKey,
  useEnrollmentCandidates,
  useSeasonEnrollments,
  useSeasonMentorships,
} from '@/api/queries';
import type {
  Enrollment,
  EnrollmentPage,
  Mentorship,
} from '@/api/generated/models';
import type { Season } from '@/types';

const noMentor = 'none';

/**
 * Corrections to an enrollment: which season or person it belongs to (or no
 * season, which removes them), and a student's mentor. Shown as a section of
 * the manage dialog opened from a table's row menu.
 */
export function EnrollmentCorrectionForm({
  enrollment,
  seasons,
  name,
  onSaved,
  register,
}: {
  enrollment: Enrollment;
  seasons: Season[];
  name: string;
  onSaved?: () => void;
  /** Hands the edits to the dialog, which commits them on Confirm. */
  register?: (section: DraftSection | null) => void;
}) {
  const client = useQueryClient();
  const [seasonId, setSeasonId] = useState(enrollment.seasonId);
  const [userId, setUserId] = useState(enrollment.userId);
  // A student's mentor, from the season's current mentorships.
  const student = enrollment.role === 'student';
  const mentorships = useSeasonMentorships(
    student ? enrollment.seasonId : undefined,
  );
  const members = useSeasonEnrollments(
    student ? enrollment.seasonId : undefined,
  );
  const mentorship = mentorships.data?.items.find(
    (item: Mentorship) => item.studentUserId === enrollment.userId,
  );
  const currentMentor = mentorship?.mentorUserId ?? noMentor;
  const [mentorId, setMentorId] = useState<string | null>(null);
  const chosenMentor = mentorId ?? currentMentor;
  const mentorItems = [
    { label: '(No Mentor)', value: noMentor },
    ...(members.data?.items ?? [])
      .filter(
        (member) =>
          member.state === 'active' &&
          (member.role === 'mentor' || member.role === 'coordinator'),
      )
      .map((member) => ({
        label: member.member?.name ?? member.userId,
        value: member.userId,
      })),
  ];
  const candidates = useEnrollmentCandidates(seasonId, true);
  const placementChanged =
    userId !== enrollment.userId || seasonId !== enrollment.seasonId;
  const mentorChanged = chosenMentor !== currentMentor;
  const dirty = placementChanged || mentorChanged;
  const seasonItems = [
    ...seasons
      .filter((s) => s.status === 'open')
      .map((s) => ({ label: s.name, value: s.id })),
  ];
  const personItems = [
    { label: name, value: enrollment.userId },
    ...(candidates.data?.items
      .filter((p) => p.id !== enrollment.userId)
      .map((p) => ({ label: `${p.name} (${p.slug})`, value: p.id })) ?? []),
  ];
  // A student's mentor changes on its own: an existing mentorship moves to
  // the new mentor, or is created or ended.
  async function saveMentor() {
    const base = `/seasons/${encodeURIComponent(enrollment.seasonId)}/mentorships`;
    if (demoMode) return;
    if (chosenMentor === noMentor && mentorship)
      await apiRequest(`${base}/${encodeURIComponent(mentorship.id)}`, {
        method: 'DELETE',
      });
    else if (mentorship)
      await apiRequest(`${base}/${encodeURIComponent(mentorship.id)}`, {
        method: 'PATCH',
        body: JSON.stringify({
          mentorUserId: chosenMentor,
          studentUserId: enrollment.userId,
        }),
      });
    else
      await apiRequest(base, {
        method: 'POST',
        body: JSON.stringify({
          mentorUserId: chosenMentor,
          studentUserId: enrollment.userId,
        }),
      });
  }
  async function save() {
    if (!placementChanged && mentorChanged) {
      await saveMentor();
      await client.invalidateQueries();
      onSaved?.();
      return;
    }
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
  }
  useDraftSection(
    dirty && !candidates.isLoading && !candidates.isError,
    save,
    register,
  );
  return (
    <section
      aria-labelledby={`enrollment-correction-${enrollment.id}`}
      className="grid gap-3"
    >
      <h3
        id={`enrollment-correction-${enrollment.id}`}
        className="text-base font-semibold"
      >
        Corrections
      </h3>
      <form
        id={`enrollment-correction-form-${enrollment.id}`}
        className="grid gap-4"
        onSubmit={(e) => {
          e.preventDefault();
        }}
      >
        <Field>
          <FieldLabel htmlFor="enrollment-correction-season">
            Correct Season
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
            Correct Person
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
        {student ? (
          <Field>
            <FieldLabel htmlFor="enrollment-correction-mentor">
              Correct Mentor
            </FieldLabel>
            <Select
              items={mentorItems}
              value={chosenMentor}
              onValueChange={(value) => setMentorId(value ?? noMentor)}
              // A season or person change clears mentors; save that first.
              disabled={placementChanged || mentorships.isLoading}
            >
              <SelectTrigger
                id="enrollment-correction-mentor"
                className="w-full"
              >
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
      </form>
    </section>
  );
}
