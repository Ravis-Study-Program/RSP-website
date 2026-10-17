import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Dialog,
  DialogTrigger,
  DialogContent,
  DialogTitle,
  DialogDescription,
  DialogClose,
} from '@/components/ui/dialog';
import { IconSearch, IconX } from '@tabler/icons-react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';

import { useWorkspace } from '@/auth/WorkspaceContext';

const accountCommands = [
  {
    label: 'Profile',
    description: 'Public profile and private account details',
    to: '/profile',
  },
  {
    label: 'Settings',
    description: 'Timezone, practice goals and security',
    to: '/settings',
  },
];

export function CommandPalette() {
  const { activeWorkspace } = useWorkspace();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const searchInputRef = useRef<HTMLInputElement>(null);
  const commands = useMemo(() => {
    const role = activeWorkspace?.role;
    const seasonBase = activeWorkspace?.seasonSlug
      ? `/seasons/${activeWorkspace.seasonSlug}`
      : '/seasons';
    const roleCommands =
      role === 'student'
        ? [
            {
              label: 'Season',
              description: 'Current season workspace',
              to: seasonBase,
            },
            {
              label: 'Practice',
              description: 'Problem attempt history',
              to: `${seasonBase}/practice`,
            },
            {
              label: 'Mock interviews',
              description: 'Received, given and available interviews',
              to: `${seasonBase}/mock-interviews`,
            },
            {
              label: 'People',
              description: 'Your season directory',
              to: `${seasonBase}/people`,
            },
          ]
        : role === 'mentor'
          ? [
              {
                label: 'My mentees',
                description: 'Assigned students and mentoring actions',
                to: `${seasonBase}/mentees`,
              },
              {
                label: 'Mock interviews',
                description: 'Received, given and available interviews',
                to: `${seasonBase}/mock-interviews`,
              },
              {
                label: 'People',
                description: 'Your season directory',
                to: `${seasonBase}/people`,
              },
            ]
          : role === 'coordinator'
            ? [
                {
                  label: 'Season',
                  description: 'Current season workspace',
                  to: seasonBase,
                },
                {
                  label: 'People',
                  description: 'Season roster',
                  to: `${seasonBase}/people`,
                },
                {
                  label: 'Mentor teams',
                  description: 'Mentorship assignments',
                  to: `${seasonBase}/mentor-teams`,
                },
                {
                  label: 'Mock interviews',
                  description: 'Season interview activity',
                  to: `${seasonBase}/mock-interviews`,
                },
                {
                  label: 'Season operations',
                  description: 'Weeks, enrollments and mentorships',
                  to: '/admin/enrollments',
                },
              ]
            : role === 'director' || role === 'system_admin'
              ? [
                  {
                    label: 'Seasons',
                    description: 'Current and completed programmes',
                    to: '/seasons',
                  },
                  {
                    label: 'Graduates',
                    description: 'Alumni directory',
                    to: '/graduates',
                  },
                  {
                    label: 'Practice',
                    description: 'Problem attempt history',
                    to: '/practice',
                  },
                  {
                    label: 'Administration',
                    description: 'Audited programme operations',
                    to: '/admin',
                  },
                ]
              : role === 'graduate'
                ? [
                    {
                      label: 'Practice',
                      description: 'Problem attempt history',
                      to: '/practice',
                    },
                    {
                      label: 'Mock interviews',
                      description: 'Received, given and available interviews',
                      to: '/mock-interviews',
                    },
                    {
                      label: 'Graduates',
                      description: 'Alumni directory',
                      to: '/graduates',
                    },
                  ]
                : role === 'former_member'
                  ? [
                      {
                        label: 'Practice',
                        description: 'Global practice history',
                        to: '/practice',
                      },
                      {
                        label: 'Mock interviews',
                        description: 'Global interview activity',
                        to: '/mock-interviews',
                      },
                    ]
                  : [];
    const dashboard = role
      ? [
          {
            label: 'Dashboard',
            description: 'Your current workspace summary',
            to: '/dashboard',
          },
        ]
      : [];
    return [...dashboard, ...accountCommands, ...roleCommands];
  }, [activeWorkspace]);
  const filtered = useMemo(
    () =>
      commands.filter((command) =>
        `${command.label} ${command.description}`
          .toLowerCase()
          .includes(query.toLowerCase()),
      ),
    [commands, query],
  );

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        setOpen((value) => !value);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) setQuery('');
      }}
    >
      <DialogTrigger
        aria-label="Search pages"
        render={
          <Button
            variant="ghost"
            className="gap-2 text-muted-foreground max-sm:size-10 max-sm:px-0 sm:border-border sm:bg-background"
          />
        }
      >
        <IconSearch size={16} aria-hidden="true" />
        <span className="hidden sm:inline">Search pages</span>
        <kbd className="ml-5 hidden rounded border px-1.5 text-xs sm:inline">
          ⌘ K
        </kbd>
      </DialogTrigger>
      <DialogContent showCloseButton={false} initialFocus={searchInputRef}>
        <div className="mb-4 flex items-start justify-between gap-4">
          <div>
            <DialogTitle>Quick navigation</DialogTitle>
            <DialogDescription>
              Search pages available in your current role.
            </DialogDescription>
          </div>
          <DialogClose
            render={<Button variant="ghost" size="icon" />}
            aria-label="Close quick navigation"
          >
            <IconX size={18} aria-hidden="true" />
          </DialogClose>
        </div>
        <div className="relative mx-auto w-full">
          <IconSearch
            size={18}
            aria-hidden="true"
            className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-muted-foreground"
          />
          <label className="sr-only" htmlFor="command-search">
            Search pages
          </label>
          <Input
            id="command-search"
            ref={searchInputRef}
            className="pl-9"
            autoComplete="off"
            placeholder="Search pages…"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
        </div>
        <nav aria-label="Quick navigation results" className="-mx-1">
          <ul className="m-0 grid list-none gap-1 p-0">
            {filtered.map((command) => (
              <li key={command.to}>
                <Link
                  className="block rounded-md px-3 py-2 text-sm hover:bg-muted focus-visible:bg-muted"
                  to={command.to}
                  onClick={() => setOpen(false)}
                >
                  <span>
                    <strong>{command.label}</strong>
                    <span className="mt-0.5 block text-sm font-normal text-muted-foreground">
                      {command.description}
                    </span>
                  </span>
                </Link>
              </li>
            ))}
          </ul>
          {!filtered.length ? (
            <p className="py-3 text-sm text-muted-foreground">
              No matching pages.
            </p>
          ) : null}
        </nav>
      </DialogContent>
    </Dialog>
  );
}
