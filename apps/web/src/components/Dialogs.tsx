import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import {
  AlertDialog,
  AlertDialogTrigger,
  AlertDialogContent,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogCancel,
} from '@/components/ui/alert-dialog';
import {
  Dialog,
  DialogTrigger,
  DialogContent,
  DialogTitle,
  DialogDescription,
  DialogClose,
} from '@/components/ui/dialog';
import { IconTrash, IconX } from '@tabler/icons-react';
import { useId, useState, type PropsWithChildren, type ReactNode } from 'react';

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
    <Dialog open={open} onOpenChange={onOpenChange}>
      {trigger ? (
        <DialogTrigger render={<Button />}>{trigger}</DialogTrigger>
      ) : null}

      <DialogContent showCloseButton={false}>
        <div className={styles.dialogHeader}>
          <div>
            <DialogTitle>{title}</DialogTitle>
            <DialogDescription>{description}</DialogDescription>
          </div>
          <DialogClose
            render={<Button variant="ghost" size="icon" />}
            aria-label="Close dialog"
          >
            <IconX size={18} aria-hidden="true" />
          </DialogClose>
        </div>
        {children}
      </DialogContent>
    </Dialog>
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
  onConfirm: () => void | Promise<void>;
}) {
  const [value, setValue] = useState('');
  const [open, setOpen] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const confirmationId = useId();
  const confirm = async () => {
    setPending(true);
    setError('');
    try {
      await onConfirm();
      setOpen(false);
      setValue('');
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : `Unable to ${actionLabel.toLowerCase()}.`,
      );
    } finally {
      setPending(false);
    }
  };
  return (
    <AlertDialog
      open={open}
      onOpenChange={(next) => {
        if (pending) return;
        setOpen(next);
        if (!next) {
          setValue('');
          setError('');
        }
      }}
    >
      <AlertDialogTrigger render={<Button variant="destructive" />}>
        <IconTrash size={17} aria-hidden="true" /> {actionLabel}
      </AlertDialogTrigger>

      <AlertDialogContent>
        <AlertDialogTitle>
          {actionLabel} {name}?
        </AlertDialogTitle>
        <AlertDialogDescription>{description}</AlertDialogDescription>
        <div className={`${styles.field} ${styles.fieldSpaced}`}>
          <label htmlFor={confirmationId}>
            Type <strong>{name}</strong> to confirm
          </label>
          <Input
            id={confirmationId}
            value={value}
            autoComplete="off"
            onChange={(event) => setValue(event.target.value)}
          />
        </div>
        {error ? (
          <p className={styles.fieldError} role="alert">
            {error}
          </p>
        ) : null}
        <div className={styles.dialogActions}>
          <AlertDialogCancel
            className={styles.buttonSecondary}
            disabled={pending}
          >
            Cancel
          </AlertDialogCancel>
          <Button
            variant="destructive"
            type="button"
            disabled={value !== name || pending}
            onClick={() => void confirm()}
          >
            {pending ? 'Working…' : actionLabel}
          </Button>
        </div>
      </AlertDialogContent>
    </AlertDialog>
  );
}

export function DirtyFormGuard({
  blocker,
}: {
  blocker: { state: string; proceed?: () => void; reset?: () => void };
}) {
  return (
    <AlertDialog open={blocker.state === 'blocked'}>
      <AlertDialogContent>
        <AlertDialogTitle>Discard unsaved changes?</AlertDialogTitle>
        <AlertDialogDescription>
          Your edits have not been saved. You can stay and finish, or discard
          them and leave.
        </AlertDialogDescription>
        <div className={styles.dialogActions}>
          <Button
            variant="outline"
            type="button"
            onClick={() => blocker.reset?.()}
          >
            Stay here
          </Button>
          <Button
            variant="destructive"
            type="button"
            onClick={() => blocker.proceed?.()}
          >
            Discard changes
          </Button>
        </div>
      </AlertDialogContent>
    </AlertDialog>
  );
}
