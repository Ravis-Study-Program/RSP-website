import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import type { Enrollment } from '@/api/generated/models';
import { PersonRowActions } from '@/components/PersonRowActions';
import type { CurrentUser, Person, Season } from '@/types';

const season: Season = {
  id: 'season-1',
  slug: 'season-one',
  name: 'Season One',
  status: 'open',
  startsAt: '2026-01-01T00:00:00Z',
  endsAt: '2026-12-31T00:00:00Z',
  location: '',
  imageUrl: '',
  resourcesUrl: '',
  memberCount: null,
  weekCount: null,
  summary: '',
};

const coordinator: CurrentUser = {
  id: 'coordinator',
  name: 'Coordinator',
  slug: 'coordinator',
  avatarUrl: null,
  email: 'coordinator@example.test',
  timezone: 'Australia/Adelaide',
  timezoneConfigured: true,
  emailVerified: true,
  mfaVerified: true,
  accountState: 'active',
  globalRoles: [],
  seasonRoles: [
    {
      seasonId: season.id,
      seasonSlug: season.slug,
      role: 'coordinator',
      state: 'active',
    },
  ],
  alumni: false,
  attemptCount: 0,
  mockInterviewCount: 0,
};

const student: Person = {
  id: 'student',
  name: 'Student Member',
  slug: 'student-member',
  initials: 'SM',
  avatarUrl: null,
  roles: ['student'],
  season: season.name,
  status: 'active',
  attempts: 0,
  interviews: 0,
  lastActiveAt: null,
  enrollmentId: 'enrollment-student',
  seasonRole: 'student',
  studentLevel: 'beginner',
  enrollmentState: 'active',
  mentorshipMentorId: 'mentor',
};

const enrollment: Enrollment = {
  id: 'enrollment-student',
  seasonId: season.id,
  userId: student.id,
  role: 'student',
  studentLevel: 'beginner',
  state: 'active',
  assignmentState: 'active',
};

it('offers a coordinator Remove From Season beside Manage Account', async () => {
  const user = userEvent.setup();
  render(
    <PersonRowActions
      viewer={coordinator}
      person={student}
      season={season}
      seasons={[season]}
      enrollment={enrollment}
      onChanged={vi.fn()}
    />,
  );
  await user.click(
    screen.getByRole('button', { name: 'More actions for Student Member' }),
  );
  expect(
    (await screen.findAllByRole('menuitem')).map((item) =>
      item.textContent?.trim(),
    ),
  ).toEqual(['Edit Details', 'Manage Account', 'Remove From Season']);
});
