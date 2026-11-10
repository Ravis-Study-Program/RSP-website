import { useId, useState, type FormEvent, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { JobRequestError, useJobActions, useJobCompanies } from '@/api/jobs';
import type {
  JobCompany,
  JobPosition,
  JobPositionInput,
  JobStage,
  JobReferral,
} from '@/api/generated/models';
import { useJobsUser } from '@/components/JobsShell';
import { ReferralNotesHelp } from '@/components/ReferralNotesHelp';
import { JobCompanyField } from '@/components/JobCompanyField';
import { FormDialog } from '@/components/Dialogs';
import { AppDatePicker } from '@/components/AppDatePicker';
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

function JobForm({
  children,
  onSave,
  onClose,
  label = 'Save changes',
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
          View existing position
        </Link>
      ) : null}
      <FormActions pending={pending} label={label} onCancel={onClose} />
    </form>
  );
}

export function ReferralForm({
  referral,
  onClose,
}: {
  referral?: JobReferral;
  onClose: () => void;
}) {
  const user = useJobsUser();
  const save = useJobActions(user?.id);
  const id = useId();
  const [companyName, setCompanyName] = useState(referral?.company.name ?? '');
  const [notes, setNotes] = useState(referral?.notes ?? '');
  return (
    <FormDialog
      title={referral ? 'Edit referral listing' : 'Open to referring'}
      description="Your name, company and notes will be visible to signed-in members."
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <JobForm
        onClose={onClose}
        label={referral ? 'Save changes' : 'Open to referring'}
        onSave={() =>
          save(
            `/jobs/referrals${referral ? `/${referral.id}` : ''}`,
            referral ? 'PATCH' : 'POST',
            { companyName, notes },
          )
        }
      >
        <JobCompanyField value={companyName} onChange={setCompanyName} />
        <Field>
          <div className="flex items-center gap-1">
            <FieldLabel htmlFor={`${id}-notes`}>Notes</FieldLabel>
            <ReferralNotesHelp />
          </div>
          <Textarea
            id={`${id}-notes`}
            maxLength={10000}
            rows={5}
            value={notes}
            onChange={(event) => setNotes(event.target.value)}
          />
        </Field>
      </JobForm>
    </FormDialog>
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
      title={position ? 'Edit position' : 'Add position'}
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      contentClassName="max-h-[90svh] overflow-y-auto sm:max-w-2xl"
    >
      <JobForm
        onClose={onClose}
        label={position ? 'Save changes' : 'Publish position'}
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
            label="Company category"
            value={existing?.category ?? input.category}
            options={jobCategories}
            disabled={Boolean(existing)}
            onChange={(v) => set('category', v as JobPositionInput['category'])}
          />
        </div>
        <Field>
          <FieldLabel htmlFor={`${prefix}-title`}>Job title</FieldLabel>
          <Input
            id={`${prefix}-title`}
            required
            maxLength={300}
            value={input.title}
            onChange={(e) => set('title', e.target.value)}
          />
        </Field>
        <JobSelect
          label="Job type"
          value={input.jobType}
          options={jobTypes}
          onChange={(v) => set('jobType', v as JobPositionInput['jobType'])}
        />
        <Field>
          <FieldLabel htmlFor={`${prefix}-url`}>Application link</FieldLabel>
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
            Locations · one per line
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
            Eligibility requirements · optional
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
            Notes · public, optional
          </FieldLabel>
          <Textarea
            id={`${prefix}-notes`}
            maxLength={10000}
            rows={3}
            value={input.notes}
            onChange={(e) => set('notes', e.target.value)}
          />
        </Field>
        {!position && (
          <p className="text-sm text-muted-foreground">
            Published immediately. Staff can edit or merge listings afterward.
          </p>
        )}
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
      title="Edit company"
      description="Changes apply to all positions from this company."
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
          <FieldLabel htmlFor={id}>Company name</FieldLabel>
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
  applicationId,
  stage,
  onClose,
}: {
  applicationId: string;
  stage?: JobStage;
  onClose: () => void;
}) {
  const user = useJobsUser();
  const save = useJobActions(user?.id);
  const id = useId();
  const [title, setTitle] = useState(stage?.title ?? '');
  const [date, setDate] = useState(stage?.date ?? localJobDate());
  const [notes, setNotes] = useState(stage?.notes ?? '');
  return (
    <FormDialog
      title={stage ? 'Edit stage' : 'Add stage'}
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <JobForm
        onClose={onClose}
        label={stage ? 'Save changes' : 'Add stage'}
        onSave={() =>
          save(
            `/me/job-applications/${applicationId}/stages${stage ? `/${stage.id}` : ''}`,
            stage ? 'PATCH' : 'POST',
            { title, date, notes },
          )
        }
      >
        <Field>
          <FieldLabel htmlFor={`${id}-title`}>Stage name</FieldLabel>
          <Input
            id={`${id}-title`}
            required
            maxLength={120}
            readOnly={stage?.initial}
            value={title}
            placeholder="e.g. Technical interview"
            list={`${id}-suggestions`}
            onChange={(e) => setTitle(e.target.value)}
          />
          <datalist id={`${id}-suggestions`}>
            {['Assessment', 'Interview', 'Offer', 'Rejected', 'Withdrawn'].map(
              (v) => (
                <option key={v} value={v} />
              ),
            )}
          </datalist>
        </Field>
        <AppDatePicker label="Date" value={date} onChange={setDate} />
        <Field>
          <FieldLabel htmlFor={`${id}-notes`}>
            Notes · private, optional
          </FieldLabel>
          <Textarea
            id={`${id}-notes`}
            maxLength={10000}
            rows={5}
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
          />
        </Field>
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
      title="Merge duplicate position"
      description={`Merge “${source.company.name} · ${source.title}” into another listing. The selected listing keeps its details; locations and private application histories are combined.`}
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <JobForm
        onClose={onClose}
        label="Merge positions"
        onSave={() => {
          if (!targetId) throw new Error('Choose the position to keep.');
          return save(`/jobs/positions/${source.id}/merge`, 'POST', {
            targetId,
          });
        }}
      >
        <JobSelect
          label="Position to keep"
          value={targetId}
          options={[
            { value: '', label: 'Choose a position' },
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
