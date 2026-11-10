import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useDraftSection, type DraftSection } from '@/components/draftSections';
import { Field, FieldLabel } from '@/components/ui/field';
import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import {
  demoMode,
  seasonEnrollmentsQueryKey,
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
 * Edits to an enrollment: which season it belongs to, and a student's
 * mentor. Shown as a section of
 * the manage dialog opened from a table's row menu.
 */
export function EnrollmentCorrectionForm({
  enrollment,
  seasons,
  onSaved,
  register,
}: {
  enrollment: Enrollment;
  seasons: Season[];
  onSaved?: () => void;
  /** Hands the edits to the dialog, which commits them on Confirm. */
  register?: (section: DraftSection | null) => void;
}) {
  const client = useQueryClient();
  const [seasonId, setSeasonId] = useState(enrollment.seasonId);
  const [role, setRole] = useState<Enrollment['role']>(enrollment.role);
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
  const placementChanged = seasonId !== enrollment.seasonId;
  const roleChanged = role !== enrollment.role;
  const mentorChanged = chosenMentor !== currentMentor;
  const dirty = placementChanged || roleChanged || mentorChanged;
  const roleItems = [
    { label: 'Student', value: 'student' },
    { label: 'Mentor', value: 'mentor' },
    { label: 'Coordinator', value: 'coordinator' },
  ];
  const seasonItems = [
    ...seasons
      .filter((s) => s.status === 'open')
      .map((s) => ({ label: s.name, value: s.id })),
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
    if (roleChanged) {
      const level =
        enrollment.studentLevel && enrollment.studentLevel !== 'not_applicable'
          ? enrollment.studentLevel
          : 'novice';
      const updated: Enrollment = demoMode
        ? {
            ...enrollment,
            role,
            studentLevel: role === 'student' ? level : 'not_applicable',
          }
        : await apiRequest<Enrollment>(
            `/seasons/${enrollment.seasonId}/members/${enrollment.id}`,
            {
              method: 'PATCH',
              body: JSON.stringify({
                role,
                ...(role === 'student' ? { studentLevel: level } : {}),
              }),
            },
          );
      if (demoMode)
        client.setQueryData<EnrollmentPage>(
          seasonEnrollmentsQueryKey(enrollment.seasonId),
          (current) =>
            current
              ? {
                  ...current,
                  items: current.items.map((e) =>
                    e.id === updated.id ? updated : e,
                  ),
                }
              : current,
        );
      if (!placementChanged && !mentorChanged) {
        if (!demoMode) await client.invalidateQueries();
        onSaved?.();
        return;
      }
    }
    if (!placementChanged && mentorChanged) {
      await saveMentor();
      await client.invalidateQueries();
      onSaved?.();
      return;
    }
    const saved: Enrollment = demoMode
      ? { ...enrollment, seasonId }
      : await apiRequest<Enrollment>(
          `/seasons/${enrollment.seasonId}/members/${enrollment.id}/correction`,
          {
            method: 'POST',
            body: JSON.stringify({ userId: enrollment.userId, seasonId }),
          },
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
  useDraftSection(dirty, save, register);
  return (
    <section aria-label="Edit enrollment" className="grid gap-3">
      <form
        id={`enrollment-correction-form-${enrollment.id}`}
        className="grid gap-4"
        onSubmit={(e) => {
          e.preventDefault();
        }}
      >
        <Field>
          <FieldLabel htmlFor="enrollment-correction-role">
            Edit Role
          </FieldLabel>
          <Select
            items={roleItems}
            value={role}
            onValueChange={(value) =>
              setRole((value ?? role) as Enrollment['role'])
            }
          >
            <SelectTrigger id="enrollment-correction-role" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {roleItems.map((item) => (
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
              Edit Mentor
            </FieldLabel>
            <Select
              items={mentorItems}
              value={chosenMentor}
              onValueChange={(value) => setMentorId(value ?? noMentor)}
              // A season or person change clears mentors; save that first.
              disabled={
                placementChanged || roleChanged || mentorships.isLoading
              }
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
        <Field>
          <FieldLabel htmlFor="enrollment-correction-season">
            Edit Season
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
      </form>
    </section>
  );
}
