/** What the API says about someone signing up from an invitation. */
export interface InvitationCheck {
  readonly matches: boolean;
  /** The name the invitation was sent to. */
  readonly name: string;
  /** Why the chosen username cannot be used; empty when it can. */
  readonly problem: string;
}

/** A member who has never had a sign-in, as the API describes them. */
export interface UnclaimedMember {
  readonly name: string;
  /** The version their access tokens must carry. */
  readonly securityVersion: number;
}

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
        /** The name given at sign-up (or by Google or Discord). */
        readonly name?: string;
        /** The username chosen when signing up from an invitation. */
        readonly username?: string;
        readonly emailVerified: boolean;
        readonly securityVersion: number;
        readonly occurredAt: string;
      }
    | {
        readonly type: 'discord_avatar_updated';
        readonly authUserId: string;
        readonly avatarUrl: string | null;
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

  /**
   * Whether someone signing up has a live invitation, and the name it was sent
   * to. With a token, it must be that invitation and sent to this email;
   * without one (Google sign-up), any live invitation to the email counts. A
   * username chosen at sign-up is checked and, when free, saved to the
   * invitation; `problem` says why one cannot be used. Null when the API
   * cannot be reached.
   */
  async checkInvitation(input: {
    readonly email: string;
    readonly token?: string;
    readonly username?: string;
  }): Promise<InvitationCheck | null> {
    const fetchImplementation = this.options.fetchImplementation ?? fetch;
    try {
      const response = await fetchImplementation(
        `${this.options.baseUrl}/internal/auth/invitation-email`,
        {
          method: 'POST',
          headers: {
            authorization: `Bearer ${this.options.token}`,
            'content-type': 'application/json',
          },
          body: JSON.stringify(input),
          signal: AbortSignal.timeout(this.options.timeoutMs),
        },
      );
      if (!response.ok) return null;
      const result = (await response.json()) as {
        matches?: unknown;
        name?: unknown;
        problem?: unknown;
      };
      return {
        matches: result.matches === true,
        name: typeof result.name === 'string' ? result.name : '',
        problem: typeof result.problem === 'string' ? result.problem : '',
      };
    } catch {
      return null;
    }
  }

  /**
   * The member with this contact email who has never had a sign-in, with
   * the name and security version their sign-in takes. Null when there is
   * none; throws when the API cannot be reached.
   */
  async findUnclaimedMember(email: string): Promise<UnclaimedMember | null> {
    const fetchImplementation = this.options.fetchImplementation ?? fetch;
    const response = await fetchImplementation(
      `${this.options.baseUrl}/internal/auth/unclaimed-member`,
      {
        method: 'POST',
        headers: {
          authorization: `Bearer ${this.options.token}`,
          'content-type': 'application/json',
        },
        body: JSON.stringify({ email }),
        signal: AbortSignal.timeout(this.options.timeoutMs),
      },
    );
    if (!response.ok)
      throw new Error(
        `unclaimed member lookup failed with status ${response.status}`,
      );
    const result = (await response.json()) as {
      found?: unknown;
      name?: unknown;
      securityVersion?: unknown;
    };
    if (
      result.found !== true ||
      typeof result.name !== 'string' ||
      !result.name ||
      typeof result.securityVersion !== 'number' ||
      !Number.isInteger(result.securityVersion) ||
      result.securityVersion < 1
    )
      return null;
    return { name: result.name, securityVersion: result.securityVersion };
  }
}
