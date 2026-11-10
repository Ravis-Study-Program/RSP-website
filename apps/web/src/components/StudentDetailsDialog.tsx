import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Button } from '@/components/ui/button';
import { Field, FieldError, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useId, useState } from 'react';

import { apiRequest } from '@/api/client';
import type {
  Enrollment,
  StudentLevelUpdate,
  StudentRequirements,
} from '@/api/generated/models';
import { demoMode, usePracticeSettings } from '@/api/queries';
import { LabelHint } from '@/components/Common';
import { FormDialog } from '@/components/Dialogs';
import { studentLevels } from '@/studentLevels';
import type { Page, Person } from '@/types';
import { defaultPracticeGoals, defaultWeeklyTargets } from '@/utils';

type Requirement =
  'easyMinutes' | 'mediumMinutes' | 'hardMinutes' | 'weeklyHours';
type Drafts = Record<Requirement, string>;

const goalFields: { key: Requirement; label: string }[] = [
  { key: 'easyMinutes', label: 'Easy' },
  { key: 'mediumMinutes', label: 'Medium' },
  { key: 'hardMinutes', label: 'Hard' },
];

// Demo requirements live for the session, as real ones do on the server.
const demoRequirements = new Map<string, StudentRequirements>();

function draftsFrom(requirements?: StudentRequirements): Drafts {
  const text = (value?: number | null) => (value ? String(value) : '');
  return {
    easyMinutes: text(requirements?.easyMinutes),
    mediumMinutes: text(requirements?.mediumMinutes),
    hardMinutes: text(requirements?.hardMinutes),
    weeklyHours: text(requirements?.weeklyHours),
  };
}

/** Blank follows the program default; otherwise a whole number in range. */
function invalid(value: string, max: number, min = 1) {
  if (!value) return false;
  return !/^\d+$/.test(value) || Number(value) < min || Number(value) > max;
}

/**
 * A student's details for their mentor or a coordinator: their level, their
 * own LeetCode time goals and their weekly hours. A blank goal or hours
 * follows the program default set in Settings. Opened from a row menu.
 */
export function StudentDetailsDialog({
  seasonId,
  person,
  onChanged,
  open,
  onOpenChange,
}: {
  seasonId: string;
  person: Person;
  onChanged: () => void;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const levelId = useId();
  const levelItems = Object.entries(studentLevels).map(([value, label]) => ({
    label,
    value,
  }));
  const program = usePracticeSettings().data;
  const defaults: Record<Requirement, number> = {
    easyMinutes: program?.easyMinutes ?? defaultPracticeGoals.Easy,
    mediumMinutes: program?.mediumMinutes ?? defaultPracticeGoals.Medium,
    hardMinutes: program?.hardMinutes ?? defaultPracticeGoals.Hard,
    weeklyHours: program?.weeklyHours ?? defaultWeeklyTargets.hours,
  };
  const enrollmentId = person.enrollmentId ?? '';
  const requirements = useQuery({
    queryKey: ['student-requirements', seasonId, enrollmentId],
    enabled: open && Boolean(enrollmentId),
    queryFn: async (): Promise<StudentRequirements> =>
      demoMode
        ? (demoRequirements.get(enrollmentId) ?? {})
        : apiRequest<StudentRequirements>(
            `/seasons/${encodeURIComponent(seasonId)}/members/${encodeURIComponent(enrollmentId)}/requirements`,
          ),
  });
  const initialLevel =
    person.studentLevel && person.studentLevel !== 'not_applicable'
      ? person.studentLevel
      : 'novice';
  const [level, setLevel] = useState<keyof typeof studentLevels>(initialLevel);
  const [drafts, setDrafts] = useState<Drafts>(draftsFrom());
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  // The saved requirements fill the fields once they load.
  useEffect(() => {
    if (requirements.data) setDrafts(draftsFrom(requirements.data));
  }, [requirements.data]);

  const saved = draftsFrom(requirements.data);
  const requirementsChanged = (Object.keys(drafts) as Requirement[]).some(
    (key) => drafts[key] !== saved[key],
  );
  const levelChanged = level !== person.studentLevel;
  const anyInvalid =
    goalFields.some(({ key }) => invalid(drafts[key], 180)) ||
    invalid(drafts.weeklyHours, 168);

  const save = async () => {
    if (!enrollmentId) return;
    setPending(true);
    setError('');
    try {
      const memberPath = `/seasons/${encodeURIComponent(seasonId)}/members/${encodeURIComponent(enrollmentId)}`;
      if (levelChanged) {
        if (!demoMode) {
          const body: StudentLevelUpdate = { studentLevel: level };
          await apiRequest<Enrollment>(`${memberPath}/student-level`, {
            method: 'PATCH',
            body: JSON.stringify(body),
          });
        }
        queryClient.setQueryData<Page<Person>>(
          ['season-people', seasonId],
          (current) =>
            current
              ? {
                  ...current,
                  items: current.items.map((item) =>
                    item.enrollmentId === enrollmentId
                      ? { ...item, studentLevel: level }
                      : item,
                  ),
                }
              : current,
        );
      }
      if (requirementsChanged) {
        const value = (key: Requirement) =>
          drafts[key] === '' ? null : Number(drafts[key]);
        const body: StudentRequirements = {
          easyMinutes: value('easyMinutes'),
          mediumMinutes: value('mediumMinutes'),
          hardMinutes: value('hardMinutes'),
          weeklyHours: value('weeklyHours'),
        };
        if (demoMode) demoRequirements.set(enrollmentId, body);
        else
          await apiRequest<StudentRequirements>(`${memberPath}/requirements`, {
            method: 'PUT',
            body: JSON.stringify(body),
          });
        queryClient.setQueryData(
          ['student-requirements', seasonId, enrollmentId],
          body,
        );
      }
      if (!demoMode) {
        onChanged();
        void queryClient.invalidateQueries({
          queryKey: ['season-enrollments', seasonId],
        });
      }
      onOpenChange(false);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : "Unable to save the student's details.",
      );
    } finally {
      setPending(false);
    }
  };

  const numberField = (
    key: Requirement,
    label: string,
    max: number,
    min = 1,
  ) => {
    const id = `${levelId}-${key}`;
    return (
      <Field key={key}>
        <FieldLabel htmlFor={id}>{label}</FieldLabel>
        <Input
          id={id}
          type="number"
          inputMode="numeric"
          min={min}
          max={max}
          placeholder={String(defaults[key])}
          value={drafts[key]}
          disabled={pending || requirements.isLoading}
          aria-invalid={invalid(drafts[key], max, min)}
          onChange={(event) =>
            setDrafts((current) => ({ ...current, [key]: event.target.value }))
          }
        />
        {invalid(drafts[key], max, min) ? (
          <FieldError>
            Enter {min} to {max}, or leave blank.
          </FieldError>
        ) : null}
      </Field>
    );
  };

  return (
    <FormDialog
      title={`Edit Details · ${person.name}`}
      description="Blank goals and hours follow the program defaults in Settings."
      open={open}
      onOpenChange={(next) => {
        if (pending) return;
        onOpenChange(next);
        if (next) {
          setLevel(initialLevel);
          setDrafts(draftsFrom(requirements.data));
          setError('');
        }
      }}
    >
      <div className="grid gap-4">
        {error ? (
          <p className="text-sm text-destructive" role="alert">
            {error}
          </p>
        ) : null}
        <Field>
          <FieldLabel htmlFor={levelId}>Student Level</FieldLabel>
          <Select
            items={levelItems}
            value={level}
            disabled={pending}
            onValueChange={(value) => {
              if (value) setLevel(value as keyof typeof studentLevels);
            }}
          >
            <SelectTrigger id={levelId} className="w-full">
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
        <fieldset className="grid gap-2">
          <legend className="mb-2 text-sm font-medium">
            Practice goals <LabelHint>minutes</LabelHint>
          </legend>
          <div className="grid grid-cols-3 gap-3 max-[30rem]:grid-cols-1">
            {goalFields.map(({ key, label }) => numberField(key, label, 180))}
          </div>
        </fieldset>
        {numberField('weeklyHours', 'Weekly hours', 168)}
        <div className="flex flex-wrap justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={() => onOpenChange(false)}
          >
            Cancel
          </Button>
          <Button
            type="button"
            disabled={
              pending ||
              !enrollmentId ||
              anyInvalid ||
              (!levelChanged && !requirementsChanged)
            }
            onClick={() => void save()}
          >
            {pending ? 'Saving…' : 'Save Details'}
          </Button>
        </div>
      </div>
    </FormDialog>
  );
}
