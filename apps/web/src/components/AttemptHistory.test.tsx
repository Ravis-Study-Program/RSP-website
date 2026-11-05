import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { AttemptHistory } from '@/components/AttemptHistory';
import type { Attempt } from '@/types';

const attempt: Attempt = {
  id: 'attempt-1',
  problemId: 'lc_1',
  problem: 'Two Sum',
  problemUrl: 'https://leetcode.com/problems/two-sum/',
  difficulty: 'Easy',
  category: 'Hash maps',
  outcome: 'independently_solved',
  confidence: 4,
  minutes: 12,
  attemptedAt: '2026-11-05T03:00:00Z',
  notes:
    '<p>Track the complement before inserting each number.</p><ul><li>Check duplicate values.</li><li>Explain the space complexity.</li></ul>',
};

afterEach(() => vi.restoreAllMocks());

it.each(['desktop', 'mobile'])(
  '%s history hides notes and topics until requested and preserves full rich text',
  async (layout) => {
    const matchMedia = window.matchMedia;
    vi.spyOn(window, 'matchMedia').mockImplementation((query) => ({
      ...matchMedia(query),
      matches: layout === 'mobile' && query === '(max-width: 56rem)',
    }));
    const user = userEvent.setup();
    render(
      <AttemptHistory
        ariaLabel="Practice history"
        data={[attempt]}
        emptyTitle="No attempts"
      />,
    );

    expect(screen.getByRole('link', { name: 'Two Sum' })).toHaveAttribute(
      'href',
      attempt.problemUrl,
    );
    expect(screen.queryByText(/Hash maps/)).not.toBeInTheDocument();
    expect(
      screen.queryByRole('region', { name: 'Notes for Two Sum' }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: /Edit|Delete/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('columnheader', { name: 'Actions' }),
    ).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Show topics' }));
    expect(screen.getByText(/Hash maps/)).toBeInTheDocument();
    await user.click(
      screen.getByRole('button', { name: 'Show notes for Two Sum' }),
    );
    const notes = screen.getByRole('region', { name: 'Notes for Two Sum' });
    expect(
      within(notes).getByText(
        'Track the complement before inserting each number.',
      ),
    ).toBeInTheDocument();
    expect(within(notes).getAllByRole('listitem')).toHaveLength(2);
    expect(
      screen.getByRole('button', { name: 'Hide notes for Two Sum' }),
    ).toHaveAttribute('aria-expanded', 'true');

    await user.click(
      screen.getByRole('button', { name: 'Hide notes for Two Sum' }),
    );
    expect(
      screen.queryByRole('region', { name: 'Notes for Two Sum' }),
    ).not.toBeInTheDocument();
  },
);
