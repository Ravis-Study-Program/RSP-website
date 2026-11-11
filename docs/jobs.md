# Jobs [beta]

`/jobs` is a public positions board. `/jobs/applications` is the signed-in account's private application history. `/jobs/referrals` lists members offering referrals and is available only to verified, active accounts. All three use the existing table, filters, mobile cards and light/dark themes.

## Access

| Action                                                                     | Access                                                                   |
| -------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| Browse positions and follow application links                              | Anyone                                                                   |
| Publish a position                                                         | Verified, active account; no season enrollment needed                    |
| Track Applied, add or edit stages and private notes, remove an application | Its owner only                                                           |
| Browse referral offers or publish an offer                                 | Verified, active account; no season enrollment needed                    |
| Edit or remove a referral offer                                            | Its owner only                                                           |
| Edit companies or positions, close/reopen listings, merge duplicates       | Active coordinator, director or system admin, with MFA within 15 minutes |

Posting publishes immediately. Authors cannot edit shared listings unless they have a staff role. Mentors and staff cannot read another account's applications or stage notes. Position notes are public and labelled as such in the form.

## Companies

Position and referral forms share the same company field and Jobs company database. Clicking the field opens existing companies; typing filters them without regard to case. An exact match reuses the existing company. A new name shows an `Add “name”` option after existing matches, and typed text can also be submitted without selecting an option. A new company is created only when the form saves successfully and is then available in either form. Positions reuse an existing company's category; new referral companies default to Other Tech.

## Listings and applications

- Job types are Intern, Grad and Experienced. Company categories are FAANG (red), Big Tech (orange), HFT/Quant (yellow) and Other Tech (pink). The square expand control carries the category colour; its hover text and accessible name identify the category on Positions and My Applications. Positions expanded details also show the category. The arrow beside each company opens its application link.
- Signed-in accounts have an All, Not applied or Applied filter on Positions. The selection is remembered per account in this browser and combines with search, listing status, company category and column filters. Applied includes every tracked application, including Rejected and Offer; Not applied excludes them. Visitors do not see this personal filter.
- Added is the date the position was added to RSP, not the employer's posting date. It uses the immutable server `createdAt` timestamp, appears as a desktop column and on mobile cards, and defaults to Newest order. Oldest reverses the date order. Column sorting remains available; the selector shows Custom sort when another column is primary. Direct `/jobs?position=ID` links reveal and expand their target under either date order, including when the remembered application filter would otherwise hide it.
- When a company has available referral offers, signed-in accounts see View N referrer(s) below its name on Positions, including mobile cards. The link opens `/jobs/referrals?company=ID` using the shared company ID. Visitors do not see referral counts or links.
- The initial import contains 26 positions from the supplied 27 rows. Airwallex's identical application link is one position with Sydney and Melbourne locations. The import does not verify that jobs are still open; staff maintain open/closed status.
- Application links must use HTTPS. Duplicate checks remove known tracking parameters but keep job identifiers such as `gh_jid` and `pid`. Links are never fetched by the server.
- Marking Applied creates one application per account and position and opens its history. Repeated or concurrent clicks reuse the same record.
- Applied is the first stage. Its date and private notes are editable, but its name cannot be changed and the stage cannot be removed. Later stages have user-chosen names, dates and optional private notes. The latest dated later stage appears in the table.
- Applications have a separate Active, Rejected or Offer status. Rejected also covers ghosted applications. My Applications defaults to Active; its status filter also shows Rejected, Offer or All applications. Each status change immediately adds an Active, Rejected or Offer stage dated today in the browser's local timezone, with empty notes. Existing stages, notes and the Applied marker remain intact, including when restoring Active. Repeating the current status adds nothing. Only the owner can change this status.
- Closing a listing retains its applications. A merge retains the target listing's fields, combines locations and moves all applications. When an account tracked both listings, its histories combine; source Applied notes are retained as an “Applied (merged listing)” stage. Old position links resolve to the surviving listing.
- A moved application keeps its status. When two applications merge, the surviving target application's status is retained.
- Removing an application deletes its stages. Account deletion clears private application history when the account is permanently pseudonymized. Shared listings remain.

## Referrals

- Referrals is a third Jobs tab, hidden while signed out. Direct visits require sign-in and preserve the return path. The table has Company, Name and fully readable Notes, with search and column filters. A visible Company dropdown starts at All companies and selects by shared company ID. `/jobs/referrals?company=ID` selects that company; the URL retains the selection on reload. Choosing All companies removes the parameter. The conduct notice remains above the filtered table. Names link to existing profiles, with an up-right arrow immediately before each name; contact happens through Discord, with no new contact field or messaging feature.
- Open to referring creates an offer with a company and optional notes. The notes help explains requirements such as roles, GPA thresholds or sending a resume. Each account can have one offer per company and can edit or remove only its own offers. Existing companies keep their category; a newly entered company defaults to Other Tech.
- The yellow warning notice above the table requires individuals to meet every condition in the referrer's notes, avoid harassment and repeated requests, and respect a declined referral. It states that individuals who break these rules will be removed from the program; this is a conduct notice, with no automatic enforcement.
- Offers from unavailable or deleted accounts are hidden. Permanent account pseudonymization removes the account's referral offers. Application histories and stage notes are not shared with referrers.

## Storage and API

Jobs uses five separate tables: `app.job_companies`, `app.job_positions`, `app.job_applications`, `app.job_application_stages` and `app.job_referrals`. Migration `00032_jobs.sql` creates the first four; `00033_jobs_initial_positions.sql` imports the supplied positions once. Neither migration updates existing program or profile data. Rolling back `00032` drops those four tables; the import migration's down step intentionally preserves listings that accounts may have used.

Migration `00034_job_application_status.sql` adds the constrained status field to `app.job_applications`, defaulting existing and new applications to Active. Stored and API values are `active` (Active), `inactive` (Rejected) and `offer` (Offer). The owner-only `PATCH /api/v2/me/job-applications/{id}` route requires `status` and a `date` in YYYY-MM-DD form. It saves the status change and new stage atomically; requests for the current status add no stage.

Migration `00035_job_referrals.sql` adds `app.job_referrals`, sharing the existing Jobs companies and enforcing one offer per account/company. It does not rewrite existing program or profile data; its down step drops only referral offers.

Public API routes are under `/api/v2/jobs/companies` and `/api/v2/jobs/positions`. Private application and stage routes are under `/api/v2/me/job-applications`. The API scopes every private read/write to the authenticated owner, including staff requests. Shared changes are audited without private stage notes. See `api/openapi.yaml` for the contract.

Position responses include the read-only `createdAt` timestamp from the existing `app.job_positions.created_at` column. The server assigns it when a position is inserted; editing or closing a listing does not change it, and a merge retains the target listing's timestamp. Added dates and sorting require no new database migration.

Referral routes require a verified, active account on the server: `GET` and `POST /api/v2/jobs/referrals`, and owner-only `PATCH` and `DELETE /api/v2/jobs/referrals/{id}`. The Jobs request client includes the access token for referral reads as well as writes. Referral list responses use `Cache-Control: private, no-store`.

## Local verification

1. Start the normal development stack and apply pending Goose migrations.
2. Open `/jobs` while signed out. Check search, filters, pagination, expanded details and external application links.
3. Open My Applications, sign in or verify a new email account, and confirm the return to Jobs. A verified account without a season can publish and track positions.
4. Mark a position Applied, add a named stage with notes, and reload. Check that a different account cannot see that history.
   Mark the application Rejected or Offer, find it through the status filter, reload and restore Active. Confirm each change adds its named stage with today's local date and empty notes while existing history and the Applied marker remain intact. Repeating the current status must add no stage.
5. With a staff account and recent MFA, edit, close/reopen and merge positions. Check that existing application histories remain.
6. While signed out, confirm Referrals is hidden and a direct visit requires sign-in. With a verified account without a season, browse and filter referrals, follow a profile link, then create, edit and remove an offer. Check the notes help, conduct notice and duplicate-company rejection; another account must not be able to edit or remove the offer.
7. In both forms, open the company list, search with different casing and select an existing company using the keyboard. Confirm new names show `Add “name”` after existing matches and can also be submitted as free text. Save a referral with a new company, then reuse it in a position and confirm its category is retained. Check the field and popup on desktop and mobile in both themes.
8. On Positions, select Not applied and combine it with search, listing status, category, job type and location filters. Return to Positions or reload and confirm the personal selection remains. Applied must include tracked Rejected and Offer applications. A different account must have its own remembered selection. Check that the personal application filter is absent while signed out.
9. Follow View N referrer(s), confirm the intended company is selected and the conduct notice remains visible, then reload. Choose All companies to clear the company parameter and show the other referrers. Confirm referral links are absent while signed out.
10. Check Added dates on desktop and mobile, Newest by default, Oldest, and another column sort. Publish a position and confirm it appears first under Newest; edit it and confirm its Added date is unchanged. Open a direct link to an older position that conflicts with the personal filter and confirm it is visible and expanded under both date orders.

Jobs browser tests live in `apps/web/e2e/jobs.spec.ts`, `apps/web/e2e/referrals.spec.ts` and `apps/web/e2e/jobs-browsing.spec.ts`. The browsing tests cover application filters, company referral navigation and reload, date sorting, visitor visibility and direct position links. API integration tests in `backend/internal/api/jobs_integration_test.go` and `backend/internal/api/job_referrals_integration_test.go` must use an isolated test database, never a database containing development or real user data. Demo mode uses in-memory fixtures and is excluded from the production bundle.

This beta has no scraping, notifications, approval queue, uploads or mentor access to private applications.
