import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { apiRequest } from '@/api/client';
import { StudentDetailsDialog } from '@/components/StudentDetailsDialog';
import type { Person } from '@/types';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));
vi.mock('@/api/queries', () => ({
  demoMode: false,
  usePracticeSettings: () => ({
    data: {
      goalsEnabled: true,
      easyMinutes: 10,
      mediumMinutes: 25,
      hardMinutes: 40,
      weeklyHours: 30,
    },
  }),
}));

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

const memberPath = '/seasons/season-1/members/enrollment-student';

function renderDialog() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  client.setQueryData(['season-people', 'season-1'], { items: [student] });
  const onOpenChange = vi.fn();
  render(
    <QueryClientProvider client={client}>
      <StudentDetailsDialog
        seasonId="season-1"
        person={student}
        onChanged={vi.fn()}
        open
        onOpenChange={onOpenChange}
      />
    </QueryClientProvider>,
  );
  return { client, onOpenChange };
}

describe('student details dialog', () => {
  beforeEach(() => vi.mocked(apiRequest).mockReset());

  it("sets a student's level, own goals and weekly hours", async () => {
    vi.mocked(apiRequest).mockImplementation(async (path) =>
      path === `${memberPath}/requirements` ? { mediumMinutes: 30 } : {},
    );
    const user = userEvent.setup();
    const { client, onOpenChange } = renderDialog();
    // Saved values fill the fields; blank ones show the program default.
    await waitFor(() =>
      expect(screen.getByLabelText('Medium')).toHaveValue(30),
    );
    expect(screen.getByLabelText('Easy')).toHaveAttribute('placeholder', '10');
    expect(screen.getByLabelText('Weekly hours')).toHaveAttribute(
      'placeholder',
      '30',
    );
    await user.click(screen.getByRole('combobox', { name: 'Student Level' }));
    expect(
      (await screen.findAllByRole('option')).map(
        (option) => option.textContent,
      ),
    ).toEqual(['Novice', 'Beginner', 'Intermediate', 'Advanced']);
    await user.click(await screen.findByRole('option', { name: 'Advanced' }));
    fireEvent.change(screen.getByLabelText('Easy'), {
      target: { value: '15' },
    });
    fireEvent.change(screen.getByLabelText('Weekly hours'), {
      target: { value: '20' },
    });
    await user.click(screen.getByRole('button', { name: 'Save Details' }));
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false));
    expect(apiRequest).toHaveBeenCalledWith(`${memberPath}/student-level`, {
      method: 'PATCH',
      body: JSON.stringify({ studentLevel: 'advanced' }),
    });
    expect(apiRequest).toHaveBeenCalledWith(`${memberPath}/requirements`, {
      method: 'PUT',
      body: JSON.stringify({
        easyMinutes: 15,
        mediumMinutes: 30,
        hardMinutes: null,
        weeklyHours: 20,
      }),
    });
    expect(client.getQueryData(['season-people', 'season-1'])).toEqual({
      items: [{ ...student, studentLevel: 'advanced' }],
    });
  });

  it('refuses out-of-range values and shows an API error', async () => {
    vi.mocked(apiRequest).mockImplementation(async (_path, options) => {
      if (options?.method === 'PUT') throw new Error('The season has closed.');
      return {};
    });
    const user = userEvent.setup();
    renderDialog();
    const hours = screen.getByLabelText('Weekly hours');
    await waitFor(() => expect(hours).toBeEnabled());
    fireEvent.change(hours, { target: { value: '200' } });
    expect(screen.getByRole('button', { name: 'Save Details' })).toBeDisabled();
    fireEvent.change(hours, { target: { value: '25' } });
    await user.click(screen.getByRole('button', { name: 'Save Details' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'The season has closed.',
    );
  });
});
