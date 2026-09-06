# Role and data-access matrix

Authorization combines account state, global roles, one role per season,
enrollment state, mentorship and resource ownership. A broader role does not
erase lifecycle checks: suspended, deletion-pending and deleted accounts cannot
use protected product features.

## Programme operations

| Operation                               |   Student   |     Mentor      |   Coordinator   |     Director     |   System Admin   |
| --------------------------------------- | :---------: | :-------------: | :-------------: | :--------------: | :--------------: |
| View own open-season workspace          |     Yes     |       Yes       |       Yes       |       Yes        |       Yes        |
| View approved active/alumni directory   |     Yes     |       Yes       |       Yes       |       Yes        |       Yes        |
| Manage own global practice and mocks    |     Yes     |       Yes       |       Yes       |       Yes        |       Yes        |
| View shared practice and mock feedback  |     Yes     |       Yes       |       Yes       |       Yes        |       Yes        |
| Change student role/remove student      |     No      |  Assigned only  |   Own season    |    Any season    |    Any season    |
| Set an active student level             |     No      | Own open season | Own open season | Any open season  | Any open season  |
| Manage enrollments and mentorships      |     No      |       No        | Own open season | Any open season  | Any open season  |
| Manage weeks/resources                  |     No      |       No        | Own open season | Any open season  | Any open season  |
| Close season                            |     No      |       No        |   Own season    |    Any season    |    Any season    |
| Reopen season                           |     No      |       No        |       No        | Yes, with reason | Yes, with reason |
| Create/edit season definitions          |     No      |       No        |       No        |       Yes        |       Yes        |
| Delete an empty season                  |     No      |       No        |       No        |       Yes        |       Yes        |
| Invite members or correct enrollments   |     No      |       No        | Own open season | Any open season  | Any open season  |
| Remove a mentor or coordinator          |     No      |       No        | Own open season | Any open season  | Any open season  |
| Edit profile/contact details            | Own profile |   Own profile   |   Own profile   |   Own profile    |   Any account    |
| Request a verified account email change |     No      |       No        |       No        |        No        |       Yes        |
| Grant global roles                      |     No      |       No        |       No        |        No        |       Yes        |
| Technical/auth administration           |     No      |       No        |       No        |        No        |       Yes        |

Coordinator is a season-scoped administrator, not an alias for System Admin.
There is exactly one season role—student, mentor or coordinator—per enrollment.
Privileged role assignments remain `pending_mfa` until TOTP is configured.

Student levels are Novice, Beginner, Intermediate and Advance. Any active mentor
in the same open season may set a student's level, including students assigned
to another mentor or not yet assigned. This does not change the student's role,
mentor assignment, or the mentor's access to private fields and removal actions.

Practice time goals apply automatically to every member: Easy 20 minutes,
Medium 35 minutes and Hard 50 minutes. The thresholds cannot be customised.
The chart's reference lines can be hidden without changing the goals.
Existing personal-goal rows are retained as historical data and are ignored.

## Field visibility

Basic profile fields are visible to approved active members and alumni: name,
avatar, slug, role badges, activity counts, public problem history and interview
participation.

Private fields are visible only to the member, their assigned mentor, their
season Coordinator, Directors and System Admins:

- Email, Discord and account data.
- Administrative removal reasons.

Practice notes, confidence, mock scores and review text are shared among active
members and student alumni across seasons and between seasons. Viewing these
records does not grant editing rights. Season staff retain read-only access to
their season's former students, including after the staff enrollment completes.

System Admin access is still audited. Authorization is checked before loading
or serializing private data; API clients cannot request it with an `include`
flag.

## Lifecycle and affiliation

| State                             | Protected app                            | Season resources/directory | Historical retention                                  |
| --------------------------------- | ---------------------------------------- | -------------------------- | ----------------------------------------------------- |
| Anonymous                         | Sign-in only                             | No                         | None                                                  |
| Registered, unverified            | Verification/account only                | No                         | Account only                                          |
| Verified nonmember                | Onboarding/profile/settings              | No                         | Account only                                          |
| Active enrollment                 | Yes                                      | Shared seasons and people  | Yes                                                   |
| Completed student                 | Yes as alumni                            | Alumni/global access       | Yes                                                   |
| Completed mentor/coordinator only | Personal activity and past season review | Their former seasons       | Yes                                                   |
| Kicked/withdrawn                  | Personal practice and mocks              | No                         | Earlier season activity and personal history retained |
| Suspended                         | No                                       | No                         | Retained for authorized administration                |
| Deletion pending                  | No; sessions revoked immediately         | No                         | Recoverable for 30 days                               |
| Deleted                           | No                                       | No                         | Pseudonymized programme history only                  |

Alumni derives only from a completed student enrollment. Kicked/withdrawn
enrollments never grant alumni access, although a different completed student
enrollment still can.

Former members can read their own season and week names/numbers and dates to
filter personal activity. Resource links remain hidden when season-resource
access is unavailable.

## Mock interview authority

- Eligible participants are current or former programme members, including
  kicked/withdrawn members. Suspended, deleted and unaffiliated accounts cannot participate.
- The participant picker exposes only the summaries needed to record mocks.
  It does not grant general directory access to former members.
- Interviewer identity is always the authenticated creator.
- New mocks must have occurred today in `Australia/Adelaide`, no later than now.
  The occurrence timestamp is immutable after creation.
- The interviewer can edit duration, rounds, scores and notes and can soft-delete
  the interview, including after leaving or season closure.
- Only the interviewee edits their round review status/comments.
- Participant identity changes require an audited Director/System Admin correction.
  Season identity is calculated from the interviewee's historical student participation.
- Each save appends an immutable version.
- Active members and student alumni can read shared mock feedback. Former season
  staff retain access to mocks attributed to their season, independent of mentor
  assignment. Former members always retain their own received/conducted mocks.

See [Activity dates and seasons](activity-dates-and-seasons.md) for date boundaries,
calendar-year filtering and migration behaviour.

See [Season browsing and administration](season-browsing-and-administration.md)
for invitations, enrollment corrections and deletion rules.

## Mandatory negative tests

Permission tests cover anonymous, unverified, nonmember, Student, Mentor,
Coordinator, Graduate, Director, System Admin, kicked, suspended and deleted
states across the relationship and lifecycle policy suites. Protected routes
have authenticated primary-success tests and anonymous rejection tests.
Focused regressions cover cross-member, cross-mentorship, cross-season and
caller-supplied actor spoofing where those attacks apply. New operations must
extend both the operation matrix and the relevant relationship regressions.
