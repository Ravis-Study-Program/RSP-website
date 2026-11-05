import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';

import { MockInterviewHistory } from '@/components/MockInterviewHistory';
import { mockInterviews } from '@/data/demo';
import type { MockInterview } from '@/types';

const mock: MockInterview = {
  ...mockInterviews[0],
  notes: '<p>Overall interviewer feedback.</p>',
  intervieweeNotes: '<p>Private overall reflection.</p>',
  rounds: [
    {
      ...mockInterviews[0].rounds[0],
      reviewed: false,
      notes:
        '<p>Explain your approach.</p><ul><li>Test the boundary cases.</li></ul>',
      intervieweeComment: '<p>Private round reflection.</p>',
    },
  ],
};

afterEach(() => vi.restoreAllMocks());

it.each(['desktop', 'mobile'])(
  '%s profile history uses the subject for direction and the viewer for privacy, with no editing',
  async (layout) => {
    const matchMedia = window.matchMedia;
    vi.spyOn(window, 'matchMedia').mockImplementation((query) => ({
      ...matchMedia(query),
      matches: layout === 'mobile' && query === '(max-width: 56rem)',
    }));
    const user = userEvent.setup();
    const history = (viewerId: string, subjectId = mock.interviewee.id) => (
      <MemoryRouter>
        <MockInterviewHistory
          ariaLabel="Profile mocks"
          data={[mock]}
          subjectId={subjectId}
          viewerId={viewerId}
          mode="all"
          emptyTitle="No mocks"
        />
      </MemoryRouter>
    );
    const { rerender } = render(history(mock.interviewer.id));

    expect(screen.getByText('Received')).toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: mock.interviewer.name }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('region', { name: 'Rounds and review' }),
    ).not.toBeInTheDocument();
    await user.click(
      screen.getByRole('button', {
        name:
          layout === 'desktop'
            ? `Show details for ${mock.interviewee.name}`
            : 'View details',
      }),
    );
    const details = screen.getByRole('region', { name: 'Rounds and review' });
    expect(
      within(details).getByText('Overall interviewer feedback.'),
    ).toBeInTheDocument();
    expect(within(details).getByText('Test the boundary cases.').tagName).toBe(
      'LI',
    );
    expect(screen.queryByText(/Private .* reflection/)).not.toBeInTheDocument();

    // Viewing your own profile still cannot toggle reviews or edit interviews.
    rerender(history(mock.interviewee.id));
    expect(screen.getByText('Private overall reflection.')).toBeInTheDocument();
    expect(screen.getByText('Private round reflection.')).toBeInTheDocument();
    for (const checkbox of screen.getAllByRole('checkbox')) {
      expect(checkbox).toHaveAttribute('aria-disabled', 'true');
    }
    expect(
      screen.queryByRole('button', { name: /More actions|Edit|Delete/ }),
    ).not.toBeInTheDocument();

    rerender(history(mock.interviewee.id, mock.interviewer.id));
    expect(screen.getByText('Given')).toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: mock.interviewee.name }),
    ).toBeInTheDocument();
  },
);
