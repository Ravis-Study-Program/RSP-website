import { render, screen } from '@testing-library/react';

import { ActivityChart } from './ActivityChart';

const weeks = [
  { week: '01/09/26', attempts: 4, interviews: 0 },
  { week: '08/09/26', attempts: 9, interviews: 0 },
];

describe('ActivityChart', () => {
  it('splits LeetCode and mock interviews onto their own charts', () => {
    render(<ActivityChart data={weeks} />);

    expect(
      screen.getByRole('region', { name: 'LeetCode per week' }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('region', { name: 'Mock Interviews per week' }),
    ).toBeInTheDocument();
  });

  it('keeps a chart, flat at 0, when a series has nothing in the period', () => {
    render(<ActivityChart data={weeks} />);

    // Mock interviews are all zero, yet the chart and its weekly summary stay.
    expect(
      screen.getByRole('region', { name: 'Mock Interviews per week' }),
    ).toHaveTextContent('01/09/26: 0 actual mock interviews');
    expect(screen.queryByText(/in This Period/)).not.toBeInTheDocument();
  });

  it('names the actual and expected lines for a student', () => {
    render(
      <ActivityChart data={weeks} expected={{ attempts: 10, interviews: 2 }} />,
    );

    expect(screen.getAllByText('Actual')).toHaveLength(2);
    expect(screen.getAllByText('Expected')).toHaveLength(2);
  });
});
