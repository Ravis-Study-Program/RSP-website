import { useId, useRef, useState, type ReactNode } from 'react';
import { useSearchParams } from 'react-router-dom';
import type { ColumnDef } from '@tanstack/react-table';
import { Toast } from '@base-ui/react/toast';
import {
  IconAlertCircle,
  IconAlertTriangle,
  IconPlus,
  IconX,
} from '@tabler/icons-react';
import {
  useJobActions,
  useJobCompanies,
  useJobPositions,
  useJobReferralBoard,
  useJobReferralCandidates,
  useJobReferralCounts,
} from '@/api/jobs';
import type {
  JobCompany,
  JobPosition,
  JobReferralBoard,
  JobReferralCandidate,
  JobReferralProfile,
} from '@/api/generated/models';
import { DataTable } from '@/components/DataTable';
import { FormDialog, NamedConfirmation } from '@/components/Dialogs';
import { ExternalLink } from '@/components/ExternalLink';
import { JobForm, JobSelect } from '@/components/JobForms';
import { ErrorState, PageSkeleton } from '@/components/StatusViews';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
} from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover';
import { Textarea } from '@/components/ui/textarea';
import { isPublicResumeURL, jobTypes, maxReferralRequests } from '@/jobs';

const referralGuidelineCopy = {
  referrals: {
    title: 'Request Referral Guidelines',
    instructions:
      'Request referrals for open roles you’re interested in. Withdraw requests you no longer need.',
    privacy:
      'Your resume link and reason are shared with members who opt in to refer at the companies you select. Referrers may contact you on Discord.',
  },
  'refer-candidates': {
    title: 'Referrer Guidelines',
    instructions:
      'Opt in only at companies you work for. Contact candidates you choose on Discord.',
    privacy:
      'Use candidates’ information only for the referrals they requested.',
  },
};

type ReferralGuidelinePage = keyof typeof referralGuidelineCopy;

export function ReferralGuidelines({
  userId,
  page,
  children,
}: {
  userId?: string;
  page?: ReferralGuidelinePage;
  children: (link: ReactNode, notice: ReactNode) => ReactNode;
}) {
  const id = useId();
  const storagePrefix = userId && `rsp-referral-guidelines-ack:v3:${userId}`;
  const [acknowledged, setAcknowledged] = useState<
    Partial<Record<ReferralGuidelinePage, boolean>>
  >(() => {
    if (!storagePrefix) return {};
    try {
      return {
        referrals: localStorage.getItem(`${storagePrefix}:referrals`) === '1',
        'refer-candidates':
          localStorage.getItem(`${storagePrefix}:refer-candidates`) === '1',
      };
    } catch {
      return {};
    }
  });
  const restoreFocus = useRef(false);

  if (!page || !storagePrefix) return children(null, null);

  const acknowledge = () => {
    try {
      localStorage.setItem(`${storagePrefix}:${page}`, '1');
    } catch {
      // The notice can still be dismissed when browser storage is unavailable.
    }
    restoreFocus.current = true;
    setAcknowledged((previous) => ({ ...previous, [page]: true }));
  };

  const { title, instructions, privacy } = referralGuidelineCopy[page];
  const rules = (
    <>
      <p>{instructions}</p>
      <p className="font-semibold">{privacy}</p>
    </>
  );

  if (acknowledged[page])
    return children(
      <Popover key={page}>
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

type ReferralLayout = (actions: ReactNode, content: ReactNode) => ReactNode;

export function Referrals({
  userId,
  refer,
  canRefer,
  toolbarLead,
  layout,
}: {
  userId: string;
  refer: boolean;
  canRefer: boolean;
  toolbarLead: ReactNode;
  layout: ReferralLayout;
}) {
  const board = useJobReferralBoard(userId);
  if (!board.data)
    return layout(
      null,
      <div className="grid gap-4">
        {toolbarLead}
        {board.isError ? (
          <ErrorState onRetry={() => void board.refetch()} />
        ) : (
          <PageSkeleton label="Loading referrals" />
        )}
      </div>,
    );
  return refer ? (
    <ReferrerQueue
      userId={userId}
      canRefer={canRefer}
      companies={board.data.companies}
      toolbarLead={toolbarLead}
      layout={layout}
    />
  ) : (
    <Toast.Provider limit={1} timeout={4500}>
      <ReferralRequests
        userId={userId}
        board={board.data}
        toolbarLead={toolbarLead}
        layout={layout}
      />
    </Toast.Provider>
  );
}

function ReferralRequests({
  userId,
  board,
  toolbarLead,
  layout,
}: {
  userId: string;
  board: JobReferralBoard;
  toolbarLead: ReactNode;
  layout: ReferralLayout;
}) {
  const [params, setParams] = useSearchParams();
  const [editing, setEditing] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const { add, close, toasts } = Toast.useToastManager();
  const requestErrorToast = 'referral-request-error';
  const positions = useJobPositions();
  const counts = useJobReferralCounts(userId);
  const save = useJobActions(userId);
  const companyId = params.get('company') ?? 'all';
  const requested = new Set(board.requests);
  const open = (positions.data ?? []).filter(
    (position) => position.status === 'open',
  );
  const companies = new Map(
    open.map(({ company }) => [company.id, company.name]),
  );
  const filtered = open
    .filter(
      (position) => companyId === 'all' || position.company.id === companyId,
    )
    .sort((a, b) => b.createdAt.localeCompare(a.createdAt));
  const countByCompany = new Map(
    counts.data?.map(({ companyId, count }) => [companyId, count]),
  );

  function showRequestError(title: string, description: string) {
    add({ id: requestErrorToast, title, description, priority: 'high' });
  }

  async function toggle(position: JobPosition, wanted: boolean) {
    setError('');
    if (wanted && !board.profile.resumeUrl) {
      showRequestError(
        'Create a referral profile first',
        'Choose Create Referral Profile to add your resume and reason.',
      );
      return;
    }
    if (wanted && requested.size >= maxReferralRequests) {
      showRequestError(
        'Referral request limit reached',
        `You can request up to ${maxReferralRequests} roles. Withdraw one to add another.`,
      );
      return;
    }
    close(requestErrorToast);
    setBusy(true);
    try {
      await save(
        `/jobs/referrals/requests/${encodeURIComponent(position.id)}`,
        wanted ? 'PUT' : 'DELETE',
      );
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : 'Could not update the request. Try again.',
      );
    } finally {
      setBusy(false);
    }
  }

  function requestControl(position: JobPosition) {
    const selected = requested.has(position.id);
    return (
      <Checkbox
        aria-label={`Request referral for ${position.title} at ${position.company.name}`}
        className="border-muted-foreground"
        checked={selected}
        disabled={busy}
        onCheckedChange={(wanted) => void toggle(position, wanted === true)}
      />
    );
  }
  const count = (position: JobPosition) =>
    countByCompany.get(position.company.id) ?? 0;
  const columns: ColumnDef<JobPosition>[] = [
    {
      id: 'company',
      accessorFn: (p) => p.company.name,
      header: 'Company',
      cell: ({ row }) => <ReferralCompanyLink position={row.original} />,
      meta: { width: 10 },
    },
    { accessorKey: 'title', header: 'Job Title', meta: { wrap: true } },
    {
      id: 'type',
      accessorFn: (p) => jobTypes.find((t) => t.value === p.jobType)?.label,
      header: 'Job Type',
      meta: { width: 6, filterOptions: jobTypes },
    },
    {
      accessorKey: 'eligibility',
      header: 'Eligibility Requirements',
      cell: ({ row }) => row.original.eligibility || '—',
      meta: { width: 11, wrap: true },
    },
    {
      id: 'locations',
      accessorFn: (p) => p.locations.join(', '),
      header: 'Locations',
      meta: { width: 9, wrap: true },
    },
    {
      id: 'referrers',
      accessorFn: count,
      header: 'Referrers',
      enableColumnFilter: false,
      meta: { width: 5.5 },
    },
    {
      id: 'requested',
      accessorFn: (p) => (requested.has(p.id) ? 1 : 0),
      header: 'Applied',
      cell: ({ row }) => requestControl(row.original),
      sortDescFirst: true,
      enableColumnFilter: false,
      enableHiding: false,
      meta: { width: 5.5 },
    },
  ];

  return layout(
    <>
      <span
        role="status"
        className="text-sm text-muted-foreground tabular-nums"
      >
        {requested.size} of {maxReferralRequests} requested
      </span>
      <Button
        type="button"
        variant="outline"
        onClick={() => {
          close(requestErrorToast);
          setEditing(true);
        }}
      >
        {board.profile.resumeUrl
          ? 'Edit Referral Profile'
          : 'Create Referral Profile'}
      </Button>
    </>,
    <div className="grid gap-4">
      <DataTable
        toolbarLead={toolbarLead}
        ariaLabel="Open positions for referrals"
        data={filtered}
        columns={columns}
        getRowId={(p) => p.id}
        loading={positions.isPending || counts.isPending}
        error={positions.isError || counts.isError}
        onRetry={() => {
          void positions.refetch();
          void counts.refetch();
        }}
        emptyTitle="No Open Positions"
        emptyMessage="Try another company."
        searchPlaceholder="Search positions…"
        toolbarStart={
          <>
            <JobSelect
              label="Company"
              compact
              value={companyId}
              options={[
                { value: 'all', label: 'All Companies' },
                ...[...companies]
                  .sort((a, b) => a[1].localeCompare(b[1]))
                  .map(([value, label]) => ({ value, label })),
                ...(companyId !== 'all' && !companies.has(companyId)
                  ? [{ value: companyId, label: 'Selected Company' }]
                  : []),
              ]}
              onChange={(value) =>
                setParams(
                  (current) => {
                    const next = new URLSearchParams(current);
                    if (value === 'all') next.delete('company');
                    else next.set('company', value);
                    return next;
                  },
                  { replace: true },
                )
              }
            />
            {error && <FieldError className="basis-full">{error}</FieldError>}
          </>
        }
        renderCard={({ original: position }) => (
          <div className="grid gap-2">
            <ReferralCompanyLink position={position} />
            <h3 className="font-semibold break-words">{position.title}</h3>
            <p className="text-sm text-muted-foreground">
              {jobTypes.find((t) => t.value === position.jobType)?.label} ·{' '}
              {position.locations.join(', ')}
            </p>
            {position.eligibility && (
              <p className="text-sm">{position.eligibility}</p>
            )}
            <div className="flex items-center justify-between gap-3 text-sm">
              <span>
                {count(position)}{' '}
                {count(position) === 1 ? 'referrer' : 'referrers'}
              </span>
              <div className="flex items-center gap-2">
                Request {requestControl(position)}
              </div>
            </div>
          </div>
        )}
      />
      {editing && (
        <ReferralProfileForm
          userId={userId}
          profile={board.profile}
          onClose={() => setEditing(false)}
        />
      )}
      <Toast.Portal>
        <Toast.Viewport
          aria-label="Referral notifications"
          className="pointer-events-none fixed right-4 bottom-[calc(5rem+env(safe-area-inset-bottom))] z-70 w-96 max-w-[calc(100vw-2rem)] min-[56rem]:bottom-4"
        >
          {toasts.map((toast) => (
            <Toast.Root
              key={toast.id}
              toast={toast}
              className="pointer-events-auto rounded-lg border border-destructive/40 bg-popover p-3 text-popover-foreground shadow-lg motion-safe:transition-[translate,opacity] motion-safe:duration-200 data-[ending-style]:translate-y-2 data-[ending-style]:opacity-0 data-[starting-style]:translate-y-2 data-[starting-style]:opacity-0"
            >
              <Toast.Content className="flex items-start gap-3">
                <IconAlertCircle
                  size={18}
                  className="mt-0.5 shrink-0 text-destructive"
                  aria-hidden="true"
                />
                <div className="grid flex-1 gap-1">
                  <Toast.Title className="text-sm font-semibold" />
                  <Toast.Description className="text-sm text-muted-foreground" />
                </div>
                <Toast.Close
                  render={
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label="Dismiss notification"
                    />
                  }
                >
                  <IconX aria-hidden="true" />
                </Toast.Close>
              </Toast.Content>
            </Toast.Root>
          ))}
        </Toast.Viewport>
      </Toast.Portal>
    </div>,
  );
}

function ReferralCompanyLink({ position }: { position: JobPosition }) {
  return (
    <ExternalLink
      href={position.applicationUrl}
      aria-label={`Open ${position.title} at ${position.company.name}`}
      className="font-medium"
    >
      <span className="min-w-0 truncate">{position.company.name}</span>
    </ExternalLink>
  );
}

function ReferralProfileForm({
  userId,
  profile,
  onClose,
}: {
  userId: string;
  profile: JobReferralProfile;
  onClose: () => void;
}) {
  const id = useId();
  const [reason, setReason] = useState(profile.reason);
  const [resumeUrl, setResumeUrl] = useState(profile.resumeUrl);
  const [remove, setRemove] = useState(false);
  const validResumeUrl = isPublicResumeURL(resumeUrl);
  const save = useJobActions(userId);
  return (
    <FormDialog
      title={
        profile.resumeUrl ? 'Edit Referral Profile' : 'Create Referral Profile'
      }
      description="Your profile is shared with RSP members who say they work at companies where you request a referral. They may contact you on Discord."
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <JobForm
        onClose={onClose}
        label="Save Profile"
        onSave={async () => {
          if (!validResumeUrl)
            throw new Error('Enter a public link to your resume.');
          await save('/jobs/referrals/profile', 'PUT', {
            reason,
            resumeUrl: resumeUrl.trim(),
          });
        }}
      >
        <Field>
          <FieldLabel htmlFor={`${id}-reason`}>
            Why I’d Be a Good Referral
          </FieldLabel>
          <Textarea
            id={`${id}-reason`}
            required
            maxLength={4000}
            rows={5}
            value={reason}
            onChange={(event) => setReason(event.target.value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor={`${id}-resume`}>Resume Link</FieldLabel>
          <Input
            id={`${id}-resume`}
            type="url"
            required
            maxLength={2048}
            value={resumeUrl}
            onChange={(event) => setResumeUrl(event.target.value)}
            aria-describedby={`${id}-resume-help`}
          />
          <FieldDescription id={`${id}-resume-help`}>
            Provide a public link to your resume (e.g. Google Drive). Make sure
            it opens without signing in.
          </FieldDescription>
          {validResumeUrl && (
            <ExternalLink href={resumeUrl.trim()} className="self-start">
              View Resume
            </ExternalLink>
          )}
        </Field>
        {profile.resumeUrl && (
          <Button
            type="button"
            variant="destructive"
            className="justify-self-start"
            onClick={() => setRemove(true)}
          >
            Delete Profile
          </Button>
        )}
      </JobForm>
      {remove && (
        <NamedConfirmation
          open
          name="Referral Profile"
          title="Delete your referral profile?"
          actionLabel="Delete Profile"
          requireTyping={false}
          description="Your saved resume link and reason will be removed from RSP, and all your referral requests withdrawn."
          onOpenChange={setRemove}
          onConfirm={async () => {
            await save('/jobs/referrals/profile', 'DELETE');
            onClose();
          }}
        />
      )}
    </FormDialog>
  );
}

function ReferrerQueue({
  userId,
  canRefer,
  companies,
  toolbarLead,
  layout,
}: {
  userId: string;
  canRefer: boolean;
  companies: JobCompany[];
  toolbarLead: ReactNode;
  layout: ReferralLayout;
}) {
  const [selectedCompany, setSelectedCompany] = useState<string>();
  const company = canRefer
    ? (companies.find((option) => option.id === selectedCompany) ??
      companies[0])
    : undefined;
  const candidates = useJobReferralCandidates(userId, company?.id);
  const save = useJobActions(userId);
  const [adding, setAdding] = useState(false);
  const [stopping, setStopping] = useState<JobCompany>();
  const [viewingId, setViewingId] = useState<string>();
  const viewing = candidates.data?.find(({ id }) => id === viewingId);
  const columns: ColumnDef<JobReferralCandidate>[] = [
    {
      accessorKey: 'name',
      header: 'Candidate',
      meta: { width: 20, wrap: true },
    },
    {
      id: 'roles',
      accessorFn: (candidate) =>
        candidate.positions.map((p) => p.title).join(', '),
      header: 'Requested Roles',
      cell: ({ row }) => <RequestedRoles candidate={row.original} />,
      meta: { wrap: true },
    },
    {
      id: 'profile',
      header: 'Profile',
      cell: ({ row }) => (
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => setViewingId(row.original.id)}
        >
          View Profile<span className="sr-only"> for {row.original.name}</span>
        </Button>
      ),
      enableSorting: false,
      enableColumnFilter: false,
      meta: { width: 13, align: 'end' },
    },
  ];
  const people = [...(candidates.data ?? [])].sort((a, b) =>
    a.name.localeCompare(b.name),
  );
  return layout(
    canRefer && (
      <Button type="button" onClick={() => setAdding(true)}>
        <IconPlus size={16} aria-hidden="true" /> Open to Referring
      </Button>
    ),
    <>
      <DataTable
        key={company?.id}
        toolbarLead={toolbarLead}
        ariaLabel="Referral candidates"
        data={people}
        columns={columns}
        getRowId={(candidate) => candidate.id}
        loading={Boolean(company && candidates.isPending)}
        error={Boolean(company && candidates.isError)}
        onRetry={() => void candidates.refetch()}
        emptyTitle={
          !canRefer
            ? 'Referring Is Available to RSP Graduates'
            : company
              ? 'No Requests at This Company'
              : 'Choose a Company to Start Referring'
        }
        emptyMessage={
          !canRefer
            ? ''
            : company
              ? 'Candidates will appear when they request a referral for an open position here.'
              : 'Select a company you work for to see its referral requests.'
        }
        searchPlaceholder="Search candidates or roles…"
        toolbarStart={
          <>
            {company && (
              <JobSelect
                label="Referring At"
                compact
                value={company.id}
                options={companies.map(({ id, name }) => ({
                  value: id,
                  label: name,
                }))}
                onChange={(value) => {
                  setSelectedCompany(value);
                  setViewingId(undefined);
                }}
              />
            )}
            {company && (
              <Button
                type="button"
                variant="outline"
                onClick={() => setStopping(company)}
              >
                Stop Referring
              </Button>
            )}
          </>
        }
        renderCard={({ original }) => (
          <div className="grid gap-2">
            <h3 className="font-semibold break-words">{original.name}</h3>
            <RequestedRoles candidate={original} />
            <Button
              type="button"
              variant="outline"
              className="justify-self-start"
              onClick={() => setViewingId(original.id)}
            >
              View Profile<span className="sr-only"> for {original.name}</span>
            </Button>
          </div>
        )}
      />
      {canRefer && adding && (
        <OfferReferralForm
          userId={userId}
          activeCompanies={companies}
          onClose={() => setAdding(false)}
          onSelect={setSelectedCompany}
        />
      )}
      {stopping && (
        <NamedConfirmation
          open
          name={stopping.name}
          title={`Stop referring at ${stopping.name}?`}
          actionLabel="Stop Referring"
          requireTyping={false}
          description="Candidates at this company will no longer appear in your queue."
          onOpenChange={(open) => {
            if (!open) setStopping(undefined);
          }}
          onConfirm={async () => {
            await save(
              `/jobs/referrals/offers/${encodeURIComponent(stopping.id)}`,
              'DELETE',
            );
            setViewingId(undefined);
          }}
        />
      )}
      {viewing && company && (
        <CandidateProfile
          company={company}
          candidate={viewing}
          onClose={() => setViewingId(undefined)}
        />
      )}
    </>,
  );
}

function OfferReferralForm({
  userId,
  activeCompanies,
  onClose,
  onSelect,
}: {
  userId: string;
  activeCompanies: JobCompany[];
  onClose: () => void;
  onSelect: (id: string) => void;
}) {
  const companies = useJobCompanies();
  const save = useJobActions(userId);
  const [selected, setSelected] = useState('choose');
  const options = (companies.data ?? []).filter(
    ({ id }) => !activeCompanies.some((active) => active.id === id),
  );
  return (
    <FormDialog
      title="Open to Referring"
      description="Choose a company you work for. You’ll see RSP candidates requesting referrals to its open roles."
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <JobForm
        label="Open to Referring"
        onClose={onClose}
        onSave={async () => {
          if (!options.some((option) => option.id === selected))
            throw new Error('Choose a company.');
          await save(
            `/jobs/referrals/offers/${encodeURIComponent(selected)}`,
            'PUT',
          );
          onSelect(selected);
        }}
      >
        <JobSelect
          label="Company"
          value={selected}
          options={[
            { value: 'choose', label: 'Choose a company' },
            ...options.map(({ id, name }) => ({ value: id, label: name })),
          ]}
          onChange={setSelected}
        />
        {companies.isError && (
          <FieldError>Could not load companies. Please try again.</FieldError>
        )}
      </JobForm>
    </FormDialog>
  );
}

function RequestedRoles({ candidate }: { candidate: JobReferralCandidate }) {
  return (
    <ul className="grid gap-1 text-sm">
      {candidate.positions.map((position) => (
        <li key={position.id} className="break-words">
          {position.title}
        </li>
      ))}
    </ul>
  );
}

function CandidateProfile({
  company,
  candidate,
  onClose,
}: {
  company: JobCompany;
  candidate: JobReferralCandidate;
  onClose: () => void;
}) {
  return (
    <FormDialog
      title={candidate.name}
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      contentClassName="sm:max-w-2xl"
    >
      <section className="grid gap-2">
        <h3 className="font-semibold">Requested Roles at {company.name}</h3>
        <RequestedRoles candidate={candidate} />
      </section>
      <section className="grid gap-2">
        <h3 className="font-semibold">Why I’d Be a Good Referral</h3>
        <p className="text-sm whitespace-pre-wrap break-words">
          {candidate.reason}
        </p>
      </section>
      <p className="text-sm text-muted-foreground">
        Search for {candidate.name} on Discord to reach out.
      </p>
      <div className="grid min-w-0 gap-1">
        <ExternalLink href={candidate.resumeUrl} className="justify-self-start">
          View Resume
        </ExternalLink>
        <p className="text-sm break-all text-muted-foreground select-all">
          {candidate.resumeUrl}
        </p>
      </div>
    </FormDialog>
  );
}
