import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { cn } from '@/lib/utils';
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
  triggerClassName,
  contentClassName,
  children,
  open,
  onOpenChange,
}: PropsWithChildren<{
  title: string;
  description?: string;
  trigger?: ReactNode;
  triggerVariant?: ButtonProps['variant'];
  triggerSize?: ButtonProps['size'];
  triggerClassName?: string;
  contentClassName?: string;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}>) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {trigger ? (
        <DialogTrigger
          render={
            <Button
              variant={triggerVariant}
              size={triggerSize}
              className={triggerClassName}
            />
          }
        >
          {trigger}
        </DialogTrigger>
      ) : null}

      <DialogContent showCloseButton={false} className={contentClassName}>
        {/* Without a description, centre the title on the close button and
            let the content's own gap set the space below. */}
        <div
          className={cn(
            'flex justify-between gap-4',
            description ? 'mb-4 items-start' : 'items-center',
          )}
        >
          <div className="grid gap-1.5">
            <DialogTitle>{title}</DialogTitle>
            {description ? (
              <DialogDescription>{description}</DialogDescription>
            ) : null}
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
  open: controlledOpen,
  onOpenChange,
}: {
  name: string;
  actionLabel: string;
  description: string;
  onConfirm: () => void | Promise<void>;
  triggerSize?: ButtonProps['size'];
  /** When provided, the dialog is opened externally and renders no trigger. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const [value, setValue] = useState('');
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const controlled = controlledOpen !== undefined;
  const open = controlled ? controlledOpen : uncontrolledOpen;
  const setOpen = (next: boolean) => {
    if (!controlled) setUncontrolledOpen(next);
    onOpenChange?.(next);
  };
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
      {controlled ? null : (
        <AlertDialogTrigger
          render={
            <Button
              variant="destructive"
              size={triggerSize}
              className="w-fit"
            />
          }
        >
          <IconTrash size={17} aria-hidden="true" /> {actionLabel}
        </AlertDialogTrigger>
      )}

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

/**
 * An audited account action (suspending, granting a role…): the reason is
 * recorded, and the person's name must be typed to confirm. `children` adds
 * fields of its own, such as which role to grant.
 */
export function ReasonConfirmation({
  title,
  name,
  actionLabel,
  pendingLabel = 'Working…',
  description,
  destructive = false,
  canConfirm = true,
  children,
  onConfirm,
  open,
  onOpenChange,
}: {
  title: string;
  name: string;
  actionLabel: string;
  pendingLabel?: string;
  description: string;
  destructive?: boolean;
  /** Whether the dialog's own fields allow confirming. */
  canConfirm?: boolean;
  children?: React.ReactNode;
  onConfirm: (reason: string) => Promise<void>;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const [reason, setReason] = useState('');
  const [value, setValue] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');
  const reasonId = useId();
  const confirmationId = useId();
  const confirm = async () => {
    setPending(true);
    setError('');
    try {
      await onConfirm(reason.trim());
      onOpenChange(false);
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
        if (!pending) onOpenChange(next);
      }}
    >
      <AlertDialogContent>
        <AlertDialogTitle>{title}</AlertDialogTitle>
        <AlertDialogDescription>{description}</AlertDialogDescription>
        <div className="mt-4 grid gap-4">
          {children}
          <Field>
            <FieldLabel htmlFor={reasonId}>Reason</FieldLabel>
            <Textarea
              id={reasonId}
              maxLength={500}
              value={reason}
              onChange={(event) => setReason(event.target.value)}
            />
          </Field>
          <Field>
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
        </div>
        {error ? <FieldError>{error}</FieldError> : null}
        <div className="mt-5 flex justify-end gap-2.5">
          <AlertDialogCancel disabled={pending}>Cancel</AlertDialogCancel>
          <Button
            variant={destructive ? 'destructive' : 'default'}
            type="button"
            disabled={
              pending || !canConfirm || !reason.trim() || value !== name
            }
            onClick={() => void confirm()}
          >
            {pending ? pendingLabel : actionLabel}
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
            Stay Here
          </Button>
          <Button
            variant="destructive"
            type="button"
            onClick={() => blocker.proceed?.()}
          >
            Discard Changes
          </Button>
        </div>
      </AlertDialogContent>
    </AlertDialog>
  );
}
