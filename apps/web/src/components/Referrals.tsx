import { useId, useRef, useState, type ReactNode } from 'react';
import { useSearchParams } from 'react-router-dom';
import type { ColumnDef } from '@tanstack/react-table';
import { IconAlertTriangle, IconDots, IconPlus } from '@tabler/icons-react';
import { useJobActions, useJobCompanies, useJobReferrals } from '@/api/jobs';
import type { JobReferral } from '@/api/generated/models';
import { StatusBadge } from '@/components/StatusBadge';
import { PersonIdentity } from '@/components/Common';
import { initials } from '@/utils';
import { jobCategories, jobCategoryBadgeClasses } from '@/jobs';
import { DataTable } from '@/components/DataTable';
import { NamedConfirmation } from '@/components/Dialogs';
import { JobSelect, ReferralForm } from '@/components/JobForms';
import { useJobsUser } from '@/components/JobsShell';
import { ReferralNotesHelp } from '@/components/ReferralNotesHelp';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Field, FieldLabel } from '@/components/ui/field';
import {
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover';

export function OpenToReferring() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button
        variant="outline"
        className="bg-background dark:bg-primary dark:text-primary-foreground dark:hover:bg-primary/90"
        onClick={() => setOpen(true)}
      >
        <IconPlus size={18} aria-hidden="true" />
        Open to Referring
      </Button>
      {open && <ReferralForm onClose={() => setOpen(false)} />}
    </>
  );
}

function ReferrerLink({ referral }: { referral: JobReferral }) {
  return (
    <PersonIdentity
      person={{
        ...referral.referrer,
        initials: initials(referral.referrer.name),
      }}
      showRoles={false}
    />
  );
}

function CategoryBadge({ referral }: { referral: JobReferral }) {
  const category = jobCategories.find(
    (entry) => entry.value === referral.company.category,
  )?.label;
  return (
    <StatusBadge className={jobCategoryBadgeClasses[referral.company.category]}>
      {category}
    </StatusBadge>
  );
}

function Notes({ notes }: { notes: string }) {
  return (
    <p className="whitespace-pre-wrap break-words text-sm leading-relaxed">
      {notes || '—'}
    </p>
  );
}

export function ReferralGuidelines({
  userId,
  active,
  children,
}: {
  userId?: string;
  active: boolean;
  children: (link: ReactNode, notice: ReactNode) => ReactNode;
}) {
  const id = useId();
  const storageKey = userId && `rsp-referral-guidelines-ack:v1:${userId}`;
  const [acknowledged, setAcknowledged] = useState(() => {
    try {
      return !!storageKey && localStorage.getItem(storageKey) === '1';
    } catch {
      return false;
    }
  });
  const restoreFocus = useRef(false);

  if (!active || !storageKey) return children(null, null);

  function acknowledge() {
    try {
      if (storageKey) localStorage.setItem(storageKey, '1');
    } catch {
      // The notice can still be dismissed when browser storage is unavailable.
    }
    restoreFocus.current = true;
    setAcknowledged(true);
  }

  const title = 'Read Before Contacting a Referrer';
  const rules = (
    <>
      <p>
        Only message a referrer if you meet{' '}
        <strong>every requirement in their details</strong>. Do not harass them
        or send repeated requests. Be respectful and understanding if they are
        unable or unwilling to refer you.
      </p>
      <p className="font-semibold">
        Individuals who do not follow these rules will be removed from the
        program.
      </p>
    </>
  );

  if (acknowledged)
    return children(
      <Popover>
        <PopoverTrigger
          ref={(button) => {
            if (button && restoreFocus.current) {
              button.focus();
              restoreFocus.current = false;
            }
          }}
          render={
            <Button
              variant="link"
              size="sm"
              className="px-0 text-muted-foreground underline hover:text-foreground"
            />
          }
        >
          Referral Guidelines
        </PopoverTrigger>
        <PopoverContent
          align="start"
          className="w-96 max-w-[calc(100vw-2rem)] gap-2 p-4"
        >
          <PopoverTitle className="text-base font-semibold">
            {title}
          </PopoverTitle>
          {rules}
        </PopoverContent>
      </Popover>,
      null,
    );

  return children(
    null,
    <section
      aria-labelledby={`${id}-heading`}
      className="rounded-xl border border-warning bg-warning-soft p-5 text-warning sm:p-6"
    >
      <div className="flex items-start gap-3">
        <IconAlertTriangle
          size={24}
          className="mt-0.5 shrink-0"
          aria-hidden="true"
        />
        <div className="grid min-w-0 flex-1 gap-2">
          <h2 id={`${id}-heading`} className="text-xl font-semibold">
            {title}
          </h2>
          {rules}
          <Field orientation="horizontal" className="mt-2 items-center gap-2">
            <Checkbox
              id={`${id}-acknowledge`}
              checked={false}
              className="border-current"
              onCheckedChange={(checked) => {
                if (checked) acknowledge();
              }}
            />
            <FieldLabel
              htmlFor={`${id}-acknowledge`}
              className="cursor-pointer"
            >
              I acknowledge the referral guidelines
            </FieldLabel>
          </Field>
        </div>
      </div>
    </section>,
  );
}

export function Referrals({ toolbarLead }: { toolbarLead?: ReactNode }) {
  const user = useJobsUser();
  const referrals = useJobReferrals(user?.id);
  const companies = useJobCompanies();
  const [params, setParams] = useSearchParams();
  const companyID = params.get('company') ?? 'all';
  const companyOptions = new Map(
    (referrals.data ?? []).map((r) => [r.company.id, r.company.name]),
  );
  if (companyID !== 'all' && !companyOptions.has(companyID)) {
    companyOptions.set(
      companyID,
      companies.data?.find((c) => c.id === companyID)?.name ??
        'Selected company',
    );
  }
  const shown = (referrals.data ?? []).filter(
    (r) => companyID === 'all' || r.company.id === companyID,
  );
  const save = useJobActions(user?.id);
  const [edit, setEdit] = useState<JobReferral>();
  const [remove, setRemove] = useState<JobReferral>();
  function actions(referral: JobReferral) {
    if (referral.referrer.id !== user?.id) return null;
    return (
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              variant="ghost"
              size="icon"
              aria-label={`Manage your referral listing at ${referral.company.name}`}
            />
          }
        >
          <IconDots size={18} aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem onClick={() => setEdit(referral)}>
            Edit Listing
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => setRemove(referral)}>
            Stop Referring
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    );
  }
  const columns: ColumnDef<JobReferral>[] = [
    {
      id: 'category',
      header: 'Category',
      accessorFn: (r) =>
        jobCategories.find((category) => category.value === r.company.category)
          ?.label,
      cell: ({ row }) => <CategoryBadge referral={row.original} />,
      enableColumnFilter: false,
      meta: { width: 9 },
    },
    {
      id: 'company',
      header: 'Company',
      accessorFn: (r) => r.company.name,
      enableColumnFilter: false,
      meta: { width: 12 },
    },
    {
      id: 'referrer',
      header: 'Name',
      accessorFn: (r) => r.referrer.name,
      cell: ({ row }) => <ReferrerLink referral={row.original} />,
      meta: { width: 14, wrap: true },
    },
    {
      id: 'Details',
      accessorFn: (r) => r.notes,
      header: () => (
        <span className="inline-flex items-center gap-1">
          Details
          <ReferralNotesHelp />
        </span>
      ),
      enableSorting: false,
      meta: { wrap: true },
      cell: ({ row }) => <Notes notes={row.original.notes} />,
    },
    {
      id: 'actions',
      header: '',
      enableSorting: false,
      enableColumnFilter: false,
      meta: { width: 4.5, align: 'end' },
      cell: ({ row }) => actions(row.original),
    },
  ];
  return (
    <div className="grid gap-6">
      <DataTable
        key={companyID}
        toolbarLead={toolbarLead}
        ariaLabel="Referrals"
        data={shown}
        columns={columns}
        getRowId={(referral) => referral.id}
        loading={referrals.isLoading}
        error={referrals.isError}
        onRetry={() => void referrals.refetch()}
        emptyTitle={
          companyID === 'all' ? 'No Referrals' : 'No Referrers for This Company'
        }
        emptyMessage={
          companyID === 'all'
            ? 'Members who are open to referring will appear here.'
            : 'Choose All Companies to see other members who are open to referring.'
        }
        searchPlaceholder="Search companies, people or details…"
        toolbarStart={
          <JobSelect
            label="Company"
            compact
            value={companyID}
            options={[
              { value: 'all', label: 'All Companies' },
              ...[...companyOptions]
                .sort((a, b) => a[1].localeCompare(b[1]))
                .map(([value, label]) => ({ value, label })),
            ]}
            onChange={(value) => {
              const next = new URLSearchParams(params);
              if (value === 'all') next.delete('company');
              else next.set('company', value);
              setParams(next);
            }}
          />
        }
        renderCard={({ original }) => (
          <div className="grid gap-3">
            <div className="flex items-center justify-between gap-3">
              <div className="flex min-w-0 items-center gap-4">
                <CategoryBadge referral={original} />
                <h3 className="min-w-0 truncate font-semibold">
                  {original.company.name}
                </h3>
              </div>
              {actions(original)}
            </div>
            <ReferrerLink referral={original} />
            <div>
              <div className="mb-1 flex items-center gap-1 text-sm font-medium">
                Details
                <ReferralNotesHelp />
              </div>
              <Notes notes={original.notes} />
            </div>
          </div>
        )}
      />
      {edit && (
        <ReferralForm referral={edit} onClose={() => setEdit(undefined)} />
      )}
      {remove && (
        <NamedConfirmation
          open
          name={remove.company.name}
          actionLabel="Stop Referring"
          description="Your listing will be removed. You can offer referrals again at any time."
          onOpenChange={(open) => {
            if (!open) setRemove(undefined);
          }}
          onConfirm={async () => {
            await save(`/jobs/referrals/${remove.id}`, 'DELETE');
            setRemove(undefined);
          }}
        />
      )}
    </div>
  );
}
