import { useId, useState, type FormEvent } from 'react';
import {
  IconArrowDown,
  IconArrowUp,
  IconPlus,
  IconTrash,
} from '@tabler/icons-react';
import type {
  JobApplication,
  JobApplicationStatus,
  JobStageEditInput,
} from '@/api/generated/models';
import { useJobActions } from '@/api/jobs';
import { useJobsUser } from '@/components/JobsShell';
import { FormDialog, NamedConfirmation } from '@/components/Dialogs';
import { JobSelect } from '@/components/JobForms';
import { AppDatePicker } from '@/components/AppDatePicker';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Field, FieldLabel, FieldError } from '@/components/ui/field';
import { jobApplicationStatuses, localJobDate } from '@/jobs';

type DraftStage = JobStageEditInput & { key: string; initial: boolean };

export function JobApplicationForm({
  application,
  onClose,
}: {
  application: JobApplication;
  onClose: () => void;
}) {
  const user = useJobsUser();
  const save = useJobActions(user?.id);
  const prefix = useId();
  const [status, setStatus] = useState(application.status);
  const [stages, setStages] = useState<DraftStage[]>(() =>
    application.stages.map((s) => ({ ...s, key: s.id })),
  );
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const [remove, setRemove] = useState(false);
  function update(key: string, changes: Partial<DraftStage>) {
    setStages((previous) =>
      previous.map((stage) =>
        stage.key === key ? { ...stage, ...changes } : stage,
      ),
    );
  }
  function move(index: number, delta: number) {
    setStages((previous) => {
      const next = [...previous];
      const target = index + delta;
      if (index < 1 || target < 1 || target >= next.length) return previous;
      [next[index], next[target]] = [next[target], next[index]];
      return next;
    });
  }
  async function submit(event: FormEvent) {
    event.preventDefault();
    setPending(true);
    setError('');
    try {
      await save(`/me/job-applications/${application.id}`, 'PUT', {
        status,
        date: localJobDate(),
        stages: stages.map(({ id, title, date, notes }) => ({
          ...(id ? { id } : {}),
          title,
          date,
          notes,
        })),
      });
      onClose();
    } catch (e) {
      setError(
        e instanceof Error ? e.message : 'Unable to save the application.',
      );
    } finally {
      setPending(false);
    }
  }
  return (
    <FormDialog
      title="Edit Application"
      open
      onOpenChange={(open) => {
        if (!open && !pending) onClose();
      }}
      contentClassName="max-h-[90svh] overflow-y-auto sm:max-w-2xl"
    >
      <form className="grid gap-6" onSubmit={(event) => void submit(event)}>
        <p className="text-sm text-muted-foreground">
          {application.position.title} at {application.position.company.name}
        </p>
        <fieldset disabled={pending} className="grid min-w-0 gap-6">
          <JobSelect
            label="Status"
            value={status}
            options={jobApplicationStatuses}
            onChange={(value) => setStatus(value as JobApplicationStatus)}
          />
          <div className="grid gap-4">
            {stages.map((stage, index) => (
              <fieldset
                key={stage.key}
                className="grid min-w-0 gap-4 border-t border-border pt-4"
              >
                <legend className="px-1 text-sm font-semibold">
                  {stage.initial ? 'Applied' : `Stage ${index}`}
                </legend>
                {!stage.initial && (
                  <div className="flex justify-end gap-2">
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      aria-label={`Move Stage ${index} Up`}
                      disabled={index <= 1}
                      onClick={() => move(index, -1)}
                    >
                      <IconArrowUp size={17} aria-hidden="true" />
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      aria-label={`Move Stage ${index} Down`}
                      disabled={index === stages.length - 1}
                      onClick={() => move(index, 1)}
                    >
                      <IconArrowDown size={17} aria-hidden="true" />
                    </Button>
                    <Button
                      type="button"
                      variant="destructive"
                      size="icon"
                      aria-label={`Remove Stage ${index}`}
                      onClick={() =>
                        setStages((previous) =>
                          previous.filter((s) => s.key !== stage.key),
                        )
                      }
                    >
                      <IconTrash size={17} aria-hidden="true" />
                    </Button>
                  </div>
                )}
                <Field>
                  <FieldLabel htmlFor={`${prefix}-${stage.key}-title`}>
                    Stage Name
                  </FieldLabel>
                  <Input
                    id={`${prefix}-${stage.key}-title`}
                    required
                    maxLength={120}
                    readOnly={stage.initial}
                    value={stage.title}
                    onChange={(event) =>
                      update(stage.key, { title: event.target.value })
                    }
                  />
                </Field>
                <AppDatePicker
                  label="Date"
                  value={stage.date}
                  onChange={(date) => update(stage.key, { date })}
                />
                <Field>
                  <FieldLabel htmlFor={`${prefix}-${stage.key}-notes`}>
                    Notes (Private, Optional)
                  </FieldLabel>
                  <Textarea
                    id={`${prefix}-${stage.key}-notes`}
                    rows={3}
                    maxLength={10000}
                    value={stage.notes}
                    onChange={(event) =>
                      update(stage.key, { notes: event.target.value })
                    }
                  />
                </Field>
              </fieldset>
            ))}
            <Button
              type="button"
              variant="outline"
              disabled={stages.length >= 200}
              onClick={() =>
                setStages((previous) => [
                  ...previous,
                  {
                    key: crypto.randomUUID(),
                    title: '',
                    date: localJobDate(),
                    notes: '',
                    initial: false,
                  },
                ])
              }
            >
              <IconPlus size={17} aria-hidden="true" />
              Add Stage
            </Button>
          </div>
        </fieldset>
        {error && <FieldError>{error}</FieldError>}
        <div className="flex flex-wrap justify-end gap-2">
          <Button
            type="button"
            variant="destructive"
            className="mr-auto"
            disabled={pending}
            onClick={() => setRemove(true)}
          >
            <IconTrash size={17} aria-hidden="true" />
            Remove Application
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={onClose}
          >
            Cancel
          </Button>
          <Button type="submit" disabled={pending}>
            {pending ? 'Saving…' : 'Save Changes'}
          </Button>
        </div>
      </form>
      {remove && (
        <NamedConfirmation
          open
          name={application.position.title}
          actionLabel="Remove Application"
          description="This removes your application and all its private stage notes. The shared position stays on the board."
          onOpenChange={setRemove}
          onConfirm={async () => {
            await save(`/me/job-applications/${application.id}`, 'DELETE');
            onClose();
          }}
        />
      )}
    </FormDialog>
  );
}
