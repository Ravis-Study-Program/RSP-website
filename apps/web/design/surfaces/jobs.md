# Jobs

Mode: Operate. Public visitors browse shared positions; verified, active accounts publish positions, offer referrals to other verified, active accounts and maintain private application histories. Staff edit and merge shared positions. No season enrollment is required.

## Direction contract

THESIS: A practical shared job board, personal application history and member referral directory, using the same table workflow as the rest of RSP.

OWN-WORLD: Preserve Study Ledger, Source Sans 3, existing spacing, borders, light/dark themes and shadcn/Base UI controls. The square expand controls use red, orange, yellow and pink from the existing palette to identify company categories. Category names remain available in hover text and accessible button names on Positions and My Applications, and in expanded details on Positions.

STORY: Browse a position, apply externally, mark Applied, then add named stages and private notes. Staff correct shared data without access to personal histories.

APPLICATION STATUS: My Applications opens on Active applications. A status filter beside search offers Active, Rejected, Offer and All. Each row has a Status selector; mobile cards pair it with the latest stage. Rejected covers both rejected and ghosted applications. Each change saves immediately and adds a stage named for the new status, dated today in the browser's local timezone with empty notes. Existing history remains intact; repeating the current status adds nothing. A short status announcement confirms the update when a row leaves the current filter. These neutral text controls preserve category colours for company groups.

FIRST VIEWPORT: Signed-in accounts see Positions, My Applications and Referrals at the top of the content, with Add position aligned right on Positions or Open to referring on Referrals. Referrals places its conduct notice above the searchable table. The app header already names Jobs with its Beta badge, so the content heading is screen-reader-only. Public visitors retain a visible Jobs heading and Beta badge below their compact RSP header; their two tabs share a row with Sign in to add a position. Mobile wraps the action when needed and uses the table's existing cards. No metrics or promotional content.

FORM: A three-tab table extension of the existing system for signed-in accounts. No new visual world or concept seed. Application details expand inside their table row; editing uses existing form dialogs. Below the stage list, one footer row places Remove application on the left and Add stage on the right.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Boundaries

REFERRALS: A third tab at `/jobs/referrals`, visible only to verified, active accounts. Reuse the table with Company, Name and fully readable Notes; search and column filters include companies. Names link to existing profiles, with an up-right arrow immediately before each name; members message through Discord, with no new contact field. A white Open to referring action sits at the top right. The form asks for company and optional notes; a keyboard- and touch-accessible notes tooltip suggests roles, GPA thresholds and sending a resume. A prominent yellow warning notice above the table requires individuals to meet every condition, forbids harassment, asks them to respect a declined referral, and states that breaking the rules means removal from the program. This is a conduct notice, not automatic enforcement. Owners can edit or remove their listings, one per company. Referral data lives in its own Jobs table and is never returned anonymously.

Five separate Jobs tables; no changes to the existing program tables. Position notes are public, referral notes are visible to verified, active accounts, and stage notes are owner-only. Preserve all private notes during merges. No uploads, notifications, scraping or mentor viewing of private applications in this beta. The supplied positions are imported once, without claiming their availability was checked.

## Shared patterns

All tabs reuse `DataTable`, its filters and mobile cards. Positions and My Applications also use `ExpandRowButton`; desktop expansion columns have a visible Details heading. Per the requested cleanup, the left expand control is a coloured square with a contrasting chevron, using the central `jobCategoryControlClasses` mapping in `src/jobs.ts`. These category colours describe the company group, not application progress. Company names on these two tabs link to the application, with an external-link arrow before the name; there is no separate Application Link column or inline category badge. Their mobile cards put the same expand control and company link together at the top. Category names remain visible in Positions expanded details and its category filter, and are included in the control's hover text and accessible name on these two tabs.

Editing uses the existing `FormDialog` and field controls. Stage dates use `AppDatePicker`; displayed dates use `formatCalendarDay` in dd/mm/yy form. Beta uses the existing muted badge style beside Jobs in the heading, navigation and page header. The feature adds no design tokens, typefaces or raster assets. The incumbent `DESIGN.md` and `.impeccable/design.json` remain the shared system references; feature access, storage and workflow details live in `docs/jobs.md` at the repository root.

Position and referral forms share `JobCompanyField`, built from the existing combobox and field primitives. Clicking opens existing companies; typing searches without regard to case. Exact matches reuse a company, while new names offer `Add “name”` after existing matches. Free text can still be submitted without selecting an option. Saving the form creates a new company when needed, making it available in either form; positions retain an existing company's category.
