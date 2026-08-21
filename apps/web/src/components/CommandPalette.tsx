import { Dialog } from '@base-ui/react/dialog';
import { IconSearch, IconX } from '@tabler/icons-react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';

import { useWorkspace } from '@/auth/WorkspaceContext';
import styles from '@/styles/App.module.css';

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
              description: 'Recommendation and attempt history',
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
                    description: 'Recommendation and attempt history',
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
                      description: 'Recommendation and attempt history',
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
    <Dialog.Root
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) setQuery('');
      }}
    >
      <Dialog.Portal>
        <Dialog.Backdrop className={styles.dialogBackdrop} />
        <Dialog.Viewport className={styles.dialogViewport}>
          <Dialog.Popup
            className={styles.dialogPopup}
            initialFocus={searchInputRef}
          >
            <div className={styles.dialogHeader}>
              <div>
                <Dialog.Title className={styles.dialogTitle}>
                  Quick navigation
                </Dialog.Title>
                <Dialog.Description className={styles.dialogDescription}>
                  Search pages available in your current role.
                </Dialog.Description>
              </div>
              <Dialog.Close
                className={styles.iconButton}
                aria-label="Close quick navigation"
              >
                <IconX size={18} aria-hidden="true" />
              </Dialog.Close>
            </div>
            <div className={`${styles.searchWrap} ${styles.stateForm}`}>
              <IconSearch size={18} aria-hidden="true" />
              <label className={styles.visuallyHidden} htmlFor="command-search">
                Search pages
              </label>
              <input
                id="command-search"
                ref={searchInputRef}
                className={styles.searchInput}
                autoComplete="off"
                placeholder="Search pages…"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
              />
            </div>
            <nav
              aria-label="Quick navigation results"
              className={styles.sectionHeaderSpaced}
            >
              <ul className={styles.cleanList}>
                {filtered.map((command) => (
                  <li key={command.to}>
                    <Link
                      className={styles.resourceLink}
                      to={command.to}
                      onClick={() => setOpen(false)}
                    >
                      <span>
                        <strong>{command.label}</strong>
                        <span
                          className={`${styles.helper} ${styles.helperBlock}`}
                        >
                          {command.description}
                        </span>
                      </span>
                      <span aria-hidden="true">→</span>
                    </Link>
                  </li>
                ))}
              </ul>
              {!filtered.length ? (
                <p className={styles.muted}>No matching pages.</p>
              ) : null}
            </nav>
          </Dialog.Popup>
        </Dialog.Viewport>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
