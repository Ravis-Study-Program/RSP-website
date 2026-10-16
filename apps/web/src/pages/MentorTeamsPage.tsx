import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Button, buttonVariants } from '@/components/ui/button';
import { Field, FieldLabel } from '@/components/ui/field';
import { IconArrowRight, IconPlus, IconUsersGroup } from '@tabler/icons-react';
import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';

import { apiRequest } from '@/api/client';
import type { Mentorship, MentorshipCreate } from '@/api/generated/models';
import { demoMode, useSeasons, useSeasonPeople } from '@/api/queries';
import { PageHeader, PersonIdentity, usePageTitle } from '@/components/Common';
import { FormDialog } from '@/components/Dialogs';
import {
  EmptyState,
  InlineNotice,
  PageSkeleton,
} from '@/components/StatusViews';

export function MentorTeamsPage() {
  usePageTitle('Mentor teams');
  const { slug } = useParams();
  const seasons = useSeasons();
  const season = seasons.data?.items.find((item) => item.slug === slug);
  const query = useSeasonPeople(season?.id, season?.name);
  if (query.isLoading) return <PageSkeleton label="Loading mentor teams" />;
  const mentors =
    query.data?.items.filter((person) => person.roles.includes('mentor')) ?? [];
  const students =
    query.data?.items.filter((person) => person.roles.includes('student')) ??
    [];
  return (
    <div className="mx-auto w-full max-w-[92rem]">
      <PageHeader
        eyebrow="Season operations"
        title="Mentor teams"
        description="See assignments at a glance, find unassigned students and balance each mentor's team."
        actions={
          <MentorshipDialog
            seasonId={season?.id}
            mentors={mentors}
            students={students}
            onCreated={() => void query.refetch()}
          />
        }
      />
      {students.some((student) => student.status === 'unassigned') ? (
        <InlineNotice tone="warning">
          <IconUsersGroup size={19} aria-hidden="true" />{' '}
          {students.filter((student) => student.status === 'unassigned').length}{' '}
          student needs a mentor assignment.
        </InlineNotice>
      ) : null}
      <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,18rem),1fr))] gap-4">
        {mentors.map((mentor) => (
          <article key={mentor.id}>
            <Card className="gap-4 p-5">
              <PersonIdentity
                person={mentor}
                seasonId={season?.id}
                privateView
              />
              <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
                <h2 className="text-base font-semibold tracking-tight">
                  Assigned students
                </h2>
                <Badge variant="secondary">
                  {
                    students.filter(
                      (student) => student.mentorshipMentorId === mentor.id,
                    ).length
                  }
                </Badge>
              </div>
              <ul className="m-0 grid list-none gap-3 p-0">
                {students
                  .filter((student) => student.mentorshipMentorId === mentor.id)
                  .map((student) => (
                    <li
                      className="flex items-center justify-between gap-4 border-b border-border py-3 last:border-b-0"
                      key={student.id}
                    >
                      <PersonIdentity person={student} seasonId={season?.id} />
                      <Link
                        className={buttonVariants({
                          variant: 'ghost',
                          size: 'icon',
                        })}
                        to={`/people/${student.slug}?seasonId=${encodeURIComponent(season!.id)}`}
                        aria-label={`Open ${student.name}`}
                      >
                        <IconArrowRight size={18} aria-hidden="true" />
                      </Link>
                    </li>
                  ))}
              </ul>
            </Card>
          </article>
        ))}
      </div>
      <section className="mt-6">
        <div className="mb-3 flex flex-wrap items-end justify-between gap-x-4 gap-y-1">
          <h2 className="text-xl font-semibold tracking-tight">
            Unassigned students
          </h2>
        </div>
        {students.some((student) => student.status === 'unassigned') ? (
          <Card className="gap-4 p-5">
            <ul className="m-0 grid list-none gap-3 p-0">
              {students
                .filter((student) => student.status === 'unassigned')
                .map((student) => (
                  <li
                    className="flex items-center justify-between gap-4 border-b border-border py-3 last:border-b-0"
                    key={student.id}
                  >
                    <PersonIdentity
                      person={student}
                      seasonId={season?.id}
                      privateView
                    />
                    <MentorshipDialog
                      seasonId={season?.id}
                      mentors={mentors}
                      students={students}
                      initialStudentId={student.id}
                      onCreated={() => void query.refetch()}
                    />
                  </li>
                ))}
            </ul>
          </Card>
        ) : (
          <EmptyState
            title="All students assigned"
            message="Every active student has a mentor team."
          />
        )}
      </section>
    </div>
  );
}

function MentorshipDialog({
  seasonId,
  mentors,
  students,
  initialStudentId = '',
  onCreated,
}: {
  seasonId?: string;
  mentors: Array<{ id: string; name: string }>;
  students: Array<{ id: string; name: string; status: string | null }>;
  initialStudentId?: string;
  onCreated: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [mentorId, setMentorId] = useState('');
  const [studentId, setStudentId] = useState(initialStudentId);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const dirty = mentorId !== '' || studentId !== initialStudentId;
  const requestOpenChange = (next: boolean) => {
    if (
      !next &&
      dirty &&
      !window.confirm('Discard unsaved mentorship changes?')
    )
      return;
    if (next) {
      setMentorId('');
      setStudentId(initialStudentId);
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
      setStudentId(initialStudentId);
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
      title="Create mentorship"
      description="Choose one active mentor and one active student in this season."
      open={open}
      onOpenChange={requestOpenChange}
      trigger={
        <>
          <IconPlus size={18} aria-hidden="true" />{' '}
          {initialStudentId ? 'Assign mentor' : 'Assign student'}
        </>
      }
    >
      <div className="grid gap-4">
        <InlineNotice>
          Assignment validates season consistency and prevents duplicate active
          mentorships.
        </InlineNotice>
        {error ? (
          <p className="text-sm text-destructive" role="alert">
            {error}
          </p>
        ) : null}
        <Field>
          <FieldLabel htmlFor={`mentor-${initialStudentId || 'new'}`}>
            Mentor
          </FieldLabel>
          <Select
            items={mentorItems}
            value={mentorId}
            onValueChange={(value) => setMentorId(value ?? '')}
          >
            <SelectTrigger
              id={`mentor-${initialStudentId || 'new'}`}
              className="w-full"
            >
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
        <Field>
          <FieldLabel htmlFor={`student-${initialStudentId || 'new'}`}>
            Student
          </FieldLabel>
          <Select
            items={studentItems}
            value={studentId}
            disabled={Boolean(initialStudentId)}
            onValueChange={(value) => setStudentId(value ?? '')}
          >
            <SelectTrigger
              id={`student-${initialStudentId || 'new'}`}
              className="w-full"
            >
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
            {pending ? 'Assigning…' : 'Create mentorship'}
          </Button>
        </div>
      </div>
    </FormDialog>
  );
}
