import type { ColumnDef } from '@tanstack/react-table';
import { StatusBadge } from '@/components/StatusBadge';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Button } from '@/components/ui/button';
import { Field, FieldLabel } from '@/components/ui/field';
import { IconLink, IconPlus, IconUsersGroup } from '@tabler/icons-react';
import { useState } from 'react';
import { useParams } from 'react-router-dom';

import { apiRequest } from '@/api/client';
import type { Mentorship, MentorshipCreate } from '@/api/generated/models';
import { demoMode, useSeasons, useSeasonPeople } from '@/api/queries';
import {
  Page,
  PageHeader,
  PageSection,
  PersonIdentity,
  usePageTitle,
} from '@/components/Common';
import { DataTable, ExpandRowButton } from '@/components/DataTable';
import { FormDialog } from '@/components/Dialogs';
import { InlineNotice, PageSkeleton } from '@/components/StatusViews';
import type { Person } from '@/types';

export function MentorTeamsPage() {
  usePageTitle('Mentor Teams');
  const { slug } = useParams();
  const seasons = useSeasons();
  const season = seasons.data?.items.find((item) => item.slug === slug);
  const query = useSeasonPeople(season?.id, season?.name);
  const [assigning, setAssigning] = useState<Person | null>(null);
  if (query.isLoading) return <PageSkeleton label="Loading mentor teams" />;
  const mentors =
    query.data?.items.filter((person) => person.roles.includes('mentor')) ?? [];
  const students =
    query.data?.items.filter((person) => person.roles.includes('student')) ??
    [];
  const unassignedCount = students.filter(
    (student) => student.status === 'unassigned',
  ).length;
  const unassigned = students.filter(
    (student) => student.status === 'unassigned',
  );
  // Clicking an unassigned student, or their link icon, assigns them a mentor.
  const assignButton = (student: Person) => (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      aria-label={`Assign a mentor to ${student.name}`}
      onClick={() => setAssigning(student)}
    >
      <IconLink size={18} aria-hidden="true" />
    </Button>
  );
  const unassignedColumns: ColumnDef<Person, any>[] = [
    {
      accessorKey: 'name',
      header: 'Student',
      cell: ({ row }) => (
        <PersonIdentity
          person={row.original}
          seasonId={season?.id}
          showRoles={false}
        />
      ),
    },
    {
      id: 'actions',
      header: 'Actions',
      meta: { width: 6, align: 'end' },
      enableSorting: false,
      enableHiding: false,
      cell: ({ row }) => assignButton(row.original),
    },
  ];
  const studentsOf = (mentor: Person) =>
    students.filter((student) => student.mentorshipMentorId === mentor.id);
  const studentList = (mentor: Person) => (
    <ul
      aria-label={`Students of ${mentor.name}`}
      className="m-0 grid list-none p-0"
    >
      {studentsOf(mentor).map((student) => (
        <li
          className="flex items-center justify-between gap-4 border-b border-border py-3 first:pt-0 last:border-b-0 last:pb-0"
          key={student.id}
        >
          <PersonIdentity
            person={student}
            seasonId={season?.id}
            showRoles={false}
          />
        </li>
      ))}
    </ul>
  );
  const columns: ColumnDef<Person, any>[] = [
    {
      id: 'expand',
      header: 'Details',
      meta: { width: 4.5 },
      enableSorting: false,
      enableHiding: false,
      cell: ({ row }) =>
        studentsOf(row.original).length ? (
          <ExpandRowButton
            row={row}
            subject={`students of ${row.original.name}`}
          />
        ) : null,
    },
    {
      accessorKey: 'name',
      header: 'Mentor',
      cell: ({ row }) => (
        <PersonIdentity
          person={row.original}
          seasonId={season?.id}
          showRoles={false}
        />
      ),
    },
    {
      id: 'students',
      header: 'Students',
      meta: { width: 8 },
      accessorFn: (mentor) => studentsOf(mentor).length,
    },
  ];
  return (
    <Page>
      <PageHeader
        title="Mentor Teams"
        titleHint="Admin"
        actions={
          <MentorshipDialog
            seasonId={season?.id}
            mentors={mentors}
            students={students}
            onCreated={() => void query.refetch()}
          />
        }
      />
      <div className="flex flex-col gap-6">
        {unassignedCount ? (
          <InlineNotice tone="warning">
            <IconUsersGroup size={19} aria-hidden="true" />{' '}
            {unassignedCount === 1
              ? '1 student needs a mentor.'
              : `${unassignedCount} students need a mentor.`}
          </InlineNotice>
        ) : null}
        <DataTable
          ariaLabel="Mentor teams"
          data={mentors}
          columns={columns}
          getRowId={(mentor) => mentor.id}
          rowCanExpand={(mentor) => studentsOf(mentor).length > 0}
          renderExpanded={(row) => studentList(row.original)}
          loading={query.isLoading}
          error={query.isError}
          onRetry={() => void query.refetch()}
          emptyTitle="No Mentors in This Season"
          emptyMessage="Mentors appear here once they are enrolled."
          renderCard={(row) => (
            <div className="grid gap-3">
              <div className="flex items-center justify-between gap-3">
                <PersonIdentity
                  person={row.original}
                  seasonId={season?.id}
                  showRoles={false}
                />
                <StatusBadge>{studentsOf(row.original).length}</StatusBadge>
              </div>
              {studentsOf(row.original).length
                ? studentList(row.original)
                : null}
            </div>
          )}
        />
        <PageSection
          titleId="unassigned-students-title"
          title="Unassigned Students"
        >
          <DataTable
            ariaLabel="Unassigned students"
            data={unassigned}
            columns={unassignedColumns}
            getRowId={(student) => student.id}
            onRowClick={setAssigning}
            emptyTitle="All Students Assigned"
            emptyMessage="Every active student has a mentor team."
            renderCard={(row) => (
              <div className="flex items-center justify-between gap-3">
                <PersonIdentity
                  person={row.original}
                  seasonId={season?.id}
                  showRoles={false}
                />
                {assignButton(row.original)}
              </div>
            )}
          />
        </PageSection>
        {assigning ? (
          <MentorshipDialog
            seasonId={season?.id}
            mentors={mentors}
            students={students}
            studentId={assigning.id}
            open
            onOpenChange={(next) => {
              if (!next) setAssigning(null);
            }}
            onCreated={() => void query.refetch()}
          />
        ) : null}
      </div>
    </Page>
  );
}

function MentorshipDialog({
  seasonId,
  mentors,
  students,
  onCreated,
  studentId: presetStudentId = '',
  open: controlledOpen,
  onOpenChange,
}: {
  seasonId?: string;
  mentors: Array<{ id: string; name: string }>;
  students: Array<{ id: string; name: string; status: string | null }>;
  onCreated: () => void;
  /** Opened from an unassigned student's row, starting with that student. */
  studentId?: string;
  /** When given, the dialog is opened from outside and shows no trigger. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const controlled = controlledOpen !== undefined;
  const open = controlled ? controlledOpen : uncontrolledOpen;
  const setOpen = (next: boolean) => {
    if (!controlled) setUncontrolledOpen(next);
    onOpenChange?.(next);
  };
  const [mentorId, setMentorId] = useState('');
  const [studentId, setStudentId] = useState(presetStudentId);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const dirty = mentorId !== '' || studentId !== presetStudentId;
  const requestOpenChange = (next: boolean) => {
    if (
      !next &&
      dirty &&
      !window.confirm('Discard unsaved mentorship changes?')
    )
      return;
    if (next) {
      setMentorId('');
      setStudentId(presetStudentId);
      setError('');
    }
    setOpen(next);
  };
  const create = async () => {
    if (!seasonId || !mentorId || !studentId) return;
    setPending(true);
    setError('');
    try {
      if (!demoMode) {
        const body: MentorshipCreate = {
          mentorUserId: mentorId,
          studentUserId: studentId,
        };
        await apiRequest<Mentorship>(
          `/seasons/${encodeURIComponent(seasonId)}/mentorships`,
          { method: 'POST', body: JSON.stringify(body) },
        );
      }
      onCreated();
      setMentorId('');
      setStudentId('');
      setOpen(false);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'Unable to create this mentorship.',
      );
    } finally {
      setPending(false);
    }
  };
  const mentorItems = [
    { label: 'Choose a mentor', value: '' },
    ...mentors.map((mentor) => ({ label: mentor.name, value: mentor.id })),
  ];
  const studentItems = [
    { label: 'Choose a student', value: '' },
    ...students
      .filter((student) => student.status === 'unassigned')
      .map((student) => ({ label: student.name, value: student.id })),
  ];
  return (
    <FormDialog
      title="Add Mentorship"
      open={open}
      onOpenChange={requestOpenChange}
      trigger={
        controlled ? undefined : (
          <>
            <IconPlus size={18} aria-hidden="true" /> Assign student
          </>
        )
      }
    >
      <div className="grid gap-4">
        {error ? (
          <p className="text-sm text-destructive" role="alert">
            {error}
          </p>
        ) : null}
        <Field>
          <FieldLabel htmlFor={'student-new'}>Student</FieldLabel>
          <Select
            items={studentItems}
            value={studentId}
            onValueChange={(value) => setStudentId(value ?? '')}
          >
            <SelectTrigger id={'student-new'} className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {studentItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
        <Field>
          <FieldLabel htmlFor={'mentor-new'}>Mentor</FieldLabel>
          <Select
            items={mentorItems}
            value={mentorId}
            onValueChange={(value) => setMentorId(value ?? '')}
          >
            <SelectTrigger id={'mentor-new'} className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {mentorItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>
        </Field>
        <div className="flex flex-wrap justify-end gap-2">
          <Button
            variant="outline"
            type="button"
            disabled={pending}
            onClick={() => requestOpenChange(false)}
          >
            Cancel
          </Button>
          <Button
            type="button"
            disabled={
              pending || !mentorId || !studentId || (!demoMode && !seasonId)
            }
            onClick={() => void create()}
          >
            {pending ? 'Adding…' : 'Add mentorship'}
          </Button>
        </div>
      </div>
    </FormDialog>
  );
}
