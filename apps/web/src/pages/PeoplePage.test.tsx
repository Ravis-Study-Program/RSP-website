import { render, screen, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

import {
  useEnrollmentCandidates,
  usePeople,
  useSeasons,
  useSeasonPeople,
} from '@/api/queries';
import { useWorkspace } from '@/auth/WorkspaceContext';
import { PeoplePage } from '@/pages/PeoplePage';
import type { Person, SeasonRole } from '@/types';

vi.mock('@/api/queries', () => ({
  useEnrollmentCandidates: vi.fn(),
  usePeople: vi.fn(),
  useSeasons: vi.fn(),
  useSeasonPeople: vi.fn(),
}));

vi.mock('@/auth/WorkspaceContext', () => ({
  useWorkspace: vi.fn(),
}));

function member(role: SeasonRole): Person {
  const label = role[0].toUpperCase() + role.slice(1);
  return {
    id: role,
    name: `${label} Member`,
    slug: `${role}-member`,
    initials: `${label[0]}M`,
    avatarUrl: null,
    roles: [role],
    season: 'Season One',
    status: 'active',
    attempts: 0,
    interviews: 0,
    lastActiveAt: null,
    enrollmentId: `enrollment-${role}`,
    enrollmentRevision: 1,
    enrollmentState: 'active',
  };
}

describe('season people directory', () => {
  it('renders Student, Mentor, and Coordinator badges from enrollment roles', () => {
    vi.mocked(useWorkspace).mockReturnValue({
      workspaces: [],
      activeWorkspace: null,
      setActiveWorkspaceId: vi.fn(),
    });
    vi.mocked(useSeasons).mockReturnValue({
      data: {
        items: [{ id: 'season-1', slug: 'season-one', name: 'Season One' }],
      },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSeasons>);
    vi.mocked(useSeasonPeople).mockReturnValue({
      data: {
        items: [member('student'), member('mentor'), member('coordinator')],
      },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSeasonPeople>);
    vi.mocked(usePeople).mockReturnValue({
      data: { items: [] },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof usePeople>);
    vi.mocked(useEnrollmentCandidates).mockReturnValue({
      data: { items: [] },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useEnrollmentCandidates>);

    render(
      <MemoryRouter initialEntries={['/seasons/season-one/people']}>
        <Routes>
          <Route path="/seasons/:slug/people" element={<PeoplePage />} />
        </Routes>
      </MemoryRouter>,
    );

    const directory = screen.getByRole('region', { name: 'People directory' });
    for (const label of ['Student', 'Mentor', 'Coordinator']) {
      const row = within(directory).getByRole('row', {
        name: new RegExp(`${label} Member`),
      });
      expect(within(row).getByText(label, { selector: 'span' })).toBeVisible();
    }
  });
});
