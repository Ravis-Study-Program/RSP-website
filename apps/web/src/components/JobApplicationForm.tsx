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
import { useJobActions, useJobPositionProgress } from '@/api/jobs';
import { useJobsUser } from '@/components/JobsShell';
import { FormDialog, NamedConfirmation } from '@/components/Dialogs';
import { JobSelect } from '@/components/JobForms';
import { AppDatePicker } from '@/components/AppDatePicker';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Textarea } from '@/components/ui/textarea';
import { Field, FieldLabel, FieldError } from '@/components/ui/field';
import { jobApplicationStatuses, localJobDate } from '@/jobs';
import { compareJobStageDates, isMergedAppliedNote } from '@/jobStages';
import { JobStageNameField } from '@/components/JobStageFields';

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
  const progress = useJobPositionProgress(application.position.id);
  const prefix = useId();
  const [status, setStatus] = useState(application.status);
  const [shareProgress, setShareProgress] = useState(application.shareProgress);
  const [stages, setStages] = useState<DraftStage[]>(() =>
    application.stages
      .filter((stage) => !isMergedAppliedNote(stage) || stage.notes.trim())
      .map((stage) => ({ ...stage, key: stage.id })),
  );
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const [remove, setRemove] = useState(false);
  const visibleStages = stages.filter((stage) => !isMergedAppliedNote(stage));
  const appliedNotes = stages
    .filter((stage) => stage.initial || isMergedAppliedNote(stage))
    .sort(compareJobStageDates);

  function update(key: string, changes: Partial<DraftStage>) {
    setStages((previous) =>
      previous.map((stage) =>
        stage.key === key ? { ...stage, ...changes } : stage,
      ),
    );
  }
  function move(index: number, delta: number) {
    setStages((previous) => {
      const indexes = previous.flatMap((stage, i) =>
        isMergedAppliedNote(stage) ? [] : [i],
      );
      const target = index + delta;
      if (index < 1 || target < 1 || target >= indexes.length) return previous;
      const next = [...previous];
      const from = indexes[index];
      const to = indexes[target];
      [next[from], next[to]] = [next[to], next[from]];
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
        shareProgress,
        date: localJobDate(),
        stages: stages
          .filter((stage) => !isMergedAppliedNote(stage) || stage.notes.trim())
          .map(({ id, title, date, notes }) => ({
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
      contentClassName="flex max-h-[90svh] flex-col overflow-hidden pb-6 sm:max-w-2xl"
    >
      <form
        className="flex min-h-0 flex-col"
        onSubmit={(event) => void submit(event)}
      >
        <div className="-mx-4 grid min-h-0 gap-6 overflow-y-auto overscroll-contain px-4 pb-4">
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
            <div className="grid gap-2">
              <Field orientation="horizontal" className="items-center gap-2">
                <Checkbox
                  id={`${prefix}-share`}
                  checked={shareProgress}
                  onCheckedChange={setShareProgress}
                  aria-describedby={`${prefix}-share-hint`}
                />
                <FieldLabel htmlFor={`${prefix}-share`}>
                  Share Progress
                </FieldLabel>
              </Field>
              <p
                id={`${prefix}-share-hint`}
                className="text-sm text-muted-foreground"
              >
                Anyone browsing this position can see shared stage names and
                counts. Your name, dates and notes stay private.
              </p>
            </div>
            <section
              className="grid gap-3"
              aria-labelledby={`${prefix}-stages`}
            >
              <h3 id={`${prefix}-stages`} className="text-sm font-semibold">
                Stages
              </h3>
              <div className="divide-y divide-border rounded-lg border border-border">
                {visibleStages.map((stage, index) =>
                  stage.initial ? (
                    <fieldset
                      key={stage.key}
                      aria-label="Applied"
                      className="grid min-w-0 gap-3 p-3 sm:px-4"
                    >
                      <div className="max-w-42">
                        <AppDatePicker
                          label="Applied Date"
                          value={stage.date}
                          onChange={(date) => update(stage.key, { date })}
                        />
                      </div>
                      <Field>
                        <FieldLabel
                          htmlFor={`${prefix}-${appliedNotes[0].key}-notes`}
                        >
                          Notes
                        </FieldLabel>
                        {appliedNotes.map((note, noteIndex) => (
                          <Textarea
                            key={note.key}
                            id={`${prefix}-${note.key}-notes`}
                            aria-label={
                              noteIndex ? `Notes ${noteIndex + 1}` : undefined
                            }
                            rows={2}
                            maxLength={10000}
                            value={note.notes}
                            onChange={(event) =>
                              update(note.key, { notes: event.target.value })
                            }
                          />
                        ))}
                      </Field>
                    </fieldset>
                  ) : (
                    <fieldset
                      key={stage.key}
                      aria-labelledby={`${prefix}-${stage.key}-heading`}
                      className="grid min-w-0 gap-3 p-3 sm:px-4"
                    >
                      <div className="flex items-center justify-between gap-2">
                        <h4
                          id={`${prefix}-${stage.key}-heading`}
                          className="text-sm font-semibold"
                        >
                          Stage {index}
                        </h4>
                        <div className="flex gap-1">
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
                            disabled={index === visibleStages.length - 1}
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
                      </div>
                      <div className="grid min-w-0 items-end gap-3 sm:grid-cols-[minmax(0,1fr)_10.5rem]">
                        <JobStageNameField
                          value={stage.title}
                          onChange={(title) => update(stage.key, { title })}
                          reported={progress.data?.suggestions ?? []}
                        />
                        <AppDatePicker
                          label="Date"
                          value={stage.date}
                          onChange={(date) => update(stage.key, { date })}
                        />
                      </div>
                      <Field>
                        <FieldLabel htmlFor={`${prefix}-${stage.key}-notes`}>
                          Notes
                        </FieldLabel>
                        <Textarea
                          id={`${prefix}-${stage.key}-notes`}
                          rows={2}
                          maxLength={10000}
                          value={stage.notes}
                          onChange={(event) =>
                            update(stage.key, { notes: event.target.value })
                          }
                        />
                      </Field>
                    </fieldset>
                  ),
                )}
              </div>
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
            </section>
          </fieldset>
        </div>
        <div className="flex shrink-0 flex-wrap justify-end gap-2 border-t pt-4">
          {error && <FieldError className="basis-full">{error}</FieldError>}
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
          <div className="flex gap-2">
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
        </div>
      </form>
      {remove && (
        <NamedConfirmation
          open
          name={application.position.title}
          title={`Remove application for ${application.position.company.name} (${application.position.title})?`}
          actionLabel="Remove Application"
          confirmLabel="Confirm"
          requireTyping={false}
          description="This removes your application and all its private stage notes."
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
