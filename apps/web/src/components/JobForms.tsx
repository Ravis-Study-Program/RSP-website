import { useId, useState, type FormEvent, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import {
  JobRequestError,
  useJobActions,
  useJobCompanies,
  useJobPositionProgress,
} from '@/api/jobs';
import type {
  JobApplication,
  JobCompany,
  JobPosition,
  JobPositionInput,
  JobStage,
} from '@/api/generated/models';
import { useJobsUser } from '@/components/JobsShell';
import { JobCompanyField } from '@/components/JobCompanyField';
import { JobStageNameField } from '@/components/JobStageFields';
import { AppDatePicker } from '@/components/AppDatePicker';
import { FormDialog } from '@/components/Dialogs';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Field, FieldLabel, FieldError } from '@/components/ui/field';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { jobCategories, jobTypes, localJobDate, positionInput } from '@/jobs';

export function JobSelect({
  label,
  value,
  options,
  onChange,
  disabled,
  compact = false,
}: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
  disabled?: boolean;
  compact?: boolean;
}) {
  const id = useId();
  return (
    <Field className={compact ? 'w-auto min-w-40' : undefined}>
      {!compact && <FieldLabel htmlFor={id}>{label}</FieldLabel>}
      <Select
        items={options}
        value={value}
        onValueChange={(next) => {
          if (next !== null) onChange(next);
        }}
        disabled={disabled}
      >
        <SelectTrigger
          id={id}
          aria-label={compact ? label : undefined}
          className="w-full"
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((v) => (
            <SelectItem key={v.value} value={v.value}>
              {v.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </Field>
  );
}

function FormActions({
  pending,
  label,
  onCancel,
}: {
  pending: boolean;
  label: string;
  onCancel: () => void;
}) {
  return (
    <div className="flex justify-end gap-2 pt-2">
      <Button
        type="button"
        variant="outline"
        disabled={pending}
        onClick={onCancel}
      >
        Cancel
      </Button>
      <Button type="submit" disabled={pending}>
        {pending ? 'Saving…' : label}
      </Button>
    </div>
  );
}

export function JobForm({
  children,
  onSave,
  onClose,
  label = 'Save Changes',
}: {
  children: ReactNode;
  onSave: () => Promise<unknown>;
  onClose: () => void;
  label?: string;
}) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const [duplicate, setDuplicate] = useState<string>();
  async function submit(event: FormEvent) {
    event.preventDefault();
    setPending(true);
    setError('');
    setDuplicate(undefined);
    try {
      await onSave();
      onClose();
    } catch (error) {
      setError(
        error instanceof Error ? error.message : 'Unable to save. Try again.',
      );
      if (error instanceof JobRequestError)
        setDuplicate(error.existingPositionId);
    } finally {
      setPending(false);
    }
  }
  return (
    <form className="grid gap-4" onSubmit={(event) => void submit(event)}>
      {children}
      {error ? <FieldError>{error}</FieldError> : null}
      {duplicate ? (
        <Link
          to={`/jobs?position=${encodeURIComponent(duplicate)}`}
          onClick={onClose}
        >
          View Existing Position
        </Link>
      ) : null}
      <FormActions pending={pending} label={label} onCancel={onClose} />
    </form>
  );
}

export function PositionForm({
  position,
  onClose,
}: {
  position?: JobPosition;
  onClose: () => void;
}) {
  const user = useJobsUser();
  const save = useJobActions(user?.id);
  const companies = useJobCompanies();
  const prefix = useId();
  const [input, setInput] = useState<JobPositionInput>(() =>
    position
      ? positionInput(position)
      : {
          companyName: '',
          category: 'other',
          title: '',
          jobType: 'intern',
          eligibility: '',
          applicationUrl: '',
          locations: [],
          notes: '',
          status: 'open',
        },
  );
  const [locations, setLocations] = useState(
    position?.locations.join('\n') ?? '',
  );
  const existing = companies.data?.find(
    (c) => c.name.toLowerCase() === input.companyName.trim().toLowerCase(),
  );
  const set = <K extends keyof JobPositionInput>(
    key: K,
    value: JobPositionInput[K],
  ) => setInput((v) => ({ ...v, [key]: value }));
  return (
    <FormDialog
      title={position ? 'Edit Position' : 'Add Position'}
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      contentClassName="max-h-[90svh] overflow-y-auto sm:max-w-2xl"
    >
      <JobForm
        onClose={onClose}
        label={position ? 'Save Changes' : 'Publish Position'}
        onSave={() =>
          save(
            `/jobs/positions${position ? `/${position.id}` : ''}`,
            position ? 'PATCH' : 'POST',
            {
              ...input,
              category: existing?.category ?? input.category,
              locations: locations
                .split('\n')
                .map((v) => v.trim())
                .filter(Boolean),
            },
          )
        }
      >
        <div className="grid gap-4 sm:grid-cols-2">
          <JobCompanyField
            value={input.companyName}
            onChange={(name) => set('companyName', name)}
          />
          <JobSelect
            label="Company Category"
            value={existing?.category ?? input.category}
            options={jobCategories}
            disabled={Boolean(existing)}
            onChange={(v) => set('category', v as JobPositionInput['category'])}
          />
        </div>
        <Field>
          <FieldLabel htmlFor={`${prefix}-title`}>Job Title</FieldLabel>
          <Input
            id={`${prefix}-title`}
            required
            maxLength={300}
            value={input.title}
            onChange={(e) => set('title', e.target.value)}
          />
        </Field>
        <JobSelect
          label="Job Type"
          value={input.jobType}
          options={jobTypes}
          onChange={(v) => set('jobType', v as JobPositionInput['jobType'])}
        />
        <Field>
          <FieldLabel htmlFor={`${prefix}-url`}>Application Link</FieldLabel>
          <Input
            id={`${prefix}-url`}
            type="url"
            required
            maxLength={2048}
            placeholder="https://"
            value={input.applicationUrl}
            onChange={(e) => set('applicationUrl', e.target.value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor={`${prefix}-locations`}>
            Locations (One per Line)
          </FieldLabel>
          <Textarea
            id={`${prefix}-locations`}
            required
            rows={2}
            value={locations}
            onChange={(e) => setLocations(e.target.value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor={`${prefix}-eligibility`}>
            Eligibility Requirements (Optional)
          </FieldLabel>
          <Textarea
            id={`${prefix}-eligibility`}
            maxLength={4000}
            rows={2}
            value={input.eligibility}
            onChange={(e) => set('eligibility', e.target.value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor={`${prefix}-notes`}>
            Details (Public, Optional)
          </FieldLabel>
          <Textarea
            id={`${prefix}-notes`}
            maxLength={10000}
            rows={3}
            value={input.notes}
            onChange={(e) => set('notes', e.target.value)}
          />
        </Field>
      </JobForm>
    </FormDialog>
  );
}

export function CompanyForm({
  company,
  onClose,
}: {
  company: JobCompany;
  onClose: () => void;
}) {
  const user = useJobsUser();
  const save = useJobActions(user?.id);
  const id = useId();
  const [name, setName] = useState(company.name);
  const [category, setCategory] = useState(company.category);
  return (
    <FormDialog
      title="Edit Company"
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <JobForm
        onClose={onClose}
        onSave={() =>
          save(`/jobs/companies/${company.id}`, 'PATCH', { name, category })
        }
      >
        <Field>
          <FieldLabel htmlFor={id}>Company Name</FieldLabel>
          <Input
            id={id}
            required
            maxLength={120}
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
        </Field>
        <JobSelect
          label="Category"
          value={category}
          options={jobCategories}
          onChange={(v) => setCategory(v as JobCompany['category'])}
        />
      </JobForm>
    </FormDialog>
  );
}

export function StageForm({
  application,
  stage,
  onClose,
}: {
  application: JobApplication;
  stage?: JobStage;
  onClose: () => void;
}) {
  const user = useJobsUser();
  const save = useJobActions(user?.id);
  const progress = useJobPositionProgress(
    application.position.id,
    !stage?.initial,
  );
  const id = useId();
  const [title, setTitle] = useState(stage?.title ?? '');
  const [date, setDate] = useState(stage?.date ?? localJobDate());
  const [notes, setNotes] = useState(stage?.notes ?? '');
  return (
    <FormDialog
      title={
        stage?.initial
          ? 'Edit Applied Date'
          : stage
            ? 'Edit Stage'
            : 'Add Stage'
      }
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <JobForm
        onClose={onClose}
        label={stage ? 'Save Changes' : 'Add Stage'}
        onSave={() =>
          save(
            `/me/job-applications/${application.id}/stages${stage ? `/${stage.id}` : ''}`,
            stage ? 'PATCH' : 'POST',
            { title, date, notes },
          )
        }
      >
        {!stage?.initial && (
          <>
            <JobStageNameField
              value={title}
              onChange={setTitle}
              reported={progress.data?.suggestions ?? []}
            />
            {application.shareProgress && (
              <p className="text-sm text-muted-foreground">
                This stage name is public while sharing is on.
              </p>
            )}
          </>
        )}
        <AppDatePicker
          label={stage?.initial ? 'Applied Date' : 'Date'}
          value={date}
          onChange={setDate}
        />
        {!stage?.initial && (
          <Field>
            <FieldLabel htmlFor={`${id}-notes`}>
              Notes (Private, Optional)
            </FieldLabel>
            <Textarea
              id={`${id}-notes`}
              maxLength={10000}
              rows={5}
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
            />
          </Field>
        )}
      </JobForm>
    </FormDialog>
  );
}

export function MergePositionForm({
  source,
  positions,
  onClose,
}: {
  source: JobPosition;
  positions: JobPosition[];
  onClose: () => void;
}) {
  const user = useJobsUser();
  const save = useJobActions(user?.id);
  const [targetId, setTargetId] = useState('');
  const targets = positions.filter((p) => p.id !== source.id);
  return (
    <FormDialog
      title="Merge Duplicate Position"
      description={`Merge “${source.title}” at ${source.company.name} into another job listing.`}
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <JobForm
        onClose={onClose}
        label="Merge Positions"
        onSave={() => {
          if (!targetId) throw new Error('Choose the position to keep.');
          return save(`/jobs/positions/${source.id}/merge`, 'POST', {
            targetId,
          });
        }}
      >
        <div className="grid gap-2 text-sm">
          <h3 className="font-semibold">In the Combined Listing</h3>
          <ul className="list-disc space-y-2 pl-5">
            <li>
              Company name, job title, job type, eligibility requirements,
              application link, details, and open or closed status use the
              selected position’s data.
            </li>
            <li>
              Locations and each person’s private application histories are
              combined. Private notes remain private.
            </li>
          </ul>
        </div>
        <JobSelect
          label="Position to Keep"
          value={targetId}
          options={[
            { value: '', label: 'Choose a Position' },
            ...targets.map((p) => ({
              value: p.id,
              label: `${p.company.name} · ${p.title} · ${p.locations.join(' / ')}`,
            })),
          ]}
          onChange={setTargetId}
        />
      </JobForm>
    </FormDialog>
  );
}
