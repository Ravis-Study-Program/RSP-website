import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { personOptions } from '@/components/personOptions';

const people = [
  { id: 'a', name: 'Maddie Student', slug: 'maddie-student' },
  { id: 'b', name: 'Maddie Student', slug: 'maddie-student2' },
  { id: 'c', name: 'Noah Williams', slug: 'noah' },
];

describe('personOptions', () => {
  it('adds the slug only for people who share a name', () => {
    const labels = personOptions(people).map((option) => option.label);
    expect(labels[2]).toBe('Noah Williams');
    expect(labels[0]).not.toBe('Maddie Student');
  });

  it('shows the slug in the open list and on the chosen value', async () => {
    const items = personOptions(people);
    render(
      <Select items={items} defaultValue="b">
        <SelectTrigger aria-label="Student">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {items.map((item) => (
            <SelectItem key={item.value} value={item.value}>
              {item.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>,
    );
    expect(screen.getByRole('combobox', { name: 'Student' })).toHaveTextContent(
      'Maddie Student (maddie-student2)',
    );
    await userEvent.click(screen.getByRole('combobox', { name: 'Student' }));
    expect(await screen.findAllByText('(maddie-student)')).not.toHaveLength(0);
    expect(screen.queryByText('(noah)')).not.toBeInTheDocument();
  });
});
