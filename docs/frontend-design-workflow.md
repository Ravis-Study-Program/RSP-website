# Frontend design workflow

The web app's design rules live in [`apps/web/DESIGN.md`](../apps/web/DESIGN.md)
(visual system) and [`apps/web/PRODUCT.md`](../apps/web/PRODUCT.md) (users,
terminology, constraints). Both are read by the
[Impeccable](https://impeccable.style) skill, which is installed for this
project in `.agents/skills/impeccable` and works with any coding agent that
loads project skills (omp, Claude Code, Codex, Cursor and others).

## Live edit any part of the frontend

Live mode lets you pick an element in the running app, ask for a change, and
choose between generated variants in the browser. Accepted changes are
written back to the source files.

1. Start the stack: `just dev` (or `docker compose up -d`) and open the app
   at your `PUBLIC_ORIGIN` (default <http://localhost:8080>). Sign in with a
   fake account from `just fake-data` if needed.
2. In your coding agent, from the repository root, run:
   - omp: `/skill:impeccable live`
   - Claude Code: `/impeccable live`
   - other agents: ask it to "run impeccable live".
3. In the browser, use the Impeccable toolbar at the bottom of the page:
   pick an element with the crosshair, choose an action (or type what you
   want), preview the variants, adjust any knobs, then **Accept** or
   **Discard**. The Steer box takes page-wide instructions.
4. Stop with the toolbar's **×** or by telling the agent to stop. This removes
   the picker script it injected into `apps/web/index.html`.

Notes:

- The `web` container mounts `apps/web/index.html` so the injected picker
  script reaches Vite; the agent and the helper (`localhost:8400`) run on the
  host. Configuration is in `apps/web/.impeccable/live/config.json`.
- Never commit `apps/web/index.html` while live mode is running. If a session
  crashed, run `.agents/skills/impeccable/scripts/impeccable live-inject --remove`.
- Variants follow `DESIGN.md`. To change the visual system itself, update
  `DESIGN.md` first.

## Other commands

Run `/skill:impeccable` (omp) or `/impeccable` (Claude Code) with no argument
for a menu. Useful ones: `audit`, `critique`, `polish`, `layout`, `typeset`,
`clarify`, `adapt` with a file or route, e.g.
`/skill:impeccable polish apps/web/src/pages/DashboardPage.tsx`.

## Automatic checks

- CI runs `impeccable detect src` in `apps/web` (pattern scan of source files).
- In Claude Code and Codex, a hook runs the detector after UI edits
  (`impeccable hooks status` shows its state).
