# Jobs

Mode: Operate. Public visitors browse shared positions. Verified active accounts can publish and maintain private application histories. Referrals requires an active or completed RSP enrollment. Staff edit and merge shared positions.

## Direction contract

THESIS: A practical shared job board, personal application history and member referral directory, using the same table workflow as the rest of RSP.

OWN-WORLD: Preserve Study Ledger, Source Sans 3, existing spacing, borders, light/dark themes and shadcn/Base UI controls. The square expand controls use red, orange, yellow and pink from the existing palette to identify company categories. Category names remain available in hover text and accessible button names on Positions and My Applications, and in expanded details on Positions.

STORY: Browse a position, apply externally, mark Applied, then add named stages and private notes. Staff correct shared data without access to personal histories.

POSITIONS BROWSING: The existing toolbar adds a neutral All, Not applied or Applied selector for signed-in accounts, remembered per account in this browser and combined with search, listing status, category and column filters. Applied means any tracked application, including Rejected and Offer. Added is the immutable date the position entered RSP, not the employer's posting date; use the existing date presentation in a desktop column and a labelled line on mobile cards. The adjacent sort selector defaults to Newest, offers Oldest, and shows Custom sort when another column is primary. Direct position links still reveal and expand their target under either date order even if the personal filter would exclude it.

REFERRAL DISCOVERY: Open Positions shows an anonymous referrer count when a company has opted-in referrers. Eligible RSP members can follow it to the open roles filtered by that company. The Referrals board shows the count beside each role.

APPLICATION STATUS: My Applications opens on Active applications. A status filter beside search offers Active, Rejected, Offer and All. Current Stage shows the latest stage for active applications and a badge for Offer or Rejected. The Edit icon stays available when the history is expanded. The editor lets the user change the status and add private notes to any stage, including Applied. Saving a changed status adds a stage dated today in the browser's local timezone.

FIRST VIEWPORT: Jobs shows Positions and My Applications. Students and alumni also see Request Referrals for their requests, and eligible RSP members see Refer Candidates for their company queue. Each referral page has its own guidelines and remembers acknowledgement for that account in the browser. The guidelines link then sits beside the Jobs Beta heading.

FORM: A three-tab table extension of the existing system for signed-in accounts. No new visual world or concept seed. Application details expand inside their table row; editing uses existing form dialogs. Below the stage list, one footer row places Remove application on the left and Add stage on the right.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Boundaries

REFERRALS: Students and alumni choose up to five open roles from the shared positions board. They use one referral profile with a Google Drive résumé link shared with anyone who has the link and a freeform reason. Their selection stays active until withdrawn or the role closes. RSP graduates self-declare a company and review matching candidates grouped by person. They can see only the candidate’s requested roles at that company, profile reason and résumé link. Referrers initiate contact on Discord. Students see anonymous counts without a referrer directory.

Position notes are public and application stage notes are owner-only. Referral profiles are visible only to their owners and eligible matches. Preserve private stage notes during merges. The supplied positions are imported once without verifying availability.

## Shared patterns

All tabs reuse `DataTable`, its filters and mobile cards. Positions and My Applications also use `ExpandRowButton`; desktop expansion columns have a visible Details heading. Per the requested cleanup, the left expand control is a coloured square with a contrasting chevron, using the central `jobCategoryControlClasses` mapping in `src/jobs.ts`. These category colours describe the company group, not application progress. Company names on these two tabs link to the application, with an external-link arrow before the name; there is no separate Application Link column or inline category badge. Their mobile cards put the same expand control and company link together at the top. Category names remain visible in Positions expanded details and its category filter, and are included in the control's hover text and accessible name on these two tabs.

Editing uses the existing `FormDialog` and field controls. Stage dates use `AppDatePicker`; displayed dates use `formatCalendarDay` in dd/mm/yy form. Beta uses the existing muted badge style beside Jobs in the heading, navigation and page header. The feature adds no design tokens, typefaces or raster assets. The shared system references are `DESIGN.md` and `.impeccable/design.json`.

Position and referral forms share `JobCompanyField`, built from the existing combobox and field primitives. Clicking opens existing companies; typing searches without regard to case. Exact matches reuse a company, while new names offer `Add “name”` after existing matches. Free text can still be submitted without selecting an option. Saving the form creates a new company when needed, making it available in either form; positions retain an existing company's category.
