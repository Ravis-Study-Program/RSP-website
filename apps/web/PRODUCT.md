# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Primary: **students** enrolled in a season of Ravi’s Study Program (RSP). They come back often during the season to record LeetCode practice attempts, log mock interviews, and write their Monday update. Design decisions put fast, low-friction student logging and self-review first.

Secondary, all signed-in members (see `docs/role-matrix.md`):

- **Mentor**: a season enrollment. Reviews assigned students ("My students"), marks Monday updates with private comments, and sets student levels while a season is open.
- **Coordinator**: season-scoped operator. Manages enrollments, mentorships, weeks and resources, and invitations, and closes the season.
- **Director**: global role. Creates and edits season definitions, deletes empty seasons, and reopens seasons with a reason.
- **System Admin**: global role. Grants global roles and handles account, auth, and technical administration.
- **Graduate / alumni**: derived from a completed student enrollment. Read-only directory and shared feedback, and keeps personal practice and mock history.
- **Former member**: kicked or withdrawn. Keeps personal practice and mock history.

## Product Purpose

Ravi’s Study Program (RSP) is the members-only, second-generation platform for a season-based mentoring program that prepares students for technical interviews. Students build a record of practice, mock interviews, and weekly written updates. Mentors and coordinators use that record to guide and run the season. It replaces a legacy C#/Auth0 system. Success means members record activity reliably and staff can see progress without chasing people.

## Positioning

A platform built around one program's own mechanics: seasons, program weeks in the Adelaide timezone, mentor teams, Monday updates, student levels, and program-wide practice time goals. It is not a generic LMS or a LeetCode clone.

## Operating Context

- Season lifecycle: definition, invitations (7-day, single-use, email-verified), enrollment, weekly activity, close, optional reopen with a reason.
- Weekly ritual: each student writes a Monday update for each program week once it ends, their mentor marks it and the coordinator reviews each mentor's marks. Updates exist only for the season's program weeks (a numbered "Week N", Monday to Sunday in Adelaide); dates outside the weeks are not a week of the program.
  - An update is due by the end of the Monday after its week. Statuses: "Not submitted" (before the due date), "Overdue" (past it with no update), "Submitted", and "Submitted (Late)" (posted or last edited after the due date; green like Submitted).
  - Writing is optional; hours are required. Students see one row per ended week and write, edit or delete from its menu while it is open.
  - Past its due date a week locks: the student cannot post, edit or delete it. They send their mentor a "Request unlock" with a reason; the mentor (or a coordinator) approves or declines it, or unlocks the week directly from their table. An unlocked week can be changed, and the change counts as late.
  - The program sets the hours a week it expects of each student (Settings, Weekly Requirements; 30 by default), shown on the update form.
- Mentor review: the mentor's table lists every mentee's update for the chosen week, plus a row for each mentee who has not posted. "Edit notes" sets the mentor's mark (required) and comment, on a posted update or on a week with no update. Marks and notes can be changed at any time; the coordinator always sees the current ones.
- Kicked students: a student kicked during a week still appears in their mentor's table for that week (and earlier ones), with the status "Kicked" when they posted nothing. The mentor marks the week "Kicked" (never set automatically) and explains why in their notes. Later weeks no longer list them.
- Coordinator review: one row per mentor for the week (how many students are marked, how many posted, how many late or overdue, the spread of marks). Expanding shows each student's status, mark, hours, LeetCodes, mocks and the mentor's notes, never the student's own writing.
- My Students shows each mentee's Monday Update status for the week that is due (hidden once the season is over) and their latest mark.
- Student details: from a student list's row menu, "Edit details" lets their mentor or a coordinator set the student's level, their own practice time goals (Easy, Medium, Hard) and their weekly hours. A blank goal or hours follows the program default from Settings; the student's charts, success rates and update form use their own values.
- Practice attempts are LeetCode-style problems with difficulty and topic, measured against the program's practice goals (default Easy 10 min, Medium 25 min, Hard 40 min).
- Mock interviews can be recorded for any day up to today (Adelaide date). Dates are immutable once recorded and edits are versioned.
- History is filtered by season or all time and by program week. Activity charts come with textual summaries.
- One shared navigation menu provides Dashboard, LeetCode and Mock Interviews to every member. Staff pages are added from all of the account’s roles, without selecting a workspace or role. Quick navigation uses the same links.
- Activity tables own their season filters. Seasons opens schedules, resources and permitted student/review pages for each season, including historical ones. Staff navigation shortcuts prefer active enrollments, then completed ones.
- Desktop and mobile are both real use contexts. Mobile has its own navigation.

## Capabilities and Constraints

- Stack in place: React 19 + Vite, shadcn/ui on Base UI, Tailwind CSS 4, TanStack Query/Table, and Tiptap rich text. The Go API and Better Auth service sit behind Caddy on a single origin.
- Members only: `/` redirects to `/dashboard`. The only public routes are the auth flows (sign-in, verify email, two-factor, reset password, account unavailable, accept invitation).
- Privileged roles need TOTP MFA. Accounts can be deleted, with a 30-day recoverable grace period.
- Program calendar logic uses `Australia/Adelaide`. Storage is UTC. The UI renders each member's saved IANA timezone.
- English only (`en-AU` formatting). No i18n is planned in the repo.
- Student levels: Beginner, Novice, Intermediate, Advanced (never "Advance").
- Write "program", never "programme".
- Terminology to preserve: season, week, enrollment, mentorship, mentor team, practice attempt, mock interview, Monday update, mark, level, kicked / withdrawn, graduate, invitation.
- LeetCode attempts are Solved or Unsolved. The database keeps four stored outcomes: independently solved and unrecorded read as Solved, solved with hints and not solved as Unsolved. Confidence is no longer recorded or shown. Only an attempt's owner can mark it reviewed.
- Practice goals (minutes per difficulty, default 10 / 25 / 40) are program-wide, not per member. Directors, system admins and active coordinators set them in Settings; everyone else reads them. Per-mentor goals for their own students are planned (issue #45) but not built.
- Mock interviews are dated, not timed: they can be recorded for any day up to today, never the future. An interview has at most three rounds, in one of four layouts (behavioural then two LeetCode rounds, one behavioural, one custom, or three custom). A round passes at 5/10, and a LeetCode round scores its lowest part.
- The interviewee's notes on a mock interview, overall and per round, are private from the interviewer (the API strips them). The interviewer's notes stay visible to the interviewee. Only the interviewee marks rounds reviewed.
- Profiles show all-time activity in the same tables as the LeetCode and Mock Interviews pages. On someone else's profile, their LeetCode notes and the details of interviews you were not part of are hidden, except from Directors and System Admins, coordinators of a season the person was in, and the person's current or past mentors, who can open them read-only. This is a UI rule; the API still returns notes to members allowed to read the activity.
- The People directory is read-only and the same for every role. Levels are set and students removed from the student lists: All Students for coordinators and administrators, My Students for a mentor's own students.
- The season chosen in any activity table, including All time, is remembered and applied to the others unless the address names a season or weeks. Season and people filters only offer what the member has activity in: seasons where they were a student, people they have had interviews with.
- Undecided: public application and mentor-application pages (`docs/product-backlog.md`). They are out of scope for now. Do not design public marketing or landing surfaces without a new decision.

## Brand Commitments

- Name: "Ravi’s Study Program" (curly apostrophe) in full, "RSP" as the short form, and "Ravi’s Study Program" as the document title.
- Logo: `public/assets/rsp-logo.png`. Favicons and social image live in `public/favicons/`.
- Voice: plain and operational ("Settings saved.", "This season is read-only."). There is no formal brand or voice guide.

## Evidence on Hand

- Real product docs: `docs/role-matrix.md`, `docs/season-browsing-and-administration.md`, `docs/accessibility.md`, `docs/product-backlog.md`.
- `src/data/demo.ts` is fake fixture data (`VITE_USE_DEMO_DATA=true`). It is not real content.
- There are no testimonials, outcome stats, member photos, or sponsor/university branding. Never fabricate them.
- Program content (application questions, email templates, documents, seminars, selection criteria) has not been supplied yet.

## Product Principles

1. Students log first: the most frequent student actions (record practice, log a mock, write the Monday update) stay the shortest paths.
2. Keep navigation shared: add the member’s permitted staff pages without hiding personal activity or requiring a role switch. Season-specific pages enforce that season’s permissions.
3. Records are trustworthy: immutable timestamps, versioned edits, and explicit read-only states are visible, not hidden.
4. Program time is explicit: weeks and "today" follow the Adelaide program calendar, and the UI says so where it matters.
5. Use the program's own terms, not generic LMS vocabulary.

## Accessibility & Inclusion

Target WCAG 2.2 AA (`docs/accessibility.md`). axe runs in e2e (wcag2a/aa, 21a/aa, 22aa). The UI must reflow at 320px and 200% zoom and support forced colors and reduced motion. Desktop VoiceOver was checked; the iOS Safari VoiceOver check is still open.
