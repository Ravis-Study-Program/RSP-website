import type { AccountState } from '../security/access-policy.js';

export type LifecycleCommand =
  'suspend' | 'request_deletion' | 'cancel_deletion' | 'pseudonymize' | 'erase';

export function nextAccountState(
  current: AccountState,
  command: LifecycleCommand,
): AccountState {
  const transitions: Record<
    AccountState,
    Partial<Record<LifecycleCommand, AccountState>>
  > = {
    active: {
      suspend: 'suspended',
      request_deletion: 'deletion_pending',
      erase: 'deleted',
    },
    suspended: { request_deletion: 'deletion_pending', erase: 'deleted' },
    deletion_pending: {
      cancel_deletion: 'active',
      pseudonymize: 'deleted',
      erase: 'deleted',
    },
    deleted: {},
  };
  const next = transitions[current][command];
  if (!next)
    throw new Error(
      `invalid account lifecycle transition: ${current} -> ${command}`,
    );
  return next;
}

export function isDeletionRecoverable(options: {
  readonly state: AccountState;
  readonly recoveryDeadline: Date | null;
  readonly now: Date;
}): boolean {
  return (
    options.state === 'deletion_pending' &&
    options.recoveryDeadline !== null &&
    options.recoveryDeadline.getTime() > options.now.getTime()
  );
}
