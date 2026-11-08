# Jobs [beta]

`/jobs` is a public positions board. `/jobs/applications` is the signed-in account's private application history. Both use the existing table, filters, mobile cards and light/dark themes.

## Access

| Action                                                                     | Access                                                                   |
| -------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| Browse positions and follow application links                              | Anyone                                                                   |
| Publish a position                                                         | Verified, active account; no season enrollment needed                    |
| Track Applied, add or edit stages and private notes, remove an application | Its owner only                                                           |
| Edit companies or positions, close/reopen listings, merge duplicates       | Active coordinator, director or system admin, with MFA within 15 minutes |

Posting publishes immediately. Authors cannot edit shared listings unless they have a staff role. Mentors and staff cannot read another account's applications or stage notes. Position notes are public and labelled as such in the form.

## Listings and applications

- Job types are Intern, Grad and Experienced. Company categories are FAANG (red), Big Tech (orange), HFT/Quant (yellow) and Other Tech (pink). The square expand control carries the category colour; its hover text, accessible name and expanded details also name the category. The arrow beside each company opens its application link.
- The initial import contains 26 positions from the supplied 27 rows. Airwallex's identical application link is one position with Sydney and Melbourne locations. The import does not verify that jobs are still open; staff maintain open/closed status.
- Application links must use HTTPS. Duplicate checks remove known tracking parameters but keep job identifiers such as `gh_jid` and `pid`. Links are never fetched by the server.
- Marking Applied creates one application per account and position and opens its history. Repeated or concurrent clicks reuse the same record.
- Applied is the first stage. Its date and private notes are editable, but its name cannot be changed and the stage cannot be removed. Later stages have user-chosen names, dates and optional private notes. The latest dated later stage appears in the table.
- Closing a listing retains its applications. A merge retains the target listing's fields, combines locations and moves all applications. When an account tracked both listings, its histories combine; source Applied notes are retained as an “Applied (merged listing)” stage. Old position links resolve to the surviving listing.
- Removing an application deletes its stages. Account deletion clears private application history when the account is permanently pseudonymized. Shared listings remain.

## Storage and API

The feature adds four separate tables: `app.job_companies`, `app.job_positions`, `app.job_applications` and `app.job_application_stages`. Migration `00023_jobs.sql` creates them; `00024_jobs_initial_positions.sql` imports the supplied listings once. Neither migration updates existing program or profile data. Rolling back `00023` drops all Jobs data; the import migration's down step intentionally preserves listings that accounts may have used.

Public API routes are under `/api/v2/jobs/companies` and `/api/v2/jobs/positions`. Private application and stage routes are under `/api/v2/me/job-applications`. The API scopes every private read/write to the authenticated owner, including staff requests. Shared changes are audited without private stage notes. See `api/openapi.yaml` for the contract.

## Local verification

1. Start the normal development stack and apply pending Goose migrations.
2. Open `/jobs` while signed out. Check search, filters, pagination, expanded details and external application links.
3. Open My Applications, sign in or verify a new email account, and confirm the return to Jobs. A verified account without a season can publish and track positions.
4. Mark a position Applied, add a named stage with notes, and reload. Check that a different account cannot see that history.
5. With a staff account and recent MFA, edit, close/reopen and merge positions. Check that existing application histories remain.

Jobs browser tests live in `apps/web/e2e/jobs.spec.ts`. API integration tests in `backend/internal/api/jobs_integration_test.go` must use an isolated test database, never a database containing development or real user data. Demo mode uses in-memory fixtures and is excluded from the production bundle.

This beta has no scraping, notifications, approval queue, uploads or mentor access to private applications.
