import { Button } from '@/components/ui/button';
import { Field, FieldError, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { IconClockEdit } from '@tabler/icons-react';
import { useId, useState, type ReactNode } from 'react';

import { FormDialog } from '@/components/Dialogs';
import {
  type GoalMinutes,
  goalDifficulties,
  goalMinutesRange,
  validGoalMinutes,
} from '@/practiceGoals';

type GoalDrafts = Record<keyof GoalMinutes, string>;

const draftsFrom = (goals: GoalMinutes): GoalDrafts => ({
  Easy: String(goals.Easy),
  Medium: String(goals.Medium),
  Hard: String(goals.Hard),
});

/**
 * Edits Easy, Medium and Hard time goals. `onReset`, when given, offers a
 * second action such as returning a student to the programme goals.
 */
export function PracticeGoalsDialog({
  title,
  description,
  triggerLabel,
  goals,
  saveLabel,
  onSave,
  reset,
}: {
  title: string;
  description: string;
  triggerLabel: ReactNode;
  /** Current goals; the trigger is disabled until they load. */
  goals: GoalMinutes | undefined;
  saveLabel: string;
  onSave: (goals: GoalMinutes) => Promise<void>;
  reset?: { label: string; onReset: () => Promise<void> };
}) {
  const fieldId = useId();
  const [open, setOpen] = useState(false);
  const [drafts, setDrafts] = useState<GoalDrafts>(() =>
    draftsFrom(goals ?? { Easy: 0, Medium: 0, Hard: 0 }),
  );
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const parsed = Object.fromEntries(
    goalDifficulties.map((difficulty) => [
      difficulty,
      Number(drafts[difficulty]),
    ]),
  ) as GoalMinutes;
  const invalid = goalDifficulties.filter(
    (difficulty) =>
      drafts[difficulty].trim() === '' || !validGoalMinutes(parsed[difficulty]),
  );
  const unchanged =
    goals !== undefined &&
    goalDifficulties.every(
      (difficulty) => parsed[difficulty] === goals[difficulty],
    );
  const run = async (action: () => Promise<void>) => {
    setPending(true);
    setError('');
    try {
      await action();
      setOpen(false);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to save the time goals.',
      );
    } finally {
      setPending(false);
    }
  };
  return (
    <FormDialog
      triggerVariant="outline"
      triggerSize="sm"
      title={title}
      description={description}
      trigger={
        <>
          <IconClockEdit size={17} aria-hidden="true" /> {triggerLabel}
        </>
      }
      open={open}
      onOpenChange={(next) => {
        if (pending || (next && !goals)) return;
        setOpen(next);
        if (next && goals) {
          setDrafts(draftsFrom(goals));
          setError('');
        }
      }}
    >
      {error ? (
        <p className="text-sm text-destructive" role="alert">
          {error}
        </p>
      ) : null}
      <form
        className="grid gap-4"
        onSubmit={(event) => {
          event.preventDefault();
          if (!invalid.length) void run(() => onSave(parsed));
        }}
      >
        <div className="grid grid-cols-3 gap-3 max-[30rem]:grid-cols-1">
          {goalDifficulties.map((difficulty) => {
            const id = `${fieldId}-${difficulty}`;
            const isInvalid = invalid.includes(difficulty);
            return (
              <Field key={difficulty}>
                <FieldLabel htmlFor={id}>{difficulty} (minutes)</FieldLabel>
                <Input
                  id={id}
                  type="number"
                  inputMode="numeric"
                  min={goalMinutesRange.min}
                  max={goalMinutesRange.max}
                  step={1}
                  required
                  disabled={pending}
                  value={drafts[difficulty]}
                  aria-invalid={isInvalid}
                  aria-describedby={isInvalid ? `${id}-error` : undefined}
                  onChange={(event) =>
                    setDrafts((current) => ({
                      ...current,
                      [difficulty]: event.target.value,
                    }))
                  }
                />
                {isInvalid ? (
                  <FieldError id={`${id}-error`}>
                    Enter {goalMinutesRange.min}–{goalMinutesRange.max} minutes.
                  </FieldError>
                ) : null}
              </Field>
            );
          })}
        </div>
        <div className="flex flex-wrap justify-end gap-2">
          {reset ? (
            <Button
              type="button"
              variant="outline"
              disabled={pending}
              onClick={() => void run(reset.onReset)}
            >
              {reset.label}
            </Button>
          ) : null}
          <Button
            type="submit"
            disabled={pending || invalid.length > 0 || unchanged}
          >
            {pending ? 'Saving…' : saveLabel}
          </Button>
        </div>
      </form>
    </FormDialog>
  );
}
