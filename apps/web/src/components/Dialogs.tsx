import { AlertDialog } from '@base-ui/react/alert-dialog';
import { Button } from '@base-ui/react/button';
import { Dialog } from '@base-ui/react/dialog';
import { IconTrash, IconX } from '@tabler/icons-react';
import { useState, type PropsWithChildren, type ReactNode } from 'react';

import styles from '@/styles/App.module.css';

export function FormDialog({
  title,
  description,
  trigger,
  children,
  open,
  onOpenChange,
}: PropsWithChildren<{
  title: string;
  description: string;
  trigger?: ReactNode;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}>) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      {trigger ? <Dialog.Trigger render={<Button className={styles.button} />}>{trigger}</Dialog.Trigger> : null}
      <Dialog.Portal>
        <Dialog.Backdrop className={styles.dialogBackdrop} />
        <Dialog.Viewport className={styles.dialogViewport}>
          <Dialog.Popup className={styles.dialogPopup}>
            <div className={styles.dialogHeader}>
              <div>
                <Dialog.Title className={styles.dialogTitle}>{title}</Dialog.Title>
                <Dialog.Description className={styles.dialogDescription}>{description}</Dialog.Description>
              </div>
              <Dialog.Close className={styles.iconButton} aria-label="Close dialog">
                <IconX size={18} aria-hidden="true" />
              </Dialog.Close>
            </div>
            {children}
          </Dialog.Popup>
        </Dialog.Viewport>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

export function NamedConfirmation({
  name,
  actionLabel,
  description,
  onConfirm,
}: {
  name: string;
  actionLabel: string;
  description: string;
  onConfirm: () => void;
}) {
  const [value, setValue] = useState('');
  return (
    <AlertDialog.Root onOpenChange={(open) => !open && setValue('')}>
      <AlertDialog.Trigger className={styles.buttonDanger}>
        <IconTrash size={17} aria-hidden="true" /> {actionLabel}
      </AlertDialog.Trigger>
      <AlertDialog.Portal>
        <AlertDialog.Backdrop className={styles.dialogBackdrop} />
        <AlertDialog.Viewport className={styles.dialogViewport}>
          <AlertDialog.Popup className={styles.dialogPopup}>
            <AlertDialog.Title className={styles.dialogTitle}>{actionLabel} {name}?</AlertDialog.Title>
            <AlertDialog.Description className={styles.dialogDescription}>{description}</AlertDialog.Description>
            <div className={styles.field} style={{ marginTop: '1rem' }}>
              <label htmlFor={`confirm-${name}`}>Type <strong>{name}</strong> to confirm</label>
              <input
                id={`confirm-${name}`}
                className={styles.input}
                value={value}
                autoComplete="off"
                onChange={(event) => setValue(event.target.value)}
              />
            </div>
            <div className={styles.dialogActions}>
              <AlertDialog.Close className={styles.buttonSecondary}>Cancel</AlertDialog.Close>
              <AlertDialog.Close
                className={styles.buttonDanger}
                disabled={value !== name}
                onClick={onConfirm}
              >
                {actionLabel}
              </AlertDialog.Close>
            </div>
          </AlertDialog.Popup>
        </AlertDialog.Viewport>
      </AlertDialog.Portal>
    </AlertDialog.Root>
  );
}

export function DirtyFormGuard({ blocker }: { blocker: { state: string; proceed?: () => void; reset?: () => void } }) {
  return (
    <AlertDialog.Root open={blocker.state === 'blocked'}>
      <AlertDialog.Portal>
        <AlertDialog.Backdrop className={styles.dialogBackdrop} />
        <AlertDialog.Viewport className={styles.dialogViewport}>
          <AlertDialog.Popup className={styles.dialogPopup}>
            <AlertDialog.Title className={styles.dialogTitle}>Discard unsaved changes?</AlertDialog.Title>
            <AlertDialog.Description className={styles.dialogDescription}>
              Your edits have not been saved. You can stay and finish, or discard them and leave.
            </AlertDialog.Description>
            <div className={styles.dialogActions}>
              <button className={styles.buttonSecondary} type="button" onClick={() => blocker.reset?.()}>Stay here</button>
              <button className={styles.buttonDanger} type="button" onClick={() => blocker.proceed?.()}>Discard changes</button>
            </div>
          </AlertDialog.Popup>
        </AlertDialog.Viewport>
      </AlertDialog.Portal>
    </AlertDialog.Root>
  );
}
