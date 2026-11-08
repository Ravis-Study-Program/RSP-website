import { useState } from 'react';
import { Link } from 'react-router-dom';
import type { ColumnDef } from '@tanstack/react-table';
import {
  IconAlertTriangle,
  IconArrowUpRight,
  IconDots,
  IconPlus,
} from '@tabler/icons-react';
import { useJobActions, useJobReferrals } from '@/api/jobs';
import type { JobReferral } from '@/api/generated/models';
import { DataTable } from '@/components/DataTable';
import { NamedConfirmation } from '@/components/Dialogs';
import { ReferralForm } from '@/components/JobForms';
import { useJobsUser } from '@/components/JobsShell';
import { ReferralNotesHelp } from '@/components/ReferralNotesHelp';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

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
        Open to referring
      </Button>
      {open && <ReferralForm onClose={() => setOpen(false)} />}
    </>
  );
}

function ReferrerLink({ referral }: { referral: JobReferral }) {
  return (
    <Link
      className="inline-flex min-w-0 items-center gap-1.5 rounded-sm font-medium text-foreground hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
      to={`/people/${encodeURIComponent(referral.referrer.slug)}`}
    >
      <IconArrowUpRight size={16} className="shrink-0" aria-hidden="true" />
      <span>{referral.referrer.name}</span>
    </Link>
  );
}

function Notes({ notes }: { notes: string }) {
  return (
    <p className="whitespace-pre-wrap break-words text-sm leading-relaxed">
      {notes || '—'}
    </p>
  );
}

export function Referrals() {
  const user = useJobsUser();
  const referrals = useJobReferrals(user?.id);
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
            Edit listing
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => setRemove(referral)}>
            Stop referring
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    );
  }
  const columns: ColumnDef<JobReferral>[] = [
    {
      id: 'company',
      header: 'Company',
      accessorFn: (r) => r.company.name,
      meta: {
        width: 12,
        filterOptions: [
          ...new Set((referrals.data ?? []).map((r) => r.company.name)),
        ]
          .sort()
          .map((name) => ({ label: name, value: name })),
      },
    },
    {
      id: 'referrer',
      header: 'Name',
      accessorFn: (r) => r.referrer.name,
      cell: ({ row }) => <ReferrerLink referral={row.original} />,
      meta: { width: 14, wrap: true },
    },
    {
      id: 'Notes',
      accessorFn: (r) => r.notes,
      header: () => (
        <span className="inline-flex items-center gap-1">
          Notes
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
      meta: { width: 3, align: 'end' },
      cell: ({ row }) => actions(row.original),
    },
  ];
  return (
    <div className="grid gap-6">
      <section
        aria-labelledby="referral-rules"
        className="rounded-xl border border-destructive bg-destructive-soft p-5 text-destructive sm:p-6"
      >
        <div className="flex items-start gap-3">
          <IconAlertTriangle
            size={24}
            className="mt-0.5 shrink-0"
            aria-hidden="true"
          />
          <div className="grid gap-2">
            <h2 id="referral-rules" className="text-xl font-semibold">
              Read before contacting a referrer
            </h2>
            <p className="max-w-prose">
              Only message a referrer if you meet{' '}
              <strong>every requirement in their notes</strong>. Do not harass
              them or send repeated requests. Be respectful and understanding if
              they are unable or unwilling to refer you.
            </p>
            <p className="font-semibold">
              Students who do not follow these rules will be removed from the
              program.
            </p>
          </div>
        </div>
      </section>
      <DataTable
        ariaLabel="Referrals"
        data={referrals.data ?? []}
        columns={columns}
        getRowId={(referral) => referral.id}
        loading={referrals.isLoading}
        error={referrals.isError}
        onRetry={() => void referrals.refetch()}
        emptyTitle="No referrals yet"
        emptyMessage="Members who are open to referring will appear here."
        searchPlaceholder="Search companies, people or notes…"
        renderCard={({ original }) => (
          <div className="grid gap-3">
            <div className="flex items-center justify-between gap-3">
              <h3 className="font-semibold">{original.company.name}</h3>
              {actions(original)}
            </div>
            <ReferrerLink referral={original} />
            <div>
              <div className="mb-1 flex items-center gap-1 text-sm font-medium">
                Notes
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
          actionLabel="Stop referring"
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
