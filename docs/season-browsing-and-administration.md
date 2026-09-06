# Season browsing and administration

## Profiles and activity

Opening a profile from a season keeps that season selected. The profile offers
other seasons the person participated in and an All time view, with practice
counts, mock counts, lists and charts calculated for the selected period.
Participation history shows roles, levels, enrollment states and recorded dates.
Staff can review earlier students in their season after removal or season closure.

People lists the season's members. My students lists a mentor's assigned students
and can include previous assignments; coordinators see All students. Closed
seasons include previous assignments by default.

Question pickers search by question number or title. Practice can be filtered by
difficulty, topic and season week; mocks also support result and interviewer
filters. Multiple choices within a filter mean any of those choices, while
different filters apply together. Charts use all matching records across table
pages. Historical charts follow the selected season or year, or the complete
activity range for All time. Suggestions exclude every question the member has
already attempted, even in another season. A suggestion does not record an attempt.

## Invitations

In an open season, an authorized administrator can invite a student, mentor or
coordinator by name and email. The recipient signs in or creates an account,
verifies that same email address, then explicitly accepts. Enrollment is created
only on acceptance. Inviting someone does not create an account or move any
existing account's activity. Coordinator access waits for MFA setup.

An invitation lasts seven days and can be accepted once. Resending gives it a new
link and expiry and invalidates the old link. Cancellation invalidates the link.
The administration screen displays pending, accepted, expired, cancelled and
unsent invitations. Existing members cannot receive a duplicate enrollment in
the same season. Expired invitations can be resent or cancelled.

Migration `00005_season_invitations.sql` adds invitations. Only a hash of the
acceptance token is stored in that table. Mail uses the existing durable auth
outbox and delivery retries. If the initial handoff to the auth service fails,
the invitation remains visibly unsent and an administrator can resend it.
Invitation tokens are carried in the URL fragment and removed from the browser
address after capture; they are not sent as part of the navigation request.

## Corrections and removal

System Admins can correct a person's name, profile slug, avatar URL and Discord.
An email correction sends verification to the new address. The original address
continues to work until verification succeeds; successful verification revokes
existing sessions and synchronizes the application through the identity outbox.

An administrator can correct an enrollment's person or season when both affected
seasons are open. The target person must have an active linked account and no
enrollment in the destination season. Participation dates are retained, and
activity ownership and timestamps stay with their original people. The normal
date rules recalculate season totals. Mentorships attached to the incorrect
enrollment are cleared and recorded in the audit event, so they can be reassigned
deliberately. Closed seasons must be reopened before correction.

Removing a student records kicked; removing a mentor or coordinator records
withdrawn. Removal ends current assignments and participation without erasing
history. Former members can continue recording their own practice and mocks.

Only Directors and System Admins can delete a season, with recent MFA. Deletion
requires no enrollment history, no linked activity, and no pending unexpired
invitations. Removed enrollments and deleted activity still count as history.
Its weeks are hidden with the season and the deletion audit is retained. The
database serializes deletion with new memberships, weeks and invitations so a
concurrent action cannot erase newly created programme records.
