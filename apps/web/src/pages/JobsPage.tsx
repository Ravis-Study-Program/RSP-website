import { JobApplicationForm } from '@/components/JobApplicationForm';
import { JobProgressChart } from '@/components/JobProgressChart';
import { currentJobStage } from '@/jobStages';
import { useState, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import type { ColumnDef, Row, SortingState } from '@tanstack/react-table';
import {
  Link,
  useLocation,
  useNavigate,
  useSearchParams,
} from 'react-router-dom';
import {
  IconArchive,
  IconArchiveOff,
  IconBuilding,
  IconDotsVertical,
  IconEdit,
  IconGitMerge,
  IconPlus,
  IconTrash,
} from '@tabler/icons-react';
import {
  jobRequest,
  useJobActions,
  useJobApplications,
  useJobPositions,
  useJobReferralCounts,
} from '@/api/jobs';
import type {
  JobApplication,
  JobStage,
  JobCompany,
  JobPosition,
} from '@/api/generated/models';
import { Page, PageHeader, usePageTitle } from '@/components/Common';
import { DataTable, ExpandRowButton } from '@/components/DataTable';
import { ExternalLink } from '@/components/ExternalLink';
import { NamedConfirmation } from '@/components/Dialogs';
import {
  CompanyForm,
  JobSelect,
  MergePositionForm,
  PositionForm,
  StageForm,
} from '@/components/JobForms';
import { useJobsUser } from '@/components/JobsShell';
import { ReferralGuidelines, Referrals } from '@/components/Referrals';
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
import { cn } from '@/lib/utils';
import { formatCalendarDay } from '@/utils';
import {
  canAccessJobReferrals,
  canManageJobs,
  canRequestJobReferral,
  jobApplicationStatuses,
  jobCategories,
  jobCategoryControlClasses,
  jobTypes,
  localJobDate,
  positionInput,
} from '@/jobs';

export function JobsPage() {
  usePageTitle('Jobs');
  const user = useJobsUser();
  const referralsEnabled = canAccessJobReferrals(user);
  const requestEnabled = Boolean(
    user && referralsEnabled && canRequestJobReferral(user),
  );
  const location = useLocation();
  const navigate = useNavigate();
  const active = location.pathname.split('/')[2] || 'positions';
  const pageTabs = (
    <TabsList
      aria-label="Jobs"
      className={cn(
        'group-data-[orientation=horizontal]/tabs:h-9',
        referralsEnabled &&
          'max-[30rem]:w-full max-[30rem]:flex-wrap max-[30rem]:group-data-[orientation=horizontal]/tabs:h-auto max-[30rem]:[&>[role=tab]]:h-8',
        requestEnabled && 'max-[30rem]:grid max-[30rem]:grid-cols-2',
      )}
    >
      <TabsTrigger value="positions">Positions</TabsTrigger>
      <TabsTrigger value="applications">My Applications</TabsTrigger>
      {requestEnabled && (
        <TabsTrigger value="referrals">Request Referrals</TabsTrigger>
      )}
      {referralsEnabled && (
        <TabsTrigger value="refer-candidates">Refer Candidates</TabsTrigger>
      )}
    </TabsList>
  );
  const renderPage = (
    guidelinesLink: ReactNode,
    guidelinesNotice: ReactNode,
  ) => {
    const referralTab = active === 'referrals' || active === 'refer-candidates';
    const layout = (actions: ReactNode, referralContent?: ReactNode) => (
      <>
        <PageHeader
          title="Jobs"
          status={
            <>
              <BetaBadge />
              {guidelinesLink}
            </>
          }
          actions={actions}
        />
        <Tabs
          value={active}
          onValueChange={(value) =>
            navigate(value === 'positions' ? '/jobs' : `/jobs/${value}`)
          }
        >
          <TabsContent value="positions">
            {active === 'positions' && <Positions toolbarLead={pageTabs} />}
          </TabsContent>
          <TabsContent value="applications">
            {active === 'applications' && (
              <Applications toolbarLead={pageTabs} />
            )}
          </TabsContent>
          {(['referrals', 'refer-candidates'] as const).map((tab) => (
            <TabsContent key={tab} value={tab} className="space-y-6">
              {active === tab && (
                <>
                  {guidelinesNotice}
                  {referralContent}
                </>
              )}
            </TabsContent>
          ))}
        </Tabs>
      </>
    );
    return (
      <Page>
        {referralTab &&
        user &&
        (active === 'referrals' ? requestEnabled : referralsEnabled) ? (
          <Referrals
            toolbarLead={pageTabs}
            userId={user.id}
            refer={active === 'refer-candidates'}
            canRefer={user.alumni}
            layout={layout}
          />
        ) : (
          layout(active === 'positions' ? <AddPosition /> : undefined)
        )}
      </Page>
    );
  };
  return (
    <ReferralGuidelines
      key={user?.id}
      userId={user?.id}
      page={
        active === 'referrals' || active === 'refer-candidates'
          ? active
          : undefined
      }
    >
      {renderPage}
    </ReferralGuidelines>
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
        Sign In to Add a Position
      </Link>
    );
  return (
    <>
      <Button onClick={() => setOpen(true)}>
        <IconPlus size={18} aria-hidden="true" />
        Add Position
      </Button>
      {open && <PositionForm onClose={() => setOpen(false)} />}
    </>
  );
}

function CompanyLink({ position }: { position: JobPosition }) {
  return (
    <ExternalLink
      href={position.applicationUrl}
      aria-label={`Apply for ${position.title} at ${position.company.name}`}
      title={`Apply at ${position.company.name} (opens in a new tab)`}
      className="min-h-8 min-w-0 rounded-sm font-medium text-foreground"
    >
      <span className="truncate">{position.company.name}</span>
    </ExternalLink>
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
      className={`rounded-sm hover:text-job-category-foreground aria-expanded:text-job-category-foreground ${jobCategoryControlClasses[position.company.category]}`}
    />
  );
}

function JobText({ text }: { text: string }) {
  return (
    <p className="whitespace-pre-wrap break-words text-sm leading-relaxed">
      {text.split(/(https:\/\/[^\s]+)/g).map((part, i) =>
        part.startsWith('https://') ? (
          <ExternalLink key={i} href={part}>
            <span className="min-w-0 break-all">{part}</span>
          </ExternalLink>
        ) : (
          part
        ),
      )}
    </p>
  );
}

function PositionOverview({
  position,
  showTitle = true,
}: {
  position: JobPosition;
  showTitle?: boolean;
}) {
  const type = jobTypes.find(
    (entry) => entry.value === position.jobType,
  )?.label;
  const category = jobCategories.find(
    (entry) => entry.value === position.company.category,
  )?.label;
  return (
    <div className="grid min-w-0 gap-2">
      {showTitle && (
        <h3 className="text-base font-semibold [overflow-wrap:anywhere]">
          {position.title}
        </h3>
      )}
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted-foreground">
        {showTitle && (
          <span className="font-medium text-foreground [overflow-wrap:anywhere]">
            {position.company.name}
          </span>
        )}
        <span className="[overflow-wrap:anywhere]">
          {[type, category, ...position.locations].filter(Boolean).join(' · ')}
        </span>
        <span className="whitespace-nowrap">
          Added{' '}
          <time dateTime={position.createdAt}>
            {formatCalendarDay(new Date(position.createdAt))}
          </time>
        </span>
        {position.status === 'closed' && (
          <StatusBadge>Position Closed</StatusBadge>
        )}
      </div>
      {position.eligibility && (
        <div className="grid gap-1">
          <h4 className="text-sm font-semibold">Eligibility Requirements</h4>
          <JobText text={position.eligibility} />
        </div>
      )}
    </div>
  );
}

function PositionNotes({ notes }: { notes: string }) {
  if (!notes) return null;
  return (
    <div className="grid gap-1">
      <h4 className="text-sm font-semibold">Position Notes</h4>
      <JobText text={notes} />
    </div>
  );
}

function PositionDetails({
  position,
  showOverview = true,
}: {
  position: JobPosition;
  showOverview?: boolean;
}) {
  return (
    <div className="grid min-w-0 max-w-[calc(100cqw-2rem)] gap-4 min-[56rem]:sticky min-[56rem]:left-4">
      {showOverview && <PositionOverview position={position} />}
      <PositionNotes notes={position.notes} />
      <JobProgressChart positionId={position.id} />
    </div>
  );
}

function PositionAddedDate({
  position,
  showLabel = false,
}: {
  position: JobPosition;
  showLabel?: boolean;
}) {
  const addedAt = new Date(position.createdAt);
  const age = Date.now() - addedAt.getTime();
  const isNew =
    position.status === 'open' && age >= 0 && age < 48 * 60 * 60 * 1000;

  return (
    <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
      <span className="whitespace-nowrap">
        {showLabel && 'Added '}
        <time dateTime={position.createdAt}>{formatCalendarDay(addedAt)}</time>
      </span>
      {isNew && <StatusBadge tone="info">New</StatusBadge>}
    </span>
  );
}

function Positions({ toolbarLead }: { toolbarLead: ReactNode }) {
  const user = useJobsUser();
  const positions = useJobPositions();
  const applications = useJobApplications(user?.id);
  const referralCounts = useJobReferralCounts(
    canAccessJobReferrals(user) ? user?.id : undefined,
  );
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
  const [applicationFilter, setApplicationFilter] = useState(() => {
    const saved =
      user && localStorage.getItem(`rsp-jobs-applied-filter:${user.id}`);
    return saved === 'applied' || saved === 'not-applied' ? saved : 'all';
  });
  const [sorting, setSorting] = useState<SortingState>([
    { id: 'added', desc: true },
  ]);
  const [error, setError] = useState('');
  const [pendingPositions, setPendingPositions] = useState<Set<string>>(
    () => new Set(),
  );
  const [edit, setEdit] = useState<JobPosition>();
  const [company, setCompany] = useState<JobCompany>();
  const [merge, setMerge] = useState<JobPosition>();
  const [closePosition, setClosePosition] = useState<JobPosition>();
  const [remove, setRemove] = useState<JobApplication>();
  const all = positions.data ?? [];
  const appliedIDs = new Set(applications.data?.map((a) => a.position.id));
  const referrerCounts = new Map(
    referralCounts.data?.map(({ companyId, count }) => [companyId, count]),
  );
  const shown = all.filter(
    (p) => status === 'all' || p.status === status || p.id === focusID,
  );
  const ordered = [...shown].sort(
    (a, b) => Number(b.id === focusID) - Number(a.id === focusID),
  );
  const appFor = (p: JobPosition) =>
    applications.data?.find((a) => a.position.id === p.id);
  function setPositionPending(id: string, pending: boolean) {
    setPendingPositions((previous) => {
      const next = new Set(previous);
      if (pending) next.add(id);
      else next.delete(id);
      return next;
    });
  }
  function companyCell(position: JobPosition) {
    const count =
      position.status === 'open'
        ? (referrerCounts.get(position.company.id) ?? 0)
        : 0;
    return (
      <div className="grid min-w-0 gap-1">
        <CompanyLink position={position} />
        {count > 0 &&
          (user && canRequestJobReferral(user) ? (
            <Link
              to={`/jobs/referrals?company=${encodeURIComponent(position.company.id)}`}
              aria-label={`${count} ${count === 1 ? 'referrer' : 'referrers'} at ${position.company.name}. Browse referral positions.`}
              className="w-fit rounded-sm text-xs text-muted-foreground hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
            >
              {count} {count === 1 ? 'referrer' : 'referrers'}
            </Link>
          ) : (
            <span className="text-xs text-muted-foreground">
              {count} {count === 1 ? 'referrer' : 'referrers'}
            </span>
          ))}
      </div>
    );
  }
  async function applied(p: JobPosition, checked: boolean) {
    if (!user) return;
    const existing = appFor(p);
    if (!checked) {
      setRemove(existing);
      return;
    }
    if (existing) return;
    setError('');
    setPositionPending(p.id, true);
    try {
      await save('/me/job-applications', 'POST', {
        positionId: p.id,
        date: localJobDate(),
      });
    } catch (e) {
      setError(
        e instanceof Error ? e.message : 'Unable to track this application.',
      );
    } finally {
      setPositionPending(p.id, false);
    }
  }
  function appliedControl(p: JobPosition) {
    return user ? (
      <div className="flex items-center gap-2">
        <Checkbox
          aria-label={`Applied to ${p.title} at ${p.company.name}`}
          checked={Boolean(appFor(p))}
          disabled={
            pendingPositions.has(p.id) ||
            applications.isLoading ||
            applications.isError
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
    setPositionPending(p.id, true);
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
      setPositionPending(p.id, false);
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
              disabled={pendingPositions.has(p.id)}
              aria-label={`Manage ${p.title} at ${p.company.name}`}
            />
          }
        >
          <IconDotsVertical size={18} aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem onClick={() => staffAction(() => setEdit(p))}>
            <IconEdit size={17} aria-hidden="true" /> Edit Position
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => staffAction(() => setCompany(p.company))}
          >
            <IconBuilding size={17} aria-hidden="true" /> Edit Company
          </DropdownMenuItem>
          <DropdownMenuItem
            variant={p.status === 'open' ? 'destructive' : 'default'}
            onClick={() =>
              staffAction(() => {
                if (p.status === 'open') setClosePosition(p);
                else void toggleStatus(p);
              })
            }
          >
            {p.status === 'open' ? (
              <IconArchive size={17} aria-hidden="true" />
            ) : (
              <IconArchiveOff size={17} aria-hidden="true" />
            )}
            {p.status === 'open' ? 'Close Position' : 'Reopen Position'}
          </DropdownMenuItem>
          <DropdownMenuItem
            variant="destructive"
            onClick={() => staffAction(() => setMerge(p))}
          >
            <IconGitMerge size={17} aria-hidden="true" /> Merge Duplicate
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
      cell: ({ row }) => companyCell(row.original),
      meta: { width: 12, wrap: true },
    },
    {
      accessorKey: 'title',
      header: 'Job Title',
      cell: ({ row }) => (
        <div className="flex items-center gap-2">
          <span className="truncate" title={row.original.title}>
            {row.original.title}
          </span>
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
      cell: ({ row }) => (
        <span title={row.original.eligibility}>
          {row.original.eligibility || '—'}
        </span>
      ),
      meta: { width: 10 },
    },
    {
      id: 'location',
      accessorFn: (p) => p.locations.join(' · '),
      header: 'Locations',
      meta: { width: 10 },
    },
    {
      id: 'added',
      accessorFn: (p) => new Date(p.createdAt).getTime(),
      sortingFn: (a, b) => {
        // Direct position links stay first under either date ordering.
        const focusedOrder =
          Number(a.original.id === focusID) - Number(b.original.id === focusID);
        if (focusedOrder)
          return (
            focusedOrder *
            (sorting.find((s) => s.id === 'added')?.desc ? 1 : -1)
          );
        return a.getValue<number>('added') - b.getValue<number>('added');
      },
      header: 'Added',
      cell: ({ row }) => <PositionAddedDate position={row.original} />,
      enableColumnFilter: false,
      meta: { width: 7 },
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
            meta: { width: 4.5, align: 'end' },
          } as ColumnDef<JobPosition>,
        ]
      : []),
  ];
  const [category, setCategory] = useState('all');
  const filtered = ordered.filter(
    (p) =>
      p.id === focusID ||
      ((category === 'all' || p.company.category === category) &&
        (!user ||
          applicationFilter === 'all' ||
          (applications.isSuccess &&
            appliedIDs.has(p.id) === (applicationFilter === 'applied')))),
  );
  const sortOrder =
    sorting[0]?.id === 'added'
      ? sorting[0].desc
        ? 'newest'
        : 'oldest'
      : 'custom';
  return (
    <>
      {error && (
        <p role="alert" className="mb-4 text-sm text-destructive">
          {error}
        </p>
      )}
      {applications.isError && applicationFilter === 'all' && (
        <p role="alert" className="mb-4 text-sm">
          Could not load your Applied markers.{' '}
          <Button variant="link" onClick={() => void applications.refetch()}>
            Try Again
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
        toolbarLead={toolbarLead}
        ariaLabel="Job positions"
        data={filtered}
        columns={columns}
        sorting={sorting}
        onSortingChange={setSorting}
        getRowId={(p) => p.id}
        defaultExpanded={focusID ? { [focusID]: true } : false}
        loading={
          positions.isLoading ||
          (Boolean(user) &&
            applicationFilter !== 'all' &&
            applications.isPending)
        }
        error={
          positions.isError ||
          (Boolean(user) && applicationFilter !== 'all' && applications.isError)
        }
        onRetry={() => {
          void positions.refetch();
          if (user) void applications.refetch();
        }}
        emptyTitle={
          applicationFilter === 'all' ? 'No Positions' : 'No Matching Positions'
        }
        emptyMessage={
          applicationFilter === 'all'
            ? 'New positions will appear here when they are shared.'
            : 'Try All to see positions regardless of whether you have applied.'
        }
        searchPlaceholder="Search positions…"
        toolbarStart={
          <>
            <JobSelect
              label="Listings"
              compact
              value={status}
              options={[
                { value: 'open', label: 'Open Positions' },
                { value: 'all', label: 'All Positions' },
                { value: 'closed', label: 'Closed Positions' },
              ]}
              onChange={setStatus}
            />
            {user && (
              <JobSelect
                label="Application Filter"
                compact
                value={applicationFilter}
                disabled={applications.isPending || applications.isError}
                options={[
                  { value: 'all', label: 'All' },
                  { value: 'not-applied', label: 'Not Applied' },
                  { value: 'applied', label: 'Applied' },
                ]}
                onChange={(value) => {
                  setApplicationFilter(value);
                  localStorage.setItem(
                    `rsp-jobs-applied-filter:${user.id}`,
                    value,
                  );
                }}
              />
            )}
            <JobSelect
              label="Sort Positions"
              compact
              value={sortOrder}
              options={[
                { value: 'newest', label: 'Newest' },
                { value: 'oldest', label: 'Oldest' },
                ...(sortOrder === 'custom'
                  ? [{ value: 'custom', label: 'Custom Sort' }]
                  : []),
              ]}
              onChange={(value) => {
                if (value !== 'custom')
                  setSorting([{ id: 'added', desc: value === 'newest' }]);
              }}
            />
          </>
        }
        extraFilters={{
          content: (
            <JobSelect
              label="Company Category"
              value={category}
              options={[
                { value: 'all', label: 'All Categories' },
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
                {companyCell(row.original)}
              </div>
              {staffMenu(row.original)}
            </div>
            <h3 className="font-semibold">{row.original.title}</h3>
            <p className="text-sm text-muted-foreground">
              {jobTypes.find((t) => t.value === row.original.jobType)?.label} ·{' '}
              {row.original.locations.join(' · ')}
              {row.original.status === 'closed' ? ' · Closed' : ''}
            </p>
            <p className="text-sm text-muted-foreground">
              <PositionAddedDate position={row.original} showLabel />
            </p>
            {row.original.eligibility && (
              <div className="grid gap-1 text-sm">
                <span className="text-muted-foreground">
                  Eligibility Requirements
                </span>
                <JobText text={row.original.eligibility} />
              </div>
            )}
            {user && appliedControl(row.original)}
            {row.getIsExpanded() && (
              <PositionDetails position={row.original} showOverview={false} />
            )}
          </div>
        )}
      />
      {edit && (
        <PositionForm position={edit} onClose={() => setEdit(undefined)} />
      )}
      {company && (
        <CompanyForm company={company} onClose={() => setCompany(undefined)} />
      )}
      {closePosition && (
        <NamedConfirmation
          open
          name={closePosition.title}
          actionLabel="Close Position"
          description={`This closes the listing at ${closePosition.company.name} and removes it from Open Positions. Existing applications and private notes are kept. You can reopen it later.`}
          onOpenChange={(open) => {
            if (!open) setClosePosition(undefined);
          }}
          onConfirm={() =>
            save(`/jobs/positions/${closePosition.id}`, 'PATCH', {
              ...positionInput(closePosition),
              status: 'closed',
            }).then(() => undefined)
          }
        />
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
          title={`Remove application for ${remove.position.company.name} (${remove.position.title})?`}
          actionLabel="Remove Application"
          confirmLabel="Confirm"
          requireTyping={false}
          description="This removes your application and all its private stage notes."
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

function Applications({ toolbarLead }: { toolbarLead: ReactNode }) {
  const user = useJobsUser();
  const applications = useJobApplications(user?.id);
  const [status, setStatus] = useState('active');
  const [editingId, setEditingId] = useState<string>();
  const editing = applications.data?.find((a) => a.id === editingId);
  const [params] = useSearchParams();
  const focus = params.get('application');
  const ordered = (applications.data ?? [])
    .filter((a) => status === 'all' || a.status === status)
    .sort((a, b) => Number(b.id === focus) - Number(a.id === focus));
  function editButton(application: JobApplication) {
    return (
      <Button
        type="button"
        variant="ghost"
        size="icon"
        title="Edit"
        aria-label={`Edit application for ${application.position.title} at ${application.position.company.name}`}
        onClick={() => setEditingId(application.id)}
      >
        <IconEdit size={17} aria-hidden="true" />
      </Button>
    );
  }

  function statusBadge(application: JobApplication) {
    return (
      <StatusBadge
        role="status"
        aria-label={`Status for ${application.position.title} at ${application.position.company.name}`}
        className={
          application.status === 'inactive'
            ? 'text-[color-mix(in_srgb,var(--destructive)_78%,var(--foreground))]'
            : undefined
        }
        tone={
          application.status === 'offer'
            ? 'success'
            : application.status === 'inactive'
              ? 'danger'
              : 'info'
        }
      >
        {
          jobApplicationStatuses.find((s) => s.value === application.status)
            ?.label
        }
      </StatusBadge>
    );
  }
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
      meta: { width: 7, filterOptions: jobTypes },
    },
    {
      id: 'stage',
      accessorFn: currentJobStage,
      header: 'Current Stage',
      meta: { width: 10 },
    },
    {
      accessorKey: 'status',
      header: 'Status',
      cell: ({ row }) => statusBadge(row.original),
      enableColumnFilter: false,
      meta: { width: 6.5 },
    },
    {
      id: 'date',
      accessorFn: (a) => a.stages[0]?.date ?? '',
      header: 'Applied',
      cell: ({ row }) =>
        row.original.stages[0]
          ? formatCalendarDay(new Date(row.original.stages[0].date))
          : '—',
      meta: { width: 7 },
    },
    {
      id: 'edit',
      header: 'Edit',
      cell: ({ row }) => !row.getIsExpanded() && editButton(row.original),
      enableSorting: false,
      enableColumnFilter: false,
      enableHiding: false,
      meta: { width: 4.5, align: 'end' },
    },
  ];
  return (
    <>
      <DataTable
        key={focus ?? 'applications'}
        toolbarLead={toolbarLead}
        ariaLabel="My applications"
        data={ordered}
        columns={columns}
        getRowId={(a) => a.id}
        defaultExpanded={focus ? { [focus]: true } : false}
        loading={applications.isLoading}
        error={applications.isError}
        onRetry={() => void applications.refetch()}
        emptyTitle={
          status === 'all'
            ? 'No Applications'
            : `No ${jobApplicationStatuses.find((s) => s.value === status)?.label} Applications`
        }
        emptyMessage={
          applications.data?.length
            ? 'Choose another status to view your other applications.'
            : 'Mark a position as applied to start tracking it.'
        }
        searchPlaceholder="Search your applications…"
        toolbarStart={
          <JobSelect
            label="Application Status Filter"
            compact
            value={status}
            options={[
              ...jobApplicationStatuses,
              { value: 'all', label: 'All' },
            ]}
            onChange={setStatus}
          />
        }
        renderExpanded={({ original }) => (
          <ApplicationHistory
            application={original}
            onEdit={() => setEditingId(original.id)}
          />
        )}
        renderCard={(row) => (
          <div className="grid gap-3">
            <div className="flex items-center justify-between gap-3">
              <div className="flex min-w-0 items-center gap-3">
                <JobDetailsButton
                  row={row}
                  position={row.original.position}
                  history
                />
                <CompanyLink position={row.original.position} />
              </div>
              {!row.getIsExpanded() && editButton(row.original)}
            </div>
            <h3 className="font-semibold">{row.original.position.title}</h3>
            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className="text-sm">{currentJobStage(row.original)}</p>
              {statusBadge(row.original)}
            </div>
            {row.getIsExpanded() && (
              <ApplicationHistory
                application={row.original}
                onEdit={() => setEditingId(row.original.id)}
                showPositionTitle={false}
              />
            )}
          </div>
        )}
      />
      {editing && (
        <JobApplicationForm
          application={editing}
          onClose={() => setEditingId(undefined)}
        />
      )}
    </>
  );
}

function ApplicationHistory({
  application,
  onEdit,
  showPositionTitle = true,
}: {
  application: JobApplication;
  onEdit: () => void;
  showPositionTitle?: boolean;
}) {
  const user = useJobsUser();
  const save = useJobActions(user?.id);
  const [stage, setStage] = useState<JobStage | 'new'>();
  const [remove, setRemove] = useState<JobStage | 'application'>();
  return (
    <section
      className="grid min-w-0 max-w-[calc(100cqw-2rem)] gap-4 min-[56rem]:sticky min-[56rem]:left-4"
      aria-label={`Application history for ${application.position.title}`}
    >
      <PositionOverview
        position={application.position}
        showTitle={showPositionTitle}
      />
      <PositionNotes notes={application.position.notes} />
      <ol
        className="divide-y border-t border-border"
        aria-label="Application stages"
      >
        {application.stages.map((s) => (
          <li key={s.id} className="grid gap-2 py-4">
            <div className="flex flex-wrap items-start justify-between gap-2">
              <div className="grid gap-1">
                <h4 className="font-semibold break-words">{s.title}</h4>
                {s.date && (
                  <time
                    className="text-sm text-muted-foreground"
                    dateTime={s.date}
                  >
                    {formatCalendarDay(new Date(s.date))}
                  </time>
                )}
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
                  <IconDotsVertical size={18} aria-hidden="true" />
                </DropdownMenuTrigger>
                <DropdownMenuContent>
                  <DropdownMenuItem onClick={() => setStage(s)}>
                    <IconEdit size={17} aria-hidden="true" />
                    {s.initial ? 'Edit Applied Date' : 'Edit Stage'}
                  </DropdownMenuItem>
                  {!s.initial && (
                    <DropdownMenuItem
                      variant="destructive"
                      onClick={() => setRemove(s)}
                    >
                      <IconTrash size={17} aria-hidden="true" /> Remove Stage
                    </DropdownMenuItem>
                  )}
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
            {s.notes && <JobText text={s.notes} />}
          </li>
        ))}
      </ol>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <Button
          variant="destructive"
          size="sm"
          className="text-[color-mix(in_srgb,var(--destructive)_78%,var(--foreground))]"
          onClick={() => setRemove('application')}
        >
          Remove Application
        </Button>
        <Button
          variant="outline"
          size="sm"
          className="ml-auto"
          onClick={onEdit}
        >
          <IconEdit size={17} aria-hidden="true" /> Edit Application
        </Button>
        <Button size="sm" onClick={() => setStage('new')}>
          <IconPlus size={16} aria-hidden="true" />
          Add Stage
        </Button>
      </div>
      {stage && (
        <StageForm
          application={application}
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
            remove === 'application' ? 'Remove Application' : 'Remove Stage'
          }
          description={
            remove === 'application'
              ? 'This removes your application and all its private stage notes.'
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
