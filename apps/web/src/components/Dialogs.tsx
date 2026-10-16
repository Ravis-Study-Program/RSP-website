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
import { Field, FieldError, FieldLabel } from '@/components/ui/field';
import { IconTrash, IconX } from '@tabler/icons-react';
import {
  useId,
  useState,
  type ComponentProps,
  type PropsWithChildren,
  type ReactNode,
} from 'react';

type ButtonProps = ComponentProps<typeof Button>;

export function FormDialog({
  title,
  description,
  trigger,
  triggerVariant,
  triggerSize,
  children,
  open,
  onOpenChange,
}: PropsWithChildren<{
  title: string;
  description: string;
  trigger?: ReactNode;
  triggerVariant?: ButtonProps['variant'];
  triggerSize?: ButtonProps['size'];
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}>) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {trigger ? (
        <DialogTrigger
          render={<Button variant={triggerVariant} size={triggerSize} />}
        >
          {trigger}
        </DialogTrigger>
      ) : null}

      <DialogContent showCloseButton={false}>
        <div className="mb-4 flex items-start justify-between gap-4">
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
  triggerSize,
}: {
  name: string;
  actionLabel: string;
  description: string;
  onConfirm: () => void | Promise<void>;
  triggerSize?: ButtonProps['size'];
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
      <AlertDialogTrigger
        render={
          <Button variant="destructive" size={triggerSize} className="w-fit" />
        }
      >
        <IconTrash size={17} aria-hidden="true" /> {actionLabel}
      </AlertDialogTrigger>

      <AlertDialogContent>
        <AlertDialogTitle>
          {actionLabel} {name}?
        </AlertDialogTitle>
        <AlertDialogDescription>{description}</AlertDialogDescription>
        <Field className="mt-4">
          <FieldLabel htmlFor={confirmationId}>
            Type <strong>{name}</strong> to confirm
          </FieldLabel>
          <Input
            id={confirmationId}
            value={value}
            autoComplete="off"
            onChange={(event) => setValue(event.target.value)}
          />
        </Field>
        {error ? <FieldError>{error}</FieldError> : null}
        <div className="mt-5 flex justify-end gap-2.5">
          <AlertDialogCancel disabled={pending}>Cancel</AlertDialogCancel>
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
        <div className="mt-5 flex justify-end gap-2.5">
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
