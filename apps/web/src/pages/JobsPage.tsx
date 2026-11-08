import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import type { ColumnDef, Row } from '@tanstack/react-table';
import {
  Link,
  useLocation,
  useNavigate,
  useSearchParams,
} from 'react-router-dom';
import { IconArrowUpRight, IconDots, IconPlus } from '@tabler/icons-react';
import {
  jobRequest,
  useJobActions,
  useJobApplications,
  useJobPositions,
} from '@/api/jobs';
import type {
  JobApplication,
  JobCompany,
  JobPosition,
  JobStage,
} from '@/api/generated/models';
import { Page, PageHeader, usePageTitle } from '@/components/Common';
import { DataTable, ExpandRowButton } from '@/components/DataTable';
import { NamedConfirmation } from '@/components/Dialogs';
import {
  CompanyForm,
  JobSelect,
  MergePositionForm,
  PositionForm,
  StageForm,
} from '@/components/JobForms';
import { useJobsUser } from '@/components/JobsShell';
import { StatusBadge } from '@/components/StatusBadge';
import { BetaBadge } from '@/components/BetaBadge';
import { Button, buttonVariants } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { formatCalendarDay } from '@/utils';
import {
  canManageJobs,
  jobCategories,
  jobCategoryControlClasses,
  jobTypes,
  localJobDate,
  positionInput,
} from '@/jobs';

export function JobsPage() {
  usePageTitle('Jobs');
  const user = useJobsUser();
  const location = useLocation();
  const navigate = useNavigate();
  const active =
    location.pathname === '/jobs/applications' ? 'applications' : 'positions';
  return (
    <Page>
      {user ? (
        <h1 className="sr-only">Jobs</h1>
      ) : (
        <PageHeader title="Jobs" status={<BetaBadge />} />
      )}
      <Tabs
        value={active}
        onValueChange={(v) =>
          navigate(v === 'applications' ? '/jobs/applications' : '/jobs')
        }
      >
        <div className="mb-4 flex min-h-9 flex-wrap items-center justify-between gap-3">
          <TabsList aria-label="Jobs">
            <TabsTrigger value="positions">Positions</TabsTrigger>
            <TabsTrigger value="applications">My Applications</TabsTrigger>
          </TabsList>
          {active === 'positions' && (
            <div className="ml-auto">
              <AddPosition />
            </div>
          )}
        </div>
        <TabsContent value="positions">
          {active === 'positions' && <Positions />}
        </TabsContent>
        <TabsContent value="applications">
          {active === 'applications' && <Applications />}
        </TabsContent>
      </Tabs>
    </Page>
  );
}

function AddPosition() {
  const user = useJobsUser();
  const [open, setOpen] = useState(false);
  if (!user)
    return (
      <Link
        className={buttonVariants({ variant: 'outline' })}
        to="/sign-in?returnTo=%2Fjobs"
      >
        Sign in to add a position
      </Link>
    );
  return (
    <>
      <Button onClick={() => setOpen(true)}>
        <IconPlus size={18} aria-hidden="true" />
        Add position
      </Button>
      {open && <PositionForm onClose={() => setOpen(false)} />}
    </>
  );
}

function CompanyLink({ position }: { position: JobPosition }) {
  return (
    <a
      href={position.applicationUrl}
      target="_blank"
      rel="noopener noreferrer"
      aria-label={`Apply for ${position.title} at ${position.company.name}`}
      title={`Apply at ${position.company.name} (opens in a new tab)`}
      className="flex min-h-8 min-w-0 items-center gap-1.5 rounded-sm font-medium text-foreground hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
    >
      <IconArrowUpRight size={16} className="shrink-0" aria-hidden="true" />
      <span className="truncate">{position.company.name}</span>
    </a>
  );
}

function JobDetailsButton<T>({
  row,
  position,
  history = false,
}: {
  row: Row<T>;
  position: JobPosition;
  history?: boolean;
}) {
  const category = jobCategories.find(
    (c) => c.value === position.company.category,
  )?.label;
  return (
    <ExpandRowButton
      row={row}
      subject={`${history ? 'history' : 'details'} for ${position.title} · ${category}`}
      title={category}
      className={`rounded-sm text-background hover:text-background aria-expanded:text-background ${jobCategoryControlClasses[position.company.category]}`}
    />
  );
}

function ApplyLink({ position }: { position: JobPosition }) {
  return (
    <a
      href={position.applicationUrl}
      target="_blank"
      rel="noopener noreferrer"
      aria-label={`Apply for ${position.title} at ${position.company.name}`}
      className="inline-flex min-h-8 items-center gap-1 font-medium"
    >
      Apply
      <IconArrowUpRight size={16} aria-hidden="true" />
    </a>
  );
}

function JobText({ text }: { text: string }) {
  return (
    <p className="whitespace-pre-wrap break-words text-sm leading-relaxed">
      {text.split(/(https:\/\/[^\s]+)/g).map((part, i) =>
        part.startsWith('https://') ? (
          <a
            key={i}
            className="break-all"
            href={part}
            target="_blank"
            rel="noopener noreferrer"
          >
            {part}
          </a>
        ) : (
          part
        ),
      )}
    </p>
  );
}

function PositionDetails({ position }: { position: JobPosition }) {
  return (
    <div className="grid gap-5 p-4 sm:grid-cols-2">
      <div className="grid content-start gap-2">
        <h3 className="font-semibold">{position.title}</h3>
        <p className="text-sm text-muted-foreground">
          {
            jobCategories.find((c) => c.value === position.company.category)
              ?.label
          }
        </p>
        <p className="text-sm text-muted-foreground">
          {position.locations.join(' · ')}
        </p>
        <ApplyLink position={position} />
        <h4 className="mt-2 text-sm font-semibold">Eligibility requirements</h4>
        <JobText text={position.eligibility || 'Not provided'} />
      </div>
      <div className="grid content-start gap-2">
        <h4 className="text-sm font-semibold">Notes</h4>
        <JobText text={position.notes || 'No notes.'} />
      </div>
    </div>
  );
}

function Positions() {
  const user = useJobsUser();
  const positions = useJobPositions();
  const applications = useJobApplications(user?.id);
  const save = useJobActions(user?.id);
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const focus = params.get('position');
  const focused = useQuery({
    queryKey: ['jobs', 'position', focus],
    queryFn: () =>
      jobRequest<JobPosition>(`/jobs/positions/${encodeURIComponent(focus!)}`),
    enabled: Boolean(focus),
  });
  const focusID = focused.data?.id ?? focus;
  const [status, setStatus] = useState('open');
  const [error, setError] = useState('');
  const [pending, setPending] = useState<string>();
  const [edit, setEdit] = useState<JobPosition>();
  const [company, setCompany] = useState<JobCompany>();
  const [merge, setMerge] = useState<JobPosition>();
  const [remove, setRemove] = useState<JobApplication>();
  const all = positions.data ?? [];
  const shown = all.filter(
    (p) => status === 'all' || p.status === status || p.id === focusID,
  );
  const ordered = [...shown].sort(
    (a, b) => Number(b.id === focusID) - Number(a.id === focusID),
  );
  const appFor = (p: JobPosition) =>
    applications.data?.find((a) => a.position.id === p.id);
  async function applied(p: JobPosition, checked: boolean) {
    if (!user) return;
    const existing = appFor(p);
    if (!checked && existing) {
      setRemove(existing);
      return;
    }
    setError('');
    setPending(p.id);
    try {
      const result =
        existing ??
        (await save<{ id: string }>('/me/job-applications', 'POST', {
          positionId: p.id,
          date: localJobDate(),
        }));
      navigate(
        `/jobs/applications?application=${encodeURIComponent(result.id)}`,
      );
    } catch (e) {
      setError(
        e instanceof Error ? e.message : 'Unable to track this application.',
      );
    } finally {
      setPending(undefined);
    }
  }
  function appliedControl(p: JobPosition) {
    return user ? (
      <div className="flex items-center gap-2">
        <Checkbox
          aria-label={`Applied to ${p.title} at ${p.company.name}`}
          checked={Boolean(appFor(p))}
          disabled={
            pending === p.id || applications.isLoading || applications.isError
          }
          onCheckedChange={(checked) => void applied(p, checked === true)}
        />
        <span className="min-[56rem]:sr-only">Applied</span>
      </div>
    ) : null;
  }
  function staffAction(action: () => void) {
    if (!user?.mfaVerified) {
      sessionStorage.setItem('rsp-auth-return-to', '/jobs');
      navigate('/two-factor');
      return;
    }
    action();
  }
  async function toggleStatus(p: JobPosition) {
    setPending(p.id);
    setError('');
    try {
      await save(`/jobs/positions/${p.id}`, 'PATCH', {
        ...positionInput(p),
        status: p.status === 'open' ? 'closed' : 'open',
      });
    } catch (e) {
      setError(
        e instanceof Error ? e.message : 'Unable to update the position.',
      );
    } finally {
      setPending(undefined);
    }
  }
  function staffMenu(p: JobPosition) {
    return canManageJobs(user) ? (
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              variant="ghost"
              size="icon"
              disabled={pending === p.id}
              aria-label={`Manage ${p.title} at ${p.company.name}`}
            />
          }
        >
          <IconDots size={18} aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem onClick={() => staffAction(() => setEdit(p))}>
            Edit position
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => staffAction(() => setCompany(p.company))}
          >
            Edit company
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => staffAction(() => void toggleStatus(p))}
          >
            {p.status === 'open' ? 'Close position' : 'Reopen position'}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => staffAction(() => setMerge(p))}>
            Merge duplicate
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    ) : null;
  }
  const columns: ColumnDef<JobPosition>[] = [
    {
      id: 'details',
      header: 'Details',
      cell: ({ row }) => <JobDetailsButton row={row} position={row.original} />,
      enableSorting: false,
      enableColumnFilter: false,
      meta: { width: 4.5 },
    },
    {
      id: 'company',
      accessorFn: (p) => p.company.name,
      header: 'Company',
      cell: ({ row }) => <CompanyLink position={row.original} />,
      meta: { width: 12 },
    },
    {
      accessorKey: 'title',
      header: 'Job Title',
      cell: ({ row }) => (
        <div className="flex items-center gap-2">
          <span className="truncate">{row.original.title}</span>
          {row.original.status === 'closed' && (
            <StatusBadge>Closed</StatusBadge>
          )}
        </div>
      ),
    },
    {
      accessorKey: 'jobType',
      header: 'Job Type',
      cell: ({ row }) =>
        jobTypes.find((t) => t.value === row.original.jobType)?.label,
      meta: { width: 7, filterOptions: jobTypes },
    },
    {
      accessorKey: 'eligibility',
      header: 'Eligibility Requirements',
      cell: ({ row }) => row.original.eligibility || '—',
      meta: { width: 10 },
    },
    {
      id: 'location',
      accessorFn: (p) => p.locations.join(' · '),
      header: 'Location',
      meta: { width: 10 },
    },
    {
      accessorKey: 'notes',
      header: 'Notes',
      cell: ({ row }) => row.original.notes || '—',
      meta: { width: 9 },
    },
    ...(user
      ? [
          {
            id: 'applied',
            header: 'Applied',
            cell: ({ row }) => appliedControl(row.original),
            enableColumnFilter: false,
            enableSorting: false,
            meta: { width: 5 },
          } as ColumnDef<JobPosition>,
        ]
      : []),
    ...(canManageJobs(user)
      ? [
          {
            id: 'actions',
            header: '',
            cell: ({ row }) => staffMenu(row.original),
            enableColumnFilter: false,
            enableSorting: false,
            meta: { width: 3 },
          } as ColumnDef<JobPosition>,
        ]
      : []),
  ];
  const [category, setCategory] = useState('all');
  const filtered = ordered.filter(
    (p) => category === 'all' || p.company.category === category,
  );
  return (
    <>
      {error && (
        <p role="alert" className="mb-4 text-sm text-destructive">
          {error}
        </p>
      )}
      {applications.isError && (
        <p role="alert" className="mb-4 text-sm">
          Could not load your Applied markers.{' '}
          <Button variant="link" onClick={() => void applications.refetch()}>
            Try again
          </Button>
        </p>
      )}
      {focused.isError && (
        <p role="alert" className="mb-4 text-sm">
          That position could not be loaded. You can still browse the board.
        </p>
      )}
      <DataTable
        key={focusID ?? 'positions'}
        ariaLabel="Job positions"
        data={filtered}
        columns={columns}
        getRowId={(p) => p.id}
        defaultExpanded={focusID ? { [focusID]: true } : false}
        loading={positions.isLoading}
        error={positions.isError}
        onRetry={() => void positions.refetch()}
        emptyTitle="No Positions"
        emptyMessage="New positions will appear here when they are shared."
        searchPlaceholder="Search positions…"
        toolbarStart={
          <JobSelect
            label="Listings"
            compact
            value={status}
            options={[
              { value: 'open', label: 'Open positions' },
              { value: 'all', label: 'All positions' },
              { value: 'closed', label: 'Closed positions' },
            ]}
            onChange={setStatus}
          />
        }
        extraFilters={{
          content: (
            <JobSelect
              label="Company category"
              value={category}
              options={[
                { value: 'all', label: 'All categories' },
                ...jobCategories,
              ]}
              onChange={setCategory}
            />
          ),
          activeCount: category === 'all' ? 0 : 1,
          onClear: () => setCategory('all'),
        }}
        renderExpanded={({ original }) => (
          <PositionDetails position={original} />
        )}
        renderCard={(row) => (
          <div className="grid gap-3">
            <div className="flex items-center justify-between gap-3">
              <div className="flex min-w-0 items-center gap-3">
                <JobDetailsButton row={row} position={row.original} />
                <CompanyLink position={row.original} />
              </div>
              {staffMenu(row.original)}
            </div>
            <h3 className="font-semibold">{row.original.title}</h3>
            <p className="text-sm text-muted-foreground">
              {jobTypes.find((t) => t.value === row.original.jobType)?.label} ·{' '}
              {row.original.locations.join(' · ')}
              {row.original.status === 'closed' ? ' · Closed' : ''}
            </p>
            {user && appliedControl(row.original)}
            {row.getIsExpanded() && <PositionDetails position={row.original} />}
          </div>
        )}
      />
      {edit && (
        <PositionForm position={edit} onClose={() => setEdit(undefined)} />
      )}
      {company && (
        <CompanyForm company={company} onClose={() => setCompany(undefined)} />
      )}
      {merge && (
        <MergePositionForm
          source={merge}
          positions={all}
          onClose={() => setMerge(undefined)}
        />
      )}
      {remove && (
        <NamedConfirmation
          open
          name={remove.position.title}
          actionLabel="Remove application"
          description="This removes your application and all its private stage notes. The shared position stays on the board."
          onOpenChange={(open) => {
            if (!open) setRemove(undefined);
          }}
          onConfirm={() =>
            save(`/me/job-applications/${remove.id}`, 'DELETE').then(
              () => undefined,
            )
          }
        />
      )}
    </>
  );
}

function Applications() {
  const user = useJobsUser();
  const applications = useJobApplications(user?.id);
  const [params] = useSearchParams();
  const focus = params.get('application');
  const ordered = [...(applications.data ?? [])].sort(
    (a, b) => Number(b.id === focus) - Number(a.id === focus),
  );
  const columns: ColumnDef<JobApplication>[] = [
    {
      id: 'details',
      header: 'Details',
      cell: ({ row }) => (
        <JobDetailsButton row={row} position={row.original.position} history />
      ),
      enableSorting: false,
      enableColumnFilter: false,
      meta: { width: 4.5 },
    },
    {
      id: 'company',
      accessorFn: (a) => a.position.company.name,
      header: 'Company',
      cell: ({ row }) => <CompanyLink position={row.original.position} />,
      meta: { width: 12 },
    },
    { id: 'title', accessorFn: (a) => a.position.title, header: 'Job Title' },
    {
      id: 'type',
      accessorFn: (a) => a.position.jobType,
      header: 'Job Type',
      cell: ({ row }) =>
        jobTypes.find((t) => t.value === row.original.position.jobType)?.label,
      meta: { width: 8, filterOptions: jobTypes },
    },
    {
      id: 'stage',
      accessorFn: (a) => a.stages.at(-1)?.title ?? 'Applied',
      header: 'Latest Stage',
      meta: { width: 12 },
    },
    {
      id: 'date',
      accessorFn: (a) => a.stages[0]?.date ?? '',
      header: 'Applied',
      cell: ({ row }) =>
        row.original.stages[0]
          ? formatCalendarDay(new Date(row.original.stages[0].date))
          : '—',
      meta: { width: 8 },
    },
  ];
  return (
    <DataTable
      key={focus ?? 'applications'}
      ariaLabel="My applications"
      data={ordered}
      columns={columns}
      getRowId={(a) => a.id}
      defaultExpanded={focus ? { [focus]: true } : false}
      loading={applications.isLoading}
      error={applications.isError}
      onRetry={() => void applications.refetch()}
      emptyTitle="No Applications"
      emptyMessage="Mark a position as applied to start tracking it."
      searchPlaceholder="Search your applications…"
      renderExpanded={({ original }) => (
        <ApplicationHistory application={original} />
      )}
      renderCard={(row) => (
        <div className="grid gap-3">
          <div className="flex min-w-0 items-center gap-3">
            <JobDetailsButton
              row={row}
              position={row.original.position}
              history
            />
            <CompanyLink position={row.original.position} />
          </div>
          <h3 className="font-semibold">{row.original.position.title}</h3>
          <div className="flex items-center justify-between gap-2">
            <p className="text-sm">
              {row.original.stages.at(-1)?.title ?? 'Applied'}
            </p>
          </div>
          {row.getIsExpanded() && (
            <ApplicationHistory application={row.original} />
          )}
        </div>
      )}
    />
  );
}

function ApplicationHistory({ application }: { application: JobApplication }) {
  const user = useJobsUser();
  const save = useJobActions(user?.id);
  const [stage, setStage] = useState<JobStage | 'new'>();
  const [remove, setRemove] = useState<JobStage | 'application'>();
  return (
    <section
      className="grid gap-4 p-4"
      aria-label={`Application history for ${application.position.title}`}
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-3">
          <ApplyLink position={application.position} />
          <span className="text-sm text-muted-foreground">
            {
              jobCategories.find(
                (c) => c.value === application.position.company.category,
              )?.label
            }
          </span>
          {application.position.status === 'closed' && (
            <StatusBadge>Position closed</StatusBadge>
          )}
        </div>
        <Button size="sm" onClick={() => setStage('new')}>
          <IconPlus size={16} aria-hidden="true" />
          Add stage
        </Button>
      </div>
      <ol className="divide-y">
        {application.stages.map((s) => (
          <li key={s.id} className="grid gap-2 py-4">
            <div className="flex flex-wrap items-start justify-between gap-2">
              <div className="grid gap-1">
                <h3 className="font-semibold">{s.title}</h3>
                <time
                  className="text-sm text-muted-foreground"
                  dateTime={s.date}
                >
                  {formatCalendarDay(new Date(s.date))}
                </time>
              </div>
              <DropdownMenu>
                <DropdownMenuTrigger
                  render={
                    <Button
                      variant="ghost"
                      size="icon"
                      aria-label={`Actions for ${s.title}`}
                    />
                  }
                >
                  <IconDots size={18} aria-hidden="true" />
                </DropdownMenuTrigger>
                <DropdownMenuContent>
                  <DropdownMenuItem onClick={() => setStage(s)}>
                    Edit stage
                  </DropdownMenuItem>
                  {!s.initial && (
                    <DropdownMenuItem onClick={() => setRemove(s)}>
                      Remove stage
                    </DropdownMenuItem>
                  )}
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
            {s.notes && <JobText text={s.notes} />}
          </li>
        ))}
      </ol>
      <Button
        variant="outline"
        size="sm"
        className="w-fit"
        onClick={() => setRemove('application')}
      >
        Remove application
      </Button>
      {stage && (
        <StageForm
          applicationId={application.id}
          stage={stage === 'new' ? undefined : stage}
          onClose={() => setStage(undefined)}
        />
      )}
      {remove && (
        <NamedConfirmation
          open
          name={
            remove === 'application' ? application.position.title : remove.title
          }
          actionLabel={
            remove === 'application' ? 'Remove application' : 'Remove stage'
          }
          description={
            remove === 'application'
              ? 'This removes your application and all its private stage notes. The shared position stays on the board.'
              : 'This removes the stage and its private notes.'
          }
          onOpenChange={(open) => {
            if (!open) setRemove(undefined);
          }}
          onConfirm={() =>
            save(
              `/me/job-applications/${application.id}${remove === 'application' ? '' : `/stages/${remove.id}`}`,
              'DELETE',
            ).then(() => undefined)
          }
        />
      )}
    </section>
  );
}
