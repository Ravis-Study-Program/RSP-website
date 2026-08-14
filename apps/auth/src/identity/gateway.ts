export type SessionRevocationReason =
  | 'suspension'
  | 'deletion_requested'
  | 'password_reset'
  | 'password_changed'
  | 'email_changed'
  | 'provider_linked'
  | 'mfa_changed'
  | 'security_admin';

type LifecycleEventIdentity = { readonly eventId: string };

export type IdentityLifecycleEvent = LifecycleEventIdentity &
  (
    | {
        readonly type: 'auth_user_created';
        readonly authUserId: string;
        readonly email: string;
        readonly emailVerified: boolean;
        readonly securityVersion: number;
        readonly occurredAt: string;
      }
    | {
        readonly type: 'email_verified';
        readonly authUserId: string;
        readonly securityVersion: number;
        readonly occurredAt: string;
      }
    | {
        readonly type: 'email_changed';
        readonly authUserId: string;
        readonly email: string;
        readonly emailVerified: true;
        readonly securityVersion: number;
        readonly occurredAt: string;
      }
    | {
        readonly type: 'account_state_changed';
        readonly authUserId: string;
        readonly accountState: 'active' | 'suspended';
        readonly reason: string;
        readonly actorUserId?: string;
        readonly securityVersion: number;
        readonly occurredAt: string;
      }
    | {
        readonly type: 'sessions_revoked';
        readonly authUserId: string;
        readonly reason: SessionRevocationReason;
        readonly securityVersion: number;
        readonly occurredAt: string;
      }
    | {
        readonly type: 'deletion_requested';
        readonly authUserId: string;
        readonly recoveryDeadline: string;
        readonly securityVersion: number;
        readonly occurredAt: string;
      }
    | {
        readonly type: 'deletion_cancelled' | 'auth_pseudonymized';
        readonly authUserId: string;
        readonly securityVersion: number;
        readonly occurredAt: string;
      }
    | {
        readonly type: 'mfa_configured' | 'mfa_disabled';
        readonly authUserId: string;
        readonly securityVersion: number;
        readonly occurredAt: string;
      }
  );

export type NewIdentityLifecycleEvent =
  IdentityLifecycleEvent extends infer Event
    ? Event extends IdentityLifecycleEvent
      ? Omit<Event, 'eventId'>
      : never
    : never;

export interface IdentityLifecycleGateway {
  publish(event: IdentityLifecycleEvent): Promise<void>;
}

export class HttpIdentityLifecycleGateway implements IdentityLifecycleGateway {
  constructor(
    private readonly options: {
      readonly baseUrl: string;
      readonly token: string;
      readonly timeoutMs: number;
      readonly fetchImplementation?: typeof fetch;
    },
  ) {}

  async publish(event: IdentityLifecycleEvent): Promise<void> {
    const fetchImplementation = this.options.fetchImplementation ?? fetch;
    const response = await fetchImplementation(
      `${this.options.baseUrl}/internal/auth/lifecycle-events`,
      {
        method: 'POST',
        headers: {
          authorization: `Bearer ${this.options.token}`,
          'content-type': 'application/json',
        },
        body: JSON.stringify(event),
        signal: AbortSignal.timeout(this.options.timeoutMs),
      },
    );
    if (!response.ok) {
      throw new Error(
        `identity lifecycle callback failed with status ${response.status}`,
      );
    }
  }
}
