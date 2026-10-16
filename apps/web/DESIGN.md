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
  night-hairline: '#383838'
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
    backgroundColor: '{colors.ink}'
    textColor: '{colors.paper-on-ink}'
    typography: '{typography.label}'
    rounded: '{rounded.pill}'
    padding: '2px 8px'
    height: '20px'
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

The mood is calm, precise and restrained. Density is that of a working tool: compact 14px controls, tables, filters and short summaries, never sprawling marketing composition. Hierarchy comes from weight, size and spacing, not colour. Colour carries meaning only for status: success, warning, danger.

Components should feel exact and unassuming. They are shadcn/ui primitives on Base UI, with gently rounded corners, hairline borders and immediate, motion-light feedback.

**Key Characteristics:**

- Monochrome ink-on-paper palette with a mirrored dark mode (`data-theme="dark"`).
- One typeface, Source Sans 3, at weights 400–700; tight negative tracking on page titles.
- Flat surfaces defined by 1px hairlines and rings. Shadows exist only on floating layers.
- Fixed left sidebar on desktop. Below 56rem it collapses to a mobile header and mobile navigation.
- Status colour (green, amber, red) always pairs with a soft tinted wash and text, and is never decorative.

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
- **Ledger Red** with **Red Wash** (`ledger-red`, `ledger-red-soft`): destructive actions, invalid fields, and errors. Destructive buttons are a red-on-wash tint, not a solid red block.

### Named Rules

**The Ink-Only Accent Rule.** There is no brand hue. Primary emphasis is ink, and any non-neutral colour on screen must answer "what status is this?"

**The Paired Wash Rule.** Status colour always appears as a text/icon colour on its own soft wash, so meaning survives in both themes and in forced-colors mode.

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

**The Sentence-Case Eyebrow Rule.** Eyebrows and labels are sentence case with normal tracking. Uppercase letter-spaced labels are not part of this system.

## Layout

- **Shell:** a fixed 15rem left sidebar (brand, workspace switcher, role-based nav, settings, and user summary at the foot) and a workspace body with a slim workspace breadcrumb header ("Workspace / season").
- **Page:** content maxes at 92rem and is centred. Main padding is `2rem clamp(1.25rem, 3vw, 3rem) 4rem`. The page header puts title, eyebrow and lede on the left and actions on the right, with a 1.5rem gap below.
- **Grids:** three-column card and metric grids drop to two columns below 70rem. Below 56rem the sidebar is hidden, a mobile header and mobile navigation appear, tables switch to stacked mobile cards, and page headers stack.
- **Rhythm:** a compact 4px-based Tailwind scale. Card padding is 16px (12px for small cards). Typical gaps are 0.5–1.25rem.
- **Floor:** the minimum width is 320px, and layouts must reflow at 200% zoom.

## Elevation & Depth

The system is flat. Surfaces sit on the page, separated by 1px hairline borders or a 1px ink ring at 10% opacity, plus tonal steps (paper → wash, or night paper → night card). Shadows appear only on layers that genuinely float above the page.

### Shadow Vocabulary

- **Float** (`box-shadow: 0 8px 24px rgb(0 0 0 / 0.18)`): popovers, comboboxes, the date-picker calendar, and timezone lists.
- **Dialog** (`box-shadow: 0 12px 32px rgb(0 0 0 / 0.22)`): modal dialogs.

### Named Rules

**The Flat Ledger Rule.** Nothing resting on the page casts a shadow. A shadow means "this is floating and will go away": menus, popovers, dialogs.

## Shapes

Corners are gently rounded and consistent. The base radius is 10px (`--radius: 0.625rem`), and other sizes derive from it: 6px for nav items, selects and small surfaces, 8px for compact controls, 10px for buttons, inputs and tab lists, and 14px for cards. Badges and avatars are full pills or circles. Borders are always 1px hairlines. Card images clip to the card's corners.

## Components

### Buttons

Exact and unassuming.

- **Shape:** gently rounded (10px). Heights are 36px by default, 28px for small and 24px for extra-small.
- **Primary:** ink fill, paper-on-ink text, 14px medium, 10px horizontal padding.
- **Hover / Focus:** hover drops the fill to 80% opacity. Focus shows a 3px pencil ring at 50%. Press nudges the button down 1px. Disabled is 50% opacity.
- **Outline / Secondary / Ghost:** outline is paper with a hairline border and washes on hover. Secondary is a wash fill. Ghost is transparent and washes on hover.
- **Destructive:** ledger red on a 10% red tint, deepening to 20% on hover.
- **Link:** ink text, underlined on hover.

### Chips / Badges

- **Style:** 20px-tall pills, 12px medium text. Variants are ink-filled, wash, outline, and status-tinted (success, warning, danger use the paired wash). Badges stay legible in forced-colors mode.

### Cards / Containers

- **Corner Style:** 14px.
- **Background:** paper (night card in dark mode).
- **Shadow Strategy:** none (Flat Ledger Rule). The edge is a 1px ink ring at 10% opacity.
- **Internal Padding:** 16px, or 12px for the small size. Footers sit on a 50% wash with a hairline top border.

### Inputs / Fields

- **Style:** 36px tall, 10px radius, hairline stroke, transparent fill. Text is 16px on mobile and 14px from `md` up, so iOS does not zoom on focus.
- **Focus:** the border shifts to pencil and a 3px pencil ring appears at 50%.
- **Error / Disabled:** `aria-invalid` gives a red border and a 20% red ring. Disabled is 50% opacity with a wash fill.

### Navigation

- **Sidebar:** 44px-tall nav links in graphite, 15px at weight 550, with 6px radius. Hover and active states darken the text to ink on the sidebar-hover step. There is no accent bar or colour.
- **Tabs:** a wash track with a 3px inset. The active tab is a paper chip. A line variant uses a 2px ink underline.
- **Mobile:** below 56rem a mobile header (brand "RSP") and a dedicated mobile navigation replace the sidebar.
- **Command palette:** Quick navigation is available as a keyboard-first alternative.

## Do's and Don'ts

### Do:

- **Do** take emphasis from ink fill, weight (500–700) and size before reaching for colour.
- **Do** use the tokens in `src/styles/tokens.css` (`--primary`, `--muted`, `--border`, `--success`/`--success-soft` and so on) so both themes stay in sync.
- **Do** pair every status colour with its soft wash, and keep `forced-colors` and `prefers-reduced-motion` behaviour intact.
- **Do** keep focus visible: 3px ring or outline, offset 3px.
- **Do** keep controls compact (36px default) while nav targets stay at 44px or more.

### Don't:

- **Don't** introduce a brand hue, gradients, or decorative colour. Colour means status.
- **Don't** put shadows on cards, tables or other resting surfaces. Only floating layers get shadows.
- **Don't** add a second typeface or uppercase letter-spaced eyebrows.
- **Don't** use solid red blocks for destructive actions. Use the red-on-wash tint.
