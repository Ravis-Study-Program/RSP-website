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

RSP Workspace ("RSP mentoring, practice and mock interview workspace") is the members-only, second-generation platform for a season-based mentoring programme that prepares students for technical interviews. Students build a record of practice, mock interviews, and weekly written updates. Mentors and coordinators use that record to guide and run the season. It replaces a legacy C#/Auth0 system. Success means members record activity reliably and staff can see progress without chasing people.

## Positioning

A workspace built around one programme's own mechanics: seasons, programme weeks in the Adelaide timezone, mentor teams, Monday updates, student levels, and fixed practice time goals. It is not a generic LMS or a LeetCode clone.

## Operating Context

- Season lifecycle: definition, invitations (7-day, single-use, email-verified), enrollment, weekly activity, close, optional reopen with a reason.
- Weekly ritual: each student writes a Monday update (a date-range record that can be deleted but not edited), and their mentor marks it.
- Practice attempts are LeetCode-style problems with difficulty and topic, measured against fixed time goals (Easy 20 min, Medium 35 min, Hard 50 min).
- Mock interviews can be recorded for today only (Adelaide date). Timestamps are immutable and edits are versioned.
- History is filtered by season or all time and by programme week. Activity charts come with textual summaries.
- A workspace switcher sets season and role context. There is a command palette (Quick navigation).
- Desktop and mobile are both real use contexts. Mobile has its own navigation.

## Capabilities and Constraints

- Stack in place: React 19 + Vite, shadcn/ui on Base UI, Tailwind CSS 4, TanStack Query/Table, and Tiptap rich text. The Go API and Better Auth service sit behind Caddy on a single origin.
- Members only: `/` redirects to `/dashboard`. The only public routes are the auth flows (sign-in, verify email, two-factor, reset password, account unavailable, accept invitation).
- Privileged roles need TOTP MFA. Accounts can be deleted, with a 30-day recoverable grace period.
- Programme calendar logic uses `Australia/Adelaide`. Storage is UTC. The UI renders each member's saved IANA timezone.
- English only (`en-AU` formatting). No i18n is planned in the repo.
- Student levels: Novice, Beginner, Intermediate, Advance.
- Terminology to preserve: season, week, enrollment, mentorship, mentor team, practice attempt, mock interview, Monday update, mark, level, kicked / withdrawn, graduate, invitation, workspace.
- Undecided: public application and mentor-application pages (`docs/product-backlog.md`). They are out of scope for now. Do not design public marketing or landing surfaces without a new decision.

## Brand Commitments

- Name: "Ravi’s Study Program" (curly apostrophe) in full, "RSP" as the short form, and "RSP Workspace" as the document title.
- Logo: `public/assets/rsp-logo.png`. Favicons and social image live in `public/favicons/`.
- Voice: plain and operational ("Settings saved.", "This season is read-only."). There is no formal brand or voice guide.

## Evidence on Hand

- Real product docs: `docs/role-matrix.md`, `docs/season-browsing-and-administration.md`, `docs/accessibility.md`, `docs/product-backlog.md`.
- `src/data/demo.ts` is fake fixture data (`VITE_USE_DEMO_DATA=true`). It is not real content.
- There are no testimonials, outcome stats, member photos, or sponsor/university branding. Never fabricate them.
- Programme content (application questions, email templates, documents, seminars, selection criteria) has not been supplied yet.

## Product Principles

1. Students log first: the most frequent student actions (record practice, log a mock, write the Monday update) stay the shortest paths.
2. Show the role's own work: navigation and views follow the member's role and season context and do not expose every capability.
3. Records are trustworthy: immutable timestamps, versioned edits, and explicit read-only states are visible, not hidden.
4. Programme time is explicit: weeks and "today" follow the Adelaide programme calendar, and the UI says so where it matters.
5. Use the programme's own terms, not generic LMS vocabulary.

## Accessibility & Inclusion

Target WCAG 2.2 AA (`docs/accessibility.md`). axe runs in e2e (wcag2a/aa, 21a/aa, 22aa). The UI must reflow at 320px and 200% zoom and support forced colors and reduced motion. Desktop VoiceOver was checked; the iOS Safari VoiceOver check is still open.
