import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { apiRequest } from '@/api/client';
import { StudentLevelDialog } from '@/components/StudentLevelDialog';
import type { Person } from '@/types';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
vi.mock('@/api/queries', () => ({ demoMode: false }));

const student: Person = {
  id: 'student',
  name: 'Student Member',
  slug: 'student-member',
  initials: 'SM',
  avatarUrl: null,
  roles: ['student'],
  seasonRole: 'student',
  studentLevel: 'beginner',
  season: 'Season One',
  status: 'active',
  attempts: 0,
  interviews: 0,
  lastActiveAt: null,
  enrollmentId: 'enrollment-student',
  enrollmentState: 'active',
};

function renderDialog() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  client.setQueryData(['season-people', 'season-1'], { items: [student] });
  render(
    <QueryClientProvider client={client}>
      <StudentLevelDialog
        seasonId="season-1"
        person={student}
        onChanged={vi.fn()}
      />
    </QueryClientProvider>,
  );
  return client;
}

describe('student level dialog', () => {
  it('sets a level without changing the student role', async () => {
    vi.mocked(apiRequest).mockResolvedValue({});
    const user = userEvent.setup();
    const client = renderDialog();
    await user.click(screen.getByRole('button', { name: 'Set level' }));
    expect(
      screen.getByRole('combobox', { name: 'Student level' }),
    ).toHaveTextContent('Beginner');
    await user.click(screen.getByRole('combobox', { name: 'Student level' }));
    expect(
      (await screen.findAllByRole('option')).map(
        (option) => option.textContent,
      ),
    ).toEqual(['Novice', 'Beginner', 'Intermediate', 'Advanced']);
    await user.click(await screen.findByRole('option', { name: 'Advanced' }));
    await user.click(screen.getByRole('button', { name: 'Save level' }));
    expect(apiRequest).toHaveBeenCalledWith(
      '/seasons/season-1/members/enrollment-student/student-level',
      {
        method: 'PATCH',
        body: JSON.stringify({ studentLevel: 'advanced' }),
      },
    );
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(client.getQueryData(['season-people', 'season-1'])).toEqual({
      items: [{ ...student, studentLevel: 'advanced' }],
    });
  });

  it('shows an API error and leaves the level unchanged when a save fails', async () => {
    vi.mocked(apiRequest).mockRejectedValue(
      new Error('The season has closed.'),
    );
    const user = userEvent.setup();
    const client = renderDialog();
    await user.click(screen.getByRole('button', { name: 'Set level' }));
    await user.click(screen.getByRole('combobox', { name: 'Student level' }));
    await user.click(
      await screen.findByRole('option', { name: 'Intermediate' }),
    );
    await user.click(screen.getByRole('button', { name: 'Save level' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'The season has closed.',
    );
    expect(client.getQueryData(['season-people', 'season-1'])).toEqual({
      items: [student],
    });
  });
});
