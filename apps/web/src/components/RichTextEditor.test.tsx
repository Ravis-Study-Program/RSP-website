import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { RichTextEditor } from '@/components/RichTextEditor';

describe('RichTextEditor link dialog', () => {
  it('validates safe schemes and returns focus to the toolbar control', async () => {
    const user = userEvent.setup();
    render(
      <RichTextEditor
        id="notes"
        label="Notes"
        value="<p>Useful notes</p>"
        onChange={() => undefined}
      />,
    );

    const linkButton = await screen.findByRole('button', { name: 'Link' });
    await user.click(linkButton);
    const dialog = screen.getByRole('dialog', { name: 'Add or edit link' });
    const input = screen.getByLabelText('Link address');
    expect(dialog).toBeInTheDocument();
    expect(input).toHaveFocus();

    await user.clear(input);
    await user.type(input, 'javascript:alert(1)');
    await user.click(screen.getByRole('button', { name: 'Save link' }));
    expect(screen.getByRole('alert')).toHaveTextContent(
      'HTTP, HTTPS or mailto',
    );

    await user.clear(input);
    await user.type(input, 'example.com/guide');
    await user.click(screen.getByRole('button', { name: 'Save link' }));
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    await waitFor(() => expect(linkButton).toHaveFocus());
  });
});
