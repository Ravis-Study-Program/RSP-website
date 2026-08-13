# RSP Product Backlog

This document captures the initial feature backlog for the RSP website. It is intended to be
prioritised before the next season.

## How to use this backlog

- A top-level checkbox tracks whether a feature has been delivered.
- `Readiness` says whether development can begin without further product decisions.
- `Effort` is an initial relative estimate, not a delivery commitment:
  - `S`: a focused frontend or backend change
  - `M`: a feature spanning multiple parts of the application
  - `L`: a new workflow with database, API, UI, permissions, and tests
  - `XL`: a cross-cutting product area or external integration
- Priority is intentionally unassigned until the backlog is reviewed.

## Delivery summary

| Area | Features | Implemented locally | Ready to investigate | Needs decisions or content |
| --- | ---: | ---: | ---: | ---: |
| Registrations | 5 | 0 | 0 | 5 |
| Student experience | 8 | 1 | 1 | 6 |
| Mentor experience | 5 | 0 | 0 | 5 |
| Coordinator experience | 2 | 0 | 0 | 2 |
| **Total** | **20** | **1** | **1** | **18** |

The mock-interview UX improvement has been implemented locally. The recording and catalogue
problems can begin with technical investigation, although expected behaviour still needs to be
confirmed.

## Recommended delivery order

This order reduces rework; it is not the final product priority.

1. Define shared concepts: application statuses, requirements, flags, strikes, documents, and
   announcements.
2. Fix existing mock-interview and LeetCode recording issues.
3. Deliver the small mock-interview UX improvement.
4. Build requirements and progress calculation services.
5. Build the mentee and coordinator dashboards on those calculations.
6. Build applications and acceptance workflows.
7. Add content, seminar recordings, reports, trackers, and the company directory.

---

## Registrations

### Applications

- [ ] Allow prospective students to apply to RSP directly through the website
  - Priority: `TBD`
  - Effort: `L`
  - Readiness: `Needs product decisions`
  - Likely scope: public application form, draft/submitted states, validation, consent,
    coordinator review view, database records, spam protection, and tests.
  - Decisions required:
    - [ ] Supply the application questions and which are required.
    - [ ] Decide whether applicants need an account before applying.
    - [ ] Define who can view applications and how long application data is retained.
    - [ ] Define whether applicants can save drafts or edit after submission.
    - [ ] Define application opening/closing dates and whether they vary by season.

- [ ] Give coordinators an accept/reject interface with automatic template emails
  - Priority: `TBD`
  - Effort: `XL`
  - Readiness: `Needs workflow and email decisions`
  - Depends on: student applications.
  - Likely scope: review queue, status history, reviewer notes, bulk actions, email templates,
    reliable email delivery, retry/audit log, and permissions.
  - Decisions required:
    - [ ] Define statuses beyond accepted/rejected, such as waitlisted or interview required.
    - [ ] Supply email templates and permitted template variables.
    - [ ] Choose the sending address and email provider.
    - [ ] Decide whether coordinators preview/edit an email before sending.
    - [ ] Decide whether acceptance automatically creates a user and season enrollment.

### Mentor applications and referrals

- [ ] Allow mentor applications directly through the website
  - Priority: `TBD`
  - Effort: `L`
  - Readiness: `Needs product decisions`
  - Decisions required:
    - [ ] Supply the mentor application questions and selection criteria.
    - [ ] Decide whether mentors use the same application workflow and statuses as students.
    - [ ] Define whether existing graduates receive a shortened application.

- [ ] Allow mentors to refer prospective students through the website
  - Priority: `TBD`
  - Effort: `M`
  - Readiness: `Needs privacy and workflow decisions`
  - Decisions required:
    - [ ] Define the information collected about a referred person.
    - [ ] Decide whether a referral creates an application or sends the person an invitation.
    - [ ] Define consent, duplicate detection, and who may see the referring mentor.
    - [ ] Decide whether referrals are limited to a particular season.

- [ ] Allow accepted students to confirm their place through the website
  - Priority: `TBD`
  - Effort: `M`
  - Readiness: `Needs acceptance workflow decisions`
  - Depends on: coordinator acceptance workflow.
  - Decisions required:
    - [ ] Define the confirmation deadline and expiry behaviour.
    - [ ] Decide whether confirmation requires an account, a signed link, or both.
    - [ ] Define additional onboarding questions or agreements.
    - [ ] Decide what happens to declined or expired offers.

---

## Student experience

### Content and announcements

- [ ] Host student documentation on the website instead of linking to Google Drive
  - Priority: `TBD`
  - Effort: `M` for static content; `L` for an editor and publishing workflow
  - Readiness: `Needs content and editing model`
  - Decisions required:
    - [ ] Identify the source documents and confirm migration permission.
    - [ ] Decide whether content is global, program-specific, or season-specific.
    - [ ] Decide who can edit/publish and whether version history is required.
    - [ ] Decide whether search, attachments, and acknowledgements are required.

- [ ] Upload and watch Sunday seminars on the website
  - Priority: `TBD`
  - Effort: `L` when embedding externally hosted video; `XL` when RSP stores/transcodes video
  - Readiness: `Needs media hosting decision`
  - Decisions required:
    - [ ] Choose upload/storage/streaming provider and establish a budget.
    - [ ] Define supported video size, format, captions, thumbnails, and downloads.
    - [ ] Define who uploads and which users can watch each recording.
    - [ ] Decide whether existing recordings must be migrated.

- [ ] Include Monday updates on the website
  - Priority: `TBD`
  - Effort: `M`
  - Readiness: `Needs publishing workflow`
  - Decisions required:
    - [ ] Define the update format and supply examples.
    - [ ] Decide who authors/publishes updates and who can read them.
    - [ ] Decide whether updates are global or season-specific.
    - [ ] Decide whether email/in-app notifications and read receipts are required.

### LeetCode and mock interviews

- [ ] Fix recording mocks involving graduates, kicked students, other non-active students, or
      recently-added LeetCode problems
  - Priority: `TBD`
  - Effort: `M` initially
  - Readiness: `Ready for investigation; expected behaviour needs confirmation`
  - Investigation required:
    - [ ] Capture a failing example for each affected user type.
    - [ ] Confirm who should be selectable as interviewer and interviewee.
    - [ ] Confirm whether a mock can be unrelated to a season.
    - [ ] Confirm whether kicked students retain access or can only be selected by others.
    - [ ] Confirm how quickly a new LeetCode problem should appear.
    - [ ] Determine whether the issue is scraping, caching, API querying, or stale client state.
    - [ ] Define a manual refresh/addition fallback for coordinators.

- [ ] Show the percentage of LeetCode attempts completed under the time goal
  - Priority: `TBD`
  - Effort: `M`
  - Readiness: `Needs metric definition`
  - Decisions required:
    - [ ] Define time goals by difficulty, problem, week, or student level.
    - [ ] Decide whether repeat attempts count independently.
    - [ ] Decide how missing or zero durations are treated.
    - [ ] Decide which date range and season the percentage covers.

- [x] Improve the mock interview view
  - Priority: `TBD`
  - Effort: `S`
  - Readiness: `Implemented locally; awaiting product review`
  - Acceptance criteria:
    - [x] Make received and given mocks clearly distinguishable without opening an options menu.
    - [x] Default to the mocks the user received as interviewee.
    - [x] Show round details and reviewed toggles without requiring each row to be expanded first.
    - [x] Keep an option to view all mocks.

### Applications and employment

- [ ] Give each student an individual job-application tracker
  - Priority: `TBD`
  - Effort: `L`
  - Readiness: `Needs data and privacy decisions`
  - Decisions required:
    - [ ] Define fields and stages such as interested, applied, interview, offer, and rejected.
    - [ ] Decide who can see or edit a student's tracker.
    - [ ] Decide whether reminders, notes, contacts, attachments, or activity history are needed.
    - [ ] Decide whether applications connect to the shared company directory.

- [ ] Add a shared company directory with member suggestions and coordinator approval
  - Priority: `TBD`
  - Effort: `L`
  - Readiness: `Needs directory schema and moderation rules`
  - Decisions required:
    - [ ] Define company fields, tags, locations, links, and eligibility notes.
    - [ ] Define duplicate handling and suggestion statuses.
    - [ ] Decide who can suggest, approve, edit, archive, and view companies.
    - [ ] Decide whether change history and rejection reasons are visible to submitters.

---

## Mentor experience

### Content and reporting

- [ ] Host mentor documentation on the website instead of linking to Google Drive
  - Priority: `TBD`
  - Effort: `M` for static content; `L` for an editor and publishing workflow
  - Readiness: `Needs content and editing model`
  - Decisions required:
    - [ ] Identify mentor-only documents and permissions.
    - [ ] Decide whether the student and mentor documentation share one content system.
    - [ ] Decide who can edit/publish and whether version history is required.

- [ ] Require promotion-interview feedback when promoting a student to novice
  - Priority: `TBD`
  - Effort: `M`
  - Readiness: `Needs feedback schema`
  - Decisions required:
    - [ ] Define required questions, scores, notes, and interview participants.
    - [ ] Decide whether feedback is visible to the student.
    - [ ] Decide whether drafts, multiple reviewers, or coordinator approval are required.
    - [ ] Confirm whether historical promotions need backfilled feedback.

- [ ] Include mentor reports on the website
  - Priority: `TBD`
  - Effort: `L`
  - Readiness: `Needs report definition`
  - Decisions required:
    - [ ] Supply the current report template and cadence.
    - [ ] Define who submits, views, edits, and locks reports.
    - [ ] Decide whether reports cover one mentee or the mentor's entire group.
    - [ ] Define reminders, missed-report handling, and exports.

### Mentee oversight

- [ ] Add a mentor-facing dashboard for every assigned student
  - Priority: `TBD`
  - Effort: `L`
  - Readiness: `Needs requirements and review definitions`
  - Desired metrics:
    - [ ] Total LeetCode attempts by difficulty, split into reviewed/unreviewed.
    - [ ] Total mock interviews split into reviewed/unreviewed.
    - [ ] Net hours above or below requirements.
  - Decisions required:
    - [ ] Define weekly and cumulative LeetCode, mock, and hour requirements.
    - [ ] Define what makes a LeetCode attempt reviewed.
    - [ ] Define what activity contributes to tracked hours.
    - [ ] Define whether missed targets roll over between weeks.
    - [ ] Define date ranges and treatment of excused/partial weeks.

- [ ] Add a strike count that mentors and coordinators can increment or decrement
  - Priority: `TBD`
  - Effort: `M`
  - Readiness: `Needs governance decisions`
  - Decisions required:
    - [ ] Decide whether strikes require a reason, category, evidence, and date.
    - [ ] Decide whether students can see strikes.
    - [ ] Decide whether decrementing deletes a strike or adds a reversing audit event.
    - [ ] Define notification and escalation thresholds.
    - [ ] Define whether strikes reset between seasons.

---

## Coordinator experience

- [ ] Add a coordinator dashboard summarising mentors and their students
  - Priority: `TBD`
  - Effort: `L`
  - Readiness: `Depends on requirements, strikes, and flags`
  - Desired metrics:
    - [ ] For each mentor, show student counts grouped by strike count.
    - [ ] For each mentor, show the number of flagged students.
    - [ ] Allow coordinators to inspect the students contributing to each total.
  - Decisions required:
    - [ ] Decide whether coordinators see only their assigned mentors or the whole season.
    - [ ] Define required filters, date ranges, exports, and refresh frequency.

- [ ] Automatically flag students who need attention
  - Priority: `TBD`
  - Effort: `L`
  - Readiness: `Needs precise rules`
  - Initial flag reasons:
    - [ ] Under required hours.
    - [ ] Under LeetCode requirements.
    - [ ] Under mock-interview requirements.
    - [ ] More than two unreviewed LeetCode attempts.
    - [ ] More than two unreviewed mock interviews.
  - Decisions required:
    - [ ] Define each requirement by role, level, season week, and cumulative period.
    - [ ] Decide whether flags clear automatically when the student catches up.
    - [ ] Define exemptions, grace periods, and manually dismissed flags.
    - [ ] Decide who can see flags and whether students are notified.

---

## Shared foundations likely required

These are not separate user-facing promises, but several backlog items depend on them.

- [ ] Configurable season/program requirement rules
- [ ] Audit history for consequential coordinator and mentor actions
- [ ] Notification and email delivery service
- [ ] Content/document publishing model
- [ ] File/media storage integration
- [ ] Application and approval workflow model
- [ ] Consistent permission model for public, student, mentor, coordinator, and admin access
- [ ] Activity and progress calculation service

## Suggested definition of done

Every delivered feature should include:

- [ ] Agreed acceptance criteria
- [ ] Database migration where applicable
- [ ] Backend authorization and validation
- [ ] API and frontend implementation
- [ ] Automated tests for critical rules
- [ ] Loading, empty, and error states
- [ ] Mobile/responsive review
- [ ] Accessibility review
- [ ] Operational documentation
- [ ] Data migration or backfill plan where applicable
