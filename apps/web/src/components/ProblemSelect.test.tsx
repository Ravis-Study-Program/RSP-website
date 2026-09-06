import { useState } from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ProblemSelect } from '@/components/ProblemSelect';
import type { LeetcodeProblem } from '@/api/generated/models';

const problems: LeetcodeProblem[] = [
  {
    id: 'two-sum',
    number: 1,
    title: 'Two Sum',
    link: 'https://leetcode.com/problems/two-sum/',
    difficulty: 'easy',
    categories: ['Array'],
    premium: false,
  },
  {
    id: 'three-sum',
    number: 15,
    title: '3Sum',
    link: 'https://leetcode.com/problems/3sum/',
    difficulty: 'medium',
    categories: ['Array'],
    premium: false,
  },
];

function Harness() {
  const [value, setValue] = useState('');
  return (
    <>
      <label htmlFor="problem">Problem</label>
      <ProblemSelect
        id="problem"
        problems={problems}
        value={value}
        onChange={setValue}
      />
      <output data-testid="selection">{value}</output>
    </>
  );
}

it('finds questions by title and number and requires selecting a real question', async () => {
  const user = userEvent.setup();
  render(<Harness />);
  const input = screen.getByRole('combobox', { name: 'Problem' });
  await user.type(input, 'two SUM');
  expect(
    await screen.findByRole('option', { name: /1\. Two Sum/ }),
  ).toBeVisible();
  expect(
    screen.queryByRole('option', { name: /3Sum/ }),
  ).not.toBeInTheDocument();
  await user.keyboard('{ArrowDown}{Enter}');
  expect(screen.getByTestId('selection')).toHaveTextContent('two-sum');
  await user.clear(input);
  await user.type(input, 'no such question');
  expect(await screen.findByText('No matching questions.')).toBeVisible();
  expect(screen.getByTestId('selection')).toBeEmptyDOMElement();
  await user.clear(input);
  await user.type(input, '15');
  await user.click(await screen.findByRole('option', { name: /15\. 3Sum/ }));
  expect(screen.getByTestId('selection')).toHaveTextContent('three-sum');
});
