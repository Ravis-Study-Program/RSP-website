# Role and data-access matrix

Authorization combines account state, global roles, one role per season,
enrollment state, mentorship and resource ownership. A broader role does not
erase lifecycle checks: suspended, deletion-pending and deleted accounts cannot
use protected product features.

## Programme operations

| Operation                             | Student |    Mentor     |   Coordinator   |     Director     |   System Admin   |
| ------------------------------------- | :-----: | :-----------: | :-------------: | :--------------: | :--------------: |
| View own open-season workspace        |   Yes   |      Yes      |       Yes       |       Yes        |       Yes        |
| View approved active/alumni directory |   Yes   |      Yes      |       Yes       |       Yes        |       Yes        |
| Manage own global practice and mocks  |   Yes   |      Yes      |       Yes       |       Yes        |       Yes        |
| View assigned mentee private activity |   No    |      Yes      |       Yes       |       Yes        |       Yes        |
| Promote/remove an active student      |   No    | Assigned only |   Own season    |    Any season    |    Any season    |
| Manage enrollments and mentorships    |   No    |      No       | Own open season | Any open season  | Any open season  |
| Manage weeks/resources                |   No    |      No       | Own open season | Any open season  | Any open season  |
| Close season                          |   No    |      No       |   Own season    |    Any season    |    Any season    |
| Reopen season                         |   No    |      No       |       No        | Yes, with reason | Yes, with reason |
| Create/edit season definitions        |   No    |      No       |       No        |       Yes        |       Yes        |
| Grant global roles                    |   No    |      No       |       No        |        No        |       Yes        |
| Technical/auth administration         |   No    |      No       |       No        |        No        |       Yes        |

Coordinator is a season-scoped administrator, not an alias for System Admin.
There is exactly one season role—student, mentor or coordinator—per enrollment.
Privileged role assignments remain `pending_mfa` until TOTP is configured.

## Field visibility

Basic profile fields are visible to approved active members and alumni: name,
avatar, slug, role badges, activity counts, public problem history and interview
participation.

Private fields are visible only to the member, their assigned mentor, their
season Coordinator, Directors and System Admins:

- Email and account data.
- Mentor/coordinator notes and removal reasons.
- Mock review text, scores and confidence.
- Practice confidence, private rich-text notes and personal goals.

System Admin access is still audited. Authorization is checked before loading
or serializing private data; API clients cannot request it with an `include`
flag.

## Lifecycle and affiliation

| State                             | Protected app                                         | Season resources/directory    | Historical retention                   |
| --------------------------------- | ----------------------------------------------------- | ----------------------------- | -------------------------------------- |
| Anonymous                         | Sign-in only                                          | No                            | None                                   |
| Registered, unverified            | Verification/account only                             | No                            | Account only                           |
| Verified nonmember                | Onboarding/profile/settings                           | No                            | Account only                           |
| Active enrollment                 | Yes                                                   | Own authorized season         | Yes                                    |
| Completed student                 | Yes as alumni                                         | Alumni/global access          | Yes                                    |
| Completed mentor/coordinator only | Based on another active/completed-student affiliation | No alumni by this state alone | Yes                                    |
| Kicked/withdrawn                  | No access from that enrollment                        | No                            | State and redacted audit retained      |
| Suspended                         | No                                                    | No                            | Retained for authorized administration |
| Deletion pending                  | No; sessions revoked immediately                      | No                            | Recoverable for 30 days                |
| Deleted                           | No                                                    | No                            | Pseudonymized programme history only   |

Alumni derives only from a completed student enrollment. Kicked/withdrawn
enrollments never grant alumni access, although a different completed student
enrollment still can.

## Mock interview authority

- Eligible participants are active members and alumni; unaffiliated,
  suspended, deleted, test and kicked-only users are excluded.
- Interviewer identity is always the authenticated creator.
- The interviewer can edit date, duration, rounds, scores and notes and can
  soft-delete the interview.
- Only the interviewee edits their round review status/comments.
- Interviewer, interviewee or season identity changes require an audited
  Director/System Admin correction after creation.
- Each save records an immutable version before the current row changes.

## Mandatory negative tests

Permission tests cover anonymous, unverified, nonmember, Student, Mentor,
Coordinator, Graduate, Director, System Admin, kicked, suspended and deleted
states across the relationship and lifecycle policy suites. Every protected
OpenAPI operation has an authenticated primary-success test and an anonymous
rejection test. Mutable resource families also cover stale revisions, while
focused regressions cover cross-member, cross-mentorship, cross-season and
caller-supplied actor spoofing where those attacks apply. New operations must
extend both the operation matrix and the relevant relationship regressions.
