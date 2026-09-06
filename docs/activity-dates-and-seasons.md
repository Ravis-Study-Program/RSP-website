# Activity dates and seasons

Practice and mock records belong to people. An attempt stores `attemptedAt`; a
mock stores `occurredAt`. The API calculates their season when reading them.
Clients cannot assign a season or week when creating or editing activity.

## Recording and time zones

- Mock creation accepts only the current calendar day in `Australia/Adelaide`,
  including its daylight-saving rules, and rejects future times.
- The form labels both today's date and the Adelaide time field. It rejects a
  stale form if midnight has passed. The API enforces the same policy.
- Later feedback, round edits, reviews and participant corrections retain the
  original occurrence timestamp. Historical imported mocks remain editable.
- Practice attempts keep their existing date-editing behaviour.
- Stored timestamps are absolute instants. Personal display-timezone preferences
  still control history display; recording eligibility and calendar-year filters
  always use Adelaide time.

## Calculating a season

An activity counts toward a season when its timestamp lies within the season's
start and end dates and a student participation interval in that season.
Attempts use their owner's participation; mocks use the interviewee's.
Interviewer membership does not determine a mock's season.

Season and week bounds are inclusive. Participation starts are inclusive; leaving
or changing out of the student role ends participation exclusively. A student
who rejoins gets a new interval, preserving the gap. Closing or reopening a season
changes its administrative status without rewriting participation dates.

Every record has at most one season. If eligible dates overlap, the season with
the latest start wins, followed by the latest participation start and season ID
as deterministic tie-breakers. Without an eligible student season the record is
personal activity only; it still appears in the appropriate calendar year.

For example, a student removed on 20 January keeps qualifying records before
20 January in that summer season. Their later records stay in their personal
2025 history. Enrolling in the next summer season makes qualifying new records
appear there, with personal 2026 history continuing regardless of enrolment.

Changing season or week dates recalculates existing records on the next read.
Existing weeks must still fit within their season; adjust them before shrinking
its dates. Closed seasons follow the existing reopen-and-edit administration
workflow. Historical audit/version snapshots remain as originally recorded.

Both list APIs accept `seasonId` and `year` together or separately. Year is an
Adelaide calendar year, and pagination cursors are bound to these filters.
Personal pages offer calendar-year filters; season practice/mock pages send the
selected season ID to the API.

## Membership and privacy

All current and former members can maintain their own practice and mocks while
their account remains active. Being kicked or withdrawing does not grant alumni,
directory, season-resource or administrative access. Former members can retrieve
their own season names/dates; restricted resource links are omitted. Legacy
kicked/withdrawn enrolments retain this personal access even when they also
carry the old soft-delete marker.

Active members and student alumni can read each other's practice notes,
confidence and mock feedback across seasons. Completed mentors and coordinators
retain read-only access to activity calculated into their former seasons,
including students who left early. Outside that scope, former members retain
their own activity. Contact details and administrative reasons still require a
separate private-data permission; reading feedback does not grant editing rights.

## Migration

Migration `00004_activity_seasons.sql` creates participation history and derived
activity views. Existing stored season/enrolment/week links are retained as legacy
evidence, but new writes leave them empty and reads ignore them as assignments.

Backfill uses existing enrolment creation and removal timestamps. An earlier
legacy-linked activity can establish an earlier participation start than an
import timestamp. Where no removal event exists, it uses the enrolment state-change
timestamp. Historical roles that were never recorded as intervals cannot be
reconstructed reliably: the initial backfill uses the recorded enrolment role.
All subsequent role and state transitions are recorded by database triggers.

The migration does not invent missing historical membership facts. Deployments
with incomplete legacy timestamps or prior role changes should review those
backfilled intervals before relying on historical season totals. Rolling this
migration back removes the new participation intervals, so back up that history
before downgrading a deployment that has accepted new membership changes.
