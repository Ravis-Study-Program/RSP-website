---
name: RSP Workspace
description: Members-only mentoring, practice and mock interview workspace for Ravi’s Study Program.
colors:
  ink: '#171717'
  paper: '#ffffff'
  paper-on-ink: '#fafafa'
  wash: '#f5f5f5'
  graphite: '#666666'
  hairline: '#e5e5e5'
  pencil: '#a3a3a3'
  sidebar-paper: '#fafafa'
  sidebar-hover: '#eeeeee'
  ledger-red: '#b42335'
  ledger-red-soft: '#fef0f1'
  ledger-green: '#167047'
  ledger-green-soft: '#edf8f1'
  ledger-amber: '#865b0b'
  ledger-amber-soft: '#fff7e6'
  night-paper: '#171717'
  night-card: '#202020'
  night-wash: '#292929'
  night-hairline: '#2e2e2e'
  ledger-blue: '#1d5fa8'
  ledger-blue-soft: '#eaf2fb'
  ledger-purple: '#6b3fbf'
  ledger-purple-soft: '#f1ebfb'
  ledger-pink: '#b0306f'
  ledger-pink-soft: '#fcebf3'
  ledger-orange: '#a84a0c'
  ledger-orange-soft: '#fdf0e6'
typography:
  headline:
    fontFamily: 'Source Sans 3, sans-serif'
    fontSize: 'clamp(1.5rem, 2.5vw, 1.875rem)'
    fontWeight: 600
    lineHeight: 1.12
    letterSpacing: '-0.035em'
  title:
    fontFamily: 'Source Sans 3, sans-serif'
    fontSize: '1rem'
    fontWeight: 500
    lineHeight: 1.375
  body:
    fontFamily: 'Source Sans 3, sans-serif'
    fontSize: '1rem'
    fontWeight: 400
    lineHeight: 1.5
  body-compact:
    fontFamily: 'Source Sans 3, sans-serif'
    fontSize: '0.875rem'
    fontWeight: 400
    lineHeight: 1.43
  label:
    fontFamily: 'Source Sans 3, sans-serif'
    fontSize: '0.75rem'
    fontWeight: 600
    lineHeight: 1.33
  mono:
    fontFamily: 'ui-monospace, monospace'
    fontSize: '0.875rem'
rounded:
  sm: '6px'
  md: '8px'
  lg: '10px'
  xl: '14px'
  pill: '9999px'
spacing:
  card: '16px'
  card-sm: '12px'
  page-x: 'clamp(1.25rem, 3vw, 3rem)'
  page-top: '2rem'
  section: '1.5rem'
components:
  button-primary:
    backgroundColor: '{colors.ink}'
    textColor: '{colors.paper-on-ink}'
    typography: '{typography.body-compact}'
    rounded: '{rounded.lg}'
    padding: '0 10px'
    height: '36px'
  button-outline:
    backgroundColor: '{colors.paper}'
    textColor: '{colors.ink}'
    rounded: '{rounded.lg}'
    padding: '0 10px'
    height: '36px'
  button-ghost:
    textColor: '{colors.ink}'
    rounded: '{rounded.lg}'
    padding: '0 10px'
    height: '36px'
  button-destructive:
    backgroundColor: '{colors.ledger-red-soft}'
    textColor: '{colors.ledger-red}'
    rounded: '{rounded.lg}'
    padding: '0 10px'
    height: '36px'
  input:
    backgroundColor: '{colors.paper}'
    textColor: '{colors.ink}'
    rounded: '{rounded.lg}'
    padding: '4px 10px'
    height: '36px'
  card:
    backgroundColor: '{colors.paper}'
    textColor: '{colors.ink}'
    rounded: '{rounded.xl}'
    padding: '{spacing.card}'
  badge:
    backgroundColor: '{colors.wash}'
    textColor: '{colors.ink}'
    typography: '{typography.label}'
    rounded: '{rounded.sm}'
    padding: '2px 8px'
    height: '20px'
  badge-success:
    backgroundColor: '{colors.ledger-green-soft}'
    textColor: '{colors.ledger-green}'
    typography: '{typography.label}'
    rounded: '{rounded.sm}'
  badge-danger:
    backgroundColor: '{colors.ledger-red-soft}'
    textColor: '{colors.ledger-red}'
    typography: '{typography.label}'
    rounded: '{rounded.sm}'
  nav-link:
    textColor: '{colors.graphite}'
    rounded: '{rounded.sm}'
    padding: '0.58rem 0.7rem'
    height: '44px'
  nav-link-active:
    backgroundColor: '{colors.sidebar-hover}'
    textColor: '{colors.ink}'
---

# Design System: RSP Workspace

## Overview

**Creative North Star: "The Study Ledger"**

RSP Workspace is a ledger: a quiet, trustworthy place where a season of practice, mock interviews and Monday updates gets written down and read back. Ink sits on paper. Near-black on white, with an equally restrained dark mode, and almost nothing else. Members' records are the content. The interface is the ruled page under them.

The mood is calm, precise and restrained. Density is that of a working tool: compact 14px controls, tables, filters and short summaries, never sprawling marketing composition. Hierarchy comes from weight, size and spacing, not colour. Colour carries meaning: status (success, warning, danger, info) and a few fixed categories (season years, student levels, received or given interviews). It is never decorative.

Components should feel exact and unassuming. They are shadcn/ui primitives on Base UI, with gently rounded corners, hairline borders and immediate, motion-light feedback.

**Key Characteristics:**

- Monochrome ink-on-paper palette with a mirrored dark mode (`data-theme="dark"`).
- One typeface, Source Sans 3, at weights 400–700; tight negative tracking on page titles.
- Flat surfaces defined by 1px hairlines and rings. Shadows exist only on floating layers.
- Fixed left sidebar on desktop. Below 56rem it collapses to a mobile header and mobile navigation.
- Status and category colour always pairs with a soft tinted wash and text, and is never decorative. Positive is green, negative is red.

## Colors

A near-monochrome neutral ledger. Hue appears only where it reports status.

### Primary

- **Ledger Ink** (`ink`): primary buttons, links, eyebrows, avatars, selection, and body text. Ink is both the accent and the text colour. Emphasis comes from fill and weight, not a separate brand hue. In dark mode, ink and paper swap: `paper-on-ink` becomes the primary fill.

### Neutral

- **Paper** (`paper`): page background and card surface in light mode.
- **Paper on Ink** (`paper-on-ink`): text on ink fills, and the primary fill in dark mode.
- **Wash** (`wash`): muted fills for secondary buttons, tab lists, hover states, card footers, and soft accents.
- **Graphite** (`graphite`): secondary and supporting text, descriptions, placeholders, and inactive nav labels.
- **Hairline** (`hairline`): every default border and input stroke.
- **Pencil** (`pencil`): focus ring, strong borders, and the secondary chart series.
- **Sidebar Paper** / **Sidebar Hover** (`sidebar-paper`, `sidebar-hover`): the sidebar is a barely-off-white column, and active or hovered nav items use one step darker.
- **Night Paper / Night Card / Night Wash / Night Hairline**: the dark-mode counterparts for background, raised card, muted fill, and border.

### Status

- **Ledger Green** with **Green Wash** (`ledger-green`, `ledger-green-soft`): success states and positive badges.
- **Ledger Amber** with **Amber Wash** (`ledger-amber`, `ledger-amber-soft`): warnings and read-only or attention notices.
- **Ledger Red** with **Red Wash** (`ledger-red`, `ledger-red-soft`): destructive actions, invalid fields, errors, and every negative state. Destructive buttons are a red-on-wash tint with a light red border, not a solid red block.
- **Ledger Blue** with **Blue Wash** (`ledger-blue`, `ledger-blue-soft`; the `info` tokens): neutral information, such as a received mock interview or an on-track Monday mark.

### Categories

A few fixed categories have their own wash-and-text pair (tokens `purple`, `pink`, `orange` beside the status ones in `src/styles/tokens.css`). The tone for each lives in one place in code, never inline:

- **Season years** (`SeasonBadge`): a season is named by the two years it spans ("24/25") and coloured by its start year, cycling every seven seasons: 20/21 red, 21/22 orange, 22/23 yellow, 23/24 green, 24/25 blue, 25/26 purple, 26/27 pink.
- **Student levels** (`studentLevelTone`): Beginner grey, Novice green, Intermediate blue, Advanced purple.
- **Mock interview type**: Received blue, Given yellow.
- **Difficulty**: Easy green, Medium yellow, Hard red.

### Named Rules

**The Ink-Only Accent Rule.** There is no brand hue. Primary emphasis is ink, and any non-neutral colour on screen must answer "what status is this?"

**The Paired Wash Rule.** Status colour always appears as a text/icon colour on its own soft wash, so meaning survives in both themes and in forced-colors mode.

**The Red Is Negative Rule.** Every positive state is green and every negative one red, wherever it appears: On/Off, Pass/Fail, Solved/Unsolved, a member's status. Statuses go through `statusTone` (`src/badgeTones.ts`): active and open green; pending and unassigned yellow; kicked, withdrawn, suspended, deleted and revoked red; completed and closed grey.

**The One Border Rule.** Every border and card edge is the `--border` token (`border-border`, `ring-border`), the darker of the greys the app once used. Never `ring-foreground/10` or a one-off grey.

## Typography

**Body Font:** Source Sans 3 (with sans-serif), self-hosted via `@fontsource`, weights 400/500/600/700
**Mono Font:** ui-monospace (with monospace), for code-like values only

**Character:** One humanist sans does all the work. It is friendly enough for a student's weekly record and neutral enough for admin tables. Personality comes from weight and tracking, never from a second display face.

### Hierarchy

- **Headline** (600, `clamp(1.5rem, 2.5vw, 1.875rem)`, 1.12, −0.035em): page titles. Metric and hero figures go a step larger (up to `clamp(1.7rem, 3vw, 2.35rem)`, −0.04em).
- **Title** (500, 1rem, 1.375): card titles and section headings.
- **Body** (400, 1rem, 1.5): base reading text and form values on mobile. Ledes cap at 48rem.
- **Body Compact** (400–500, 0.875rem): the dominant UI size for buttons, inputs from `md` up, table cells, card bodies, and tab triggers.
- **Label** (600, 0.75rem): eyebrows above page titles (sentence case, no letter-spacing, ink colour), badges, and small meta.

### Named Rules

**The Title Case Rule.** Titles and buttons are Title Case: page titles, section and panel headings, dialog titles, tab labels, error and empty-state titles, and the text of every button, including dialog actions and links styled as buttons ("Edit Notes and Scores", "Two-Factor", "Time by Difficulty", "No Practice in This Period", "Write Week 3 Update", "Save Settings"). Small words (a, an, and, as, at, by, for, in, of, on, or, the, to, with) stay lower case unless first or last, and each part of a hyphenated word is capitalised ("Sign-In").

**The Sentence-Case Label Rule.** Everything that is not a title or a button is sentence case with normal tracking: labels, eyebrows, menu items, badges and body text ("Edit notes and scores" as a menu item, "Hours per week" as a field label). Uppercase letter-spaced labels are not part of this system.

**Bracketed Detail.** Bracketed text after a label or title, such as "(optional)", "(minutes)", "(0–10)" or "(Expected: ~30h/week)", is normal weight in muted grey, brackets included, while the label itself stays its usual weight. Use `LabelHint` (`src/components/Common.tsx`) rather than styling it by hand.

## Layout

- **Shell:** a fixed 15rem left sidebar (brand, workspace switcher, role-based nav, settings, and user summary at the foot) and a workspace body with a slim workspace breadcrumb header ("Workspace / season").
- **Page:** content maxes at 92rem and is centred. Main padding is `2rem clamp(1.25rem, 3vw, 3rem) 4rem`. The page header (`PageHeader`) puts title, eyebrow and lede on the left and actions on the right, and carries its own 1.5rem bottom margin: the title always sits 1.5rem above the content. Put the content after the header in one `flex flex-col gap-6` stack. Never put `PageHeader` inside a gap stack or give the first section a top margin; either doubles the gap.
- **Grids:** three-column card and metric grids drop to two columns below 70rem. Below 56rem the sidebar is hidden, a mobile header and mobile navigation appear, tables switch to stacked mobile cards, and page headers stack.
- **Rhythm:** a compact 4px-based Tailwind scale. Card padding is 16px (12px for small cards). Typical gaps are 0.5–1.25rem.
- **Section spacing:** blocks stacked down a page — panels, filter rows and dropdowns, tables, chart cards — are 1.5rem apart, the same as the gap below the page header, and so are side-by-side panels and card or metric grids (dashboard, profile, season page, admin). Stack them in a `flex flex-col gap-6` container rather than giving each block its own margin, so gaps never double up; `DataTable` already stacks its table and summary this way. A section's title sits 12px above its panel, outside it (`PageSection`).
- **Panel insets:** content sits 16px (`p-4`, the card padding) in from its panel's edge, so controls line up down the page: panel bodies, chart cards, the table toolbar and pagination bars, and the first and last table columns (cells use `px-4`). Only small cards and popovers use 12px.
- **Floor:** the minimum width is 320px, and layouts must reflow at 200% zoom.

## Elevation & Depth

The system is flat. Surfaces sit on the page, separated by 1px hairline borders or a 1px `ring-border` ring (the One Border Rule), plus tonal steps (paper → wash, or night paper → night card). Shadows appear only on layers that genuinely float above the page.

### Shadow Vocabulary

- **Float** (`box-shadow: 0 8px 24px rgb(0 0 0 / 0.18)`): popovers, comboboxes, the date-picker calendar, and timezone lists.
- **Dialog** (`box-shadow: 0 12px 32px rgb(0 0 0 / 0.22)`): modal dialogs.

### Named Rules

**The Flat Ledger Rule.** Nothing resting on the page casts a shadow. A shadow means "this is floating and will go away": menus, popovers, dialogs.

## Shapes

Corners are gently rounded and consistent. The base radius is 10px (`--radius: 0.625rem`), and other sizes derive from it: 6px for nav items, selects and small surfaces, 8px for compact controls, 10px for buttons, inputs and tab lists, and 14px for cards. Avatars are circles. Badges are square-cornered (6px, `rounded-sm`), never pills. Borders are always 1px hairlines. Card images clip to the card's corners.

## Components

### Buttons

Exact and unassuming.

- **Shape:** gently rounded (10px). Heights are 36px by default, 28px for small and 24px for extra-small.
- **Primary:** ink fill, paper-on-ink text, 14px medium, 10px horizontal padding.
- **Hover / Focus:** hover drops the fill to 80% opacity. Focus shows a 3px pencil ring at 50%. Press nudges the button down 1px. Disabled is 50% opacity.
- **Outline / Secondary / Ghost:** outline is paper with a hairline border and washes on hover. Secondary is a wash fill. Ghost is transparent and washes on hover.
- **Destructive:** ledger red on a 10% red tint with a light red (40%) border, deepening to 20% on hover.
- **Row actions:** an action at the end of a settings-style row is a default (solid) button, so it reads as a button. A method that is already connected shows a disabled default button with a check ("Google connected") instead of a second row. Table row actions live in a kebab menu instead.
- **Link:** ink text, underlined on hover.

### Chips / Badges

- **Style:** every inline badge is a `StatusBadge`: 20px tall, 12px semibold text, square-cornered (6px), no border, on a `tone` (`neutral`, `muted`, `success`, `warning`, `danger`, `info`, `purple`, `pink`, `orange`; `strongSuccess` and `strongDanger` exist but no badge uses them). Never a raw `Badge` with hand-written colour classes. Monday marks use the soft tones, strongest first: way ahead `purple`, doing good `info` (blue), on track `success` (green), a bit behind `warning` (yellow), at risk `orange`, kicked `danger` (red). Badges stay legible in forced-colors mode.
- **Placement:** status badges sit inline with what they describe (beside a row title, or small beside the @handle on a profile), never on their own line.

### Cards / Containers

- **Corner Style:** 14px.
- **Background:** paper (night card in dark mode).
- **Shadow Strategy:** none (Flat Ledger Rule). The edge is a 1px `ring-border` ring.
- **Destructive panels:** a red card (`bg-destructive-soft ring-destructive`) laid out like the other rows: title and description on the left, the destructive action on the right.
- **Internal Padding:** 16px, or 12px for the small size. Footers sit on a 50% wash with a hairline top border.

### Inputs / Fields

- **Style:** 36px tall, 10px radius, hairline stroke, transparent fill. Text is 16px on mobile and 14px from `md` up, so iOS does not zoom on focus.
- **Focus:** the border shifts to pencil and a 3px pencil ring appears at 50%.
- **Error / Disabled:** `aria-invalid` gives a red border and a 20% red ring. Disabled is 50% opacity with a wash fill.

### Settings-Style Row Lists

- A titled section (its own `h2`, like every other section) holds one card of rows divided by hairlines. Each row has a bold title, a muted one-line description and an action on the right.
- Group rows into separate titled sections rather than subheadings inside one card. Settings uses Sign-In (email, password, Google), Two-Factor (two-factor, then backup codes, disabled until two-factor is on) and Sessions.
- Read-only values use the same labelled field as editable ones, greyed (`text-muted-foreground`) and `readOnly`, rather than plain text.

### Tables

- Every list is a `DataTable`. Its toolbar holds the search, the view tabs (`toolbarStart`) and table-switching tabs (`toolbarLead`, before the search), Compact rows, column visibility, Filters and full screen.
- A chevron on the left expands the row, in a first column headed "Details" (visible, never screen-reader only, on every table with expandable rows); a kebab (⋮) menu in a right-aligned actions column (`meta: { align: 'end' }`) holds its actions: Show/Hide details first, then edits, destructive actions last. The trigger is labelled "More actions for …" naming the row's subject. An action appears in one place only.
- Every body row in a table is the same height, whether or not it holds a button: 56px, or 48px with Compact rows (`DataTable` sets it on the cells).
- A sub-table inside an expanded row (mock interview rounds, a mentor's students) uses the same heading size as the table's column headings: 12px semibold, muted.
- Expanded rows and cells keep the 16px inset on both sides. Headings line up with the start of their column's content; when a heading row and body rows are separate CSS grids, every column needs a fixed or `fr` width.
- Sorts chain and are stable: clicking a column sorts by it first, the columns sorted before it still order its ties (newest first), and rows tied on every sort keep their original order. `DataTable` does this for every table.
- Roles and statuses sort, and list inside a cell, in a fixed order rather than alphabetically: roles Coordinator, Mentor, Student; student statuses Active, Completed, Kicked. Use `sortRoles`, `roleSortingFn` and `statusSortingFn` (`src/sortOrder.ts`).
- Columns run from most to least important, left to right, each sized to its data: short fixed values (outcome, result, duration, reviewed) stay narrow.
- Filters live in the Filters popover, one control per concept, every dropdown full width. Choice columns use `meta.filterOptions`; dates use `recentDateFilterOptions` / `withinRecentDays` (`src/tableFilters.ts`); page filters go in `extraFilters`. Turn off a column's text filter when a dropdown covers it. Options only offer what the member has activity in.
- Review state is a checkbox (partly filled when some parts are reviewed), not a status badge.
- Every cell is one line. Split a two-part value into two columns (a Monday update's date range and its mark are Latest Update and Mark), and give a person's name its own cell, with roles in a Roles column.
- A cell holds either text or badges, never both: a missing state is a badge too (a grey "Not marked"), and an empty value is a dash.
- The People directory and the student lists (All Students, My Students) are one `PeopleTable` and look the same for every role. People is read-only; managing students (Set level, Remove student) lives in the student lists' row menu.

### Dialogs and Forms

- Dialogs are `FormDialog`; typed confirmations of destructive actions are `NamedConfirmation`. No explanatory subtitle under a dialog title, unless it warns about a consequence (removed access, signing out, invalidated codes).
- Editing dialogs show every part in one scrolling form (all rounds at once, not a picker), and the same record's dialogs behave the same for every role.
- Editable dates use `AppDatePicker` (dd/mm/yyyy, whatever the browser); never a native date input. Groups of "Add …" buttons are equal width and fill the row.

### Navigation

- **Sidebar:** 44px-tall nav links in graphite, 15px at weight 550, with 6px radius. Hover and active states darken the text to ink on the sidebar-hover step. There is no accent bar or colour.
- **Tabs:** a wash track with a 3px inset. The active tab is a paper chip. A line variant uses a 2px ink underline.
- **Mobile:** below 56rem a mobile header (brand "RSP") and a dedicated mobile navigation replace the sidebar.
- **Command palette:** Quick navigation is available as a keyboard-first alternative.
- **Account card:** the name and role at the foot of the sidebar link to the member's profile.

### Component Inventory

Reuse these before writing new markup. If a page needs something close to one of them, extend the shared component rather than copying it; the profile and the LeetCode and Mock Interviews pages share their tables for this reason.

**Page structure** (`src/components/Common.tsx`, `src/components/StatusViews.tsx`)

- `Page`: the centred column, at most 92rem wide, that every page and page-level state sits in (`as="main"` outside the app shell).
- `LabelHint`: bracketed detail after a label or title, in muted grey.
- `PageHeader`: every page title, with optional description, status and actions. Owns the 1.5rem gap below. Its `eyebrow` prop exists but is unused; do not use it (no kickers above headings).
- `PageSection`: a titled section: an `h2` that labels the section, optional `actions` on the right, and the content 12px below.
- `Avatar`: a member's initials in an ink circle (`size="lg"` on the profile header).
- `MetricCard`: a labelled figure in a metric grid.
- `InlineNotice`: a one-line success or warning notice at the top of a page or section.
- `ErrorState`, `EmptyState`, `PageSkeleton`: failed, empty and loading states, with a Title Case title.

**Tables and activity** (`src/components`)

- `DataTable`: every table, with search, filters, column visibility, pagination, expandable rows and mobile cards.
- `AttemptTable`: LeetCode attempts, on the LeetCode page and profiles. Supply actions to make it editable; `showNotes` and `canReview` follow the viewer.
- `MockInterviewTable` and `MockTypeField` (`src/pages/MockInterviewsPage.tsx`): mock interviews, on the Mock Interviews page and profiles. `canViewDetails` limits whose details open.
- `PracticeAnalytics`: the difficulty breakdown and time-by-difficulty charts for a set of attempts.
- `PeopleTable` and `StudentRowActions` (`src/components/PeopleTable.tsx`): the People directory and the student lists, with the student row menu.
- `LatestUpdateRange` and `LatestMarkBadge` (`MondayMark.tsx`): a student's latest Monday update, as a date range and a mark badge in separate cells.
- `MockFilters`, `SeasonWeekChoices`, `ActivityPeriodFilter`: filter controls for the Filters popover.
- `ExpandRowButton` (`DataTable.tsx`): the chevron that opens a row's details, labelled "Show/Hide …" with the row's subject.
- `HighlightText`: search-match highlighting inside table cells.

**Badges and people**

- `StatusBadge` (`src/components/StatusBadge.tsx`) with `statusTone` and `BadgeTone` (`src/badgeTones.ts`): every badge.
- `RoleBadge`, `SeasonBadge`, `StudentLevelBadge` (`Common.tsx`), `MarkBadge` (`MondayMark.tsx`): badges with a fixed meaning built on `StatusBadge`.
- `PersonIdentity` (`Common.tsx`): a person's avatar, name and roles, linking to their profile.

**Content and controls**

- `ExternalLink` (`src/components/ExternalLink.tsx`): any inline link that opens a new tab, with the external-link icon (LeetCode questions, references).
- `ResourceLink` (`Common.tsx`): a block link to an outside resource, such as a week's materials.
- `FormDialog`, `NamedConfirmation`, `DirtyFormGuard` (`src/components/Dialogs.tsx`): dialogs, typed confirmations and unsaved-change guards.
- `AppDatePicker`: editable dates. `TimezoneSelect`: time zones, listed by UTC offset, smallest first.
- `RichTextEditor` and `RichTextContent`: notes, edited and displayed.
- `ProblemSelect`: choosing a LeetCode problem. `Spoiler`: hidden-until-clicked text such as topics.
- `ScoreHelp` (`MockInterviewsPage.tsx`): the one-sentence info icon beside a score.

### Known Drift

Places that still bypass the inventory or the rules. Fix them when touching the code, and add new findings here rather than copying them.

- Section spacing: the dashboard, season page and Monday updates still space sections with top margins (`mt-6`, and `mt-4` on the season page's grid) rather than one `gap-6` stack, so the gap above their first section is larger than 1.5rem. Move them into the page's stack.
- The kebab row menus (`AttemptRowMenu` in the LeetCode page, `MockRowActions` in Mock Interviews, `StudentRowActions` in `PeopleTable`, `StudentUpdateRowActions` in Monday updates) share no base. Extract a row-menu component (trigger, Show/Hide details item, dialog state) when touching them next.
- Monday updates keeps its own card and subsection title classes (`cardTitleClass`, `subTitleClass`) for the per-student blocks inside a section.
- Buttons are Title Case (see the Title Case Rule), but most still read sentence case: "Save settings", "Sign in with email", "Log attempt", "Clear filters", "Discard changes", "Change password" and `actionLabel`s such as "Delete attempt". Convert them, and the matching names in `e2e` and `e2e-real`, in one sweep.
- `PageSkeleton` draws the page column itself (it needs ARIA attributes `Page` does not take).

## Do's and Don'ts

### Do:

- **Do** take emphasis from ink fill, weight (500–700) and size before reaching for colour.
- **Do** style with Tailwind utilities on shadcn/ui components (`src/components/ui`), using the theme tokens mapped in `src/styles/global.css` (`bg-primary`, `bg-muted`, `border-border`, `text-success` / `bg-success-soft`, `bg-sidebar` and so on; values live in `src/styles/tokens.css`) so both themes stay in sync. No CSS modules or literal colours.
- **Do** keep one solid (`default`) button per header or dialog; row and secondary actions are `outline`, destructive actions `destructive`, and links styled as buttons use `buttonVariants` on `Link`.
- **Do** pair every status colour with its soft wash, and keep `forced-colors` and `prefers-reduced-motion` behaviour intact.
- **Do** keep focus visible: 3px ring or outline, offset 3px.
- **Do** keep controls compact (36px default) while nav targets stay at 44px or more.
- **Do** write "program", never "programme", in text and code alike.
- **Do** show dates as dd/mm/yy (`formatDate`, `formatDateTime` with a 24-hour time, `formatCalendarDay` for chart buckets).
- **Do** cut text that adds nothing: no subtitles that restate a title, no section headings that restate the page, no "yet" in empty states ("No notes from the interviewee."), and remove the space they leave.
- **Do** reuse a component from the inventory, or extend it, before writing new markup.

### Don't:

- **Don't** introduce a brand hue, gradients, or decorative colour. Colour means status.
- **Don't** put shadows on cards, tables or other resting surfaces. Only floating layers get shadows.
- **Don't** add a second typeface or uppercase letter-spaced eyebrows.
- **Don't** use solid red blocks for destructive actions. Use the red-on-wash tint.
- **Don't** use the middle dot (`·`) as a separator, in UI text, labels, titles, or strings. Run-on `a · b · c` lines are hard to scan. Give each fact its own labelled field (a small muted label above a bold value), put it on its own line, or rewrite as a sentence. The one exception is a list of like values in a single field, such as a mock interview's round scores ("4/10 · 6/10 · 8/10").
- **Don't** hard-code practice goals or other program settings; read them from the API (`usePracticeGoals`).
- **Don't** use a raw `Badge`, `ring-foreground/10`, a native date input, or a hand-written status colour.
