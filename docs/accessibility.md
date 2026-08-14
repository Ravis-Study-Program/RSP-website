# Accessibility verification

The target is WCAG 2.2 AA across supported current Chrome, Firefox, Safari,
Edge, iOS Safari and Android Chrome. Base UI supplies headless behavior; RSP
owns focus design, contrast, labels, errors and widget-level verification.

## Automated gate

`just test` covers component interaction and `just e2e` runs desktop/mobile
Playwright workflows with axe checks. Automated checks also require:

- No unlabeled controls or duplicate IDs.
- Semantic heading/landmark order and a working skip link.
- Dialog name, focus trap, Escape behavior and focus restoration.
- Forms associating help/error text and announcing submission results.
- Tables retaining headers/meaning and mobile card fallbacks exposing the same
  data/actions.
- Charts providing a nearby textual summary, not color-only meaning.

Automated tools cannot prove keyboard order, screen-reader clarity, zoom layout
or usable focus appearance.

## Automated acceptance record — 2026-08-14

The release candidate was checked with Node 24.19.0, Playwright 1.55.1 and the
checked-in browser projects for desktop Chromium, mobile Chromium, Firefox,
desktop WebKit and iPhone-sized WebKit. The recorded results were:

- 15 Vitest files and 31 component/unit tests passed.
- 39 Playwright cases were scheduled: 24 passed and 15 were intentionally
  skipped outside the viewport/project they target.
- Desktop and mobile dashboard axe scans reported no WCAG A/AA violations.
- Keyboard skip-navigation and command-palette operation passed.
- The 320 CSS-pixel/200%-equivalent reflow test kept primary actions reachable,
  and the reduced-motion test suppressed non-essential transitions.
- Firefox, desktop WebKit and iPhone WebKit each passed the protected-shell and
  settings-form compatibility workflow.

These are repeatable automated checks, not a substitute for the manual smoke
below. The in-app browser runtime exposed no browser, but the local macOS
accessibility session was available through Computer Use. The desktop result is
recorded separately below. A release operator must still append an iOS
VoiceOver result before live cutover; this repository pass does not include
that production cutover or a physical iOS device.

## Manual desktop acceptance record — 2026-08-14

macOS 26.6.1, VoiceOver and Chrome 151.0.7922.138 were used against the
explicitly demo-mode local build on loopback. VoiceOver was off before the
check, enabled only for the screen-reader portion, and restored to off after
the check. Student and Coordinator dashboards passed the desktop navigation,
reading-order and command-dialog subset exercised here:

- Page title, current role/workspace, primary navigation, level-one heading,
  summary metrics, recommendation and chart text were exposed with meaningful
  accessible names and reading order.
- VoiceOver's read-all command moved its visible cursor to the named `RSP home`
  link on the Coordinator dashboard; the Student dashboard exposed the
  role-specific Season, Practice, Mock interviews and People navigation.
- Keyboard Tab exposed `Skip to main content` first and activation moved focus
  into the main region.
- The `Quick navigation` dialog exposed a heading, description, named close
  button, labelled search field and descriptive result links. Tab reached the
  search field, filtering announced the reduced accessible result tree, and
  Escape closed the dialog and restored page focus.
- The activity chart exposed a textual six-week summary, so its meaning did not
  depend on the drawn line or colour.

The iOS simulator service was unavailable and no physical iPhone was connected,
so iOS Safari with VoiceOver remains a required device-level release check.

## Manual release smoke

Test at least Student and Coordinator workspaces on desktop and mobile:

1. Keyboard only: skip navigation; traverse sidebar/bottom navigation; open and
   close dialogs, menus, date picker, editor toolbar and command palette; sort,
   filter, paginate and act on a table/card; submit and correct a form.
2. Screen reader: announce page title, workspace/role, loading, empty,
   filter-empty, inline error, stale-data retry, toast/live result, dialog and
   destructive confirmation. Test one desktop reader/browser pair and VoiceOver
   on iOS Safari.
3. Zoom/reflow: 200% browser zoom and a 320 CSS-pixel viewport without clipped
   content, two-dimensional page scrolling or unreachable actions.
4. Visual: focus indicator remains visible against light/dark surfaces; text,
   controls, status badges and charts meet contrast; state never relies only on
   color.
5. Motion: `prefers-reduced-motion` removes non-essential animation and no
   flashing content is introduced.
6. Touch: mobile targets and spacing remain usable without hover; full-screen
   forms return focus/context after close.

## High-risk widgets

- `DataTable`: header/action names, multi-sort announcement, filter labels,
  loading/error/empty distinction, fullscreen exit and equivalent mobile cards.
- Rich-text editor: toolbar pressed state, keyboard reachability, link dialog,
  pasted-content sanitization and a useful editor name/instructions.
- Date picker: typed and calendar input, format help, invalid-date error, month
  navigation and locale/timezone boundary.
- Destructive dialogs: resource name in the prompt, non-destructive initial
  focus and no generic “Are you sure?” confirmation.
- Charts: textual summary/table for values/trend and hidden decorative SVG
  internals where appropriate.

Record browser/device, assistive technology/version, route/workspace, result and
issue link. An accessibility exception needs product/security approval, user
impact, workaround, owner and expiry; it cannot be waived by an automated axe
pass.
