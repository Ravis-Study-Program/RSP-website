import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { useMondayUpdates } from '@/api/queries';
import { MondayUpdateReminder } from './MondayUpdateReminder';

vi.mock('@/api/queries', () => ({ useMondayUpdates: vi.fn() }));
const weeks = [1, 2].map((number) => ({
  id: `week-${number}`,
  seasonId: 'season',
  number,
  resourceUrl: '',
  startAt: `2026-10-${number === 1 ? '05' : '12'}T00:00:00+10:30`,
  endAt: `2026-10-${number === 1 ? '11' : '18'}T23:59:59+10:30`,
}));
const props = {
  seasonId: 'season',
  seasonSlug: 'summer',
  studentId: 'student',
  weeks,
};
function show(items: unknown[] = [], overrides = {}) {
  vi.mocked(useMondayUpdates).mockReturnValue({
    data: { items },
    isLoading: false,
    isError: false,
    ...overrides,
  } as ReturnType<typeof useMondayUpdates>);
  return render(
    <MemoryRouter>
      <MondayUpdateReminder {...props} />
    </MemoryRouter>,
  );
}
beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date('2026-10-11T13:30:00Z'));
});
afterEach(() => vi.useRealTimers());
it('links a Monday reminder directly to the missing programme week', () => {
  show();
  expect(screen.getByRole('alert')).toHaveTextContent(
    'Your Week 1 Monday update is due',
  );
  expect(
    screen.getByRole('link', { name: 'Write your update' }),
  ).toHaveAttribute('href', '/seasons/summer/monday-updates?week=week-1');
});
it('disappears after submission and stays absent on later visits', () => {
  show([
    {
      student: { id: 'student' },
      startDate: '2026-10-05',
      endDate: '2026-10-11',
    },
  ]);
  expect(screen.queryByRole('alert')).not.toBeInTheDocument();
});
it('does not mistake another student or a legacy partial range for a weekly report', () => {
  show([
    {
      student: { id: 'other' },
      startDate: '2026-10-05',
      endDate: '2026-10-11',
    },
    {
      student: { id: 'student' },
      startDate: '2026-10-05',
      endDate: '2026-10-06',
    },
  ]);
  expect(screen.getByRole('alert')).toBeInTheDocument();
});
it.each([{ data: undefined, isLoading: true }, { isError: true }])(
  'does not show a missing-report alert while the query is unavailable',
  (state) => {
    show([], state);
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  },
);
it('does not show on Tuesday', () => {
  vi.setSystemTime(new Date('2026-10-12T13:30:00Z'));
  show();
  expect(screen.queryByRole('alert')).not.toBeInTheDocument();
});
