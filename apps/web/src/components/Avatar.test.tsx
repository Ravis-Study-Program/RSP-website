import { fireEvent, render, screen } from '@testing-library/react';
import { Avatar } from '@/components/Common';

it('shows initials without a photo and after a failed image, then tries a changed URL', () => {
  const { container, rerender } = render(<Avatar initials="MG" />);
  expect(screen.getByText('MG')).toBeVisible();
  rerender(
    <Avatar initials="MG" avatarUrl="https://cdn.discordapp.com/first.png" />,
  );
  expect(container.querySelector('img')).toHaveAttribute(
    'src',
    'https://cdn.discordapp.com/first.png',
  );
  fireEvent.error(container.querySelector('img')!);
  expect(screen.getByText('MG')).toBeVisible();
  rerender(
    <Avatar initials="MG" avatarUrl="https://cdn.discordapp.com/second.png" />,
  );
  expect(container.querySelector('img')).toHaveAttribute(
    'src',
    'https://cdn.discordapp.com/second.png',
  );
});
