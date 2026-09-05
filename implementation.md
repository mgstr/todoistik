# todoistik — implementation

The technical decisions behind [design.md](design.md). That document says what the app does and why; this one says what it is built out of. It records decisions, not behavior — when the two disagree, design.md wins.

## Platform

A **self-hosted web app**: one server process serving the UI, the capture API and the read API, running on an always-on machine (home server or VPS) so that it is reachable from a phone at any time.

- single process, single user. No accounts, no multi-tenancy
- the phone requirement is what settles this: capture from a share sheet has to land somewhere that is up when the capture happens. Lazy firing and idempotent capture already tolerate the *app* being away; nothing tolerates the capture endpoint being away
- the UI is a website in whatever browser is at hand, so there is nothing to install per device

## Storage

**SQLite**, one database file, WAL mode.

- a transactional store is what the audit log and the capture API need, and single-file is what a single-user app deserves: backup is copying one file
- the views are queries by design (see design.md, "Views"), and SQL is the natural home for queries. Stalled, next, overdue — all derived at read time, never stored
- the audit log is a table like any other. Recoverability means an audit entry carries a snapshot of the item as it was, not just the fact that something happened

## Stack

**Go**, server-rendered HTML with **HTMX**, and a small amount of vanilla JS for the keyboard layer.

- one static binary makes the self-hosted platform cheap to operate: copy it to the server and run it, nothing else installed
- server-rendered pages keep the app fast and the client thin. HTMX covers the in-place updates a working view needs (completing an action, toggling a filter) without a frontend framework
- no ORM ceremony required — the schema is small and the queries are the views
- JS exists for exactly one job: the keyboard layer. Everything it triggers is a request the server answers, so the server stays the single source of truth

## API authentication

A **static bearer token**, one long-lived secret, checked on every request.

- one token, set in the server's configuration, sent as `Authorization: Bearer <token>`. It covers the capture API, the read API and the UI alike — the UI accepts it once and keeps it in a cookie
- trivial for every intended caller: a curl one-liner, a share-sheet shortcut, a cron script, an AI reading a view
- this is a single-user app; scoped tokens, rotation machinery and OAuth are complexity with no second user to justify them. If the token leaks, change it — every caller is yours

The endpoints, matching design.md exactly:

- `POST /api/capture` — text payload in, an inbox item out. Replies distinctly for **accepted** and **duplicate**, so a script can tell the two apart (see design.md, "Duplicate captures")
- `GET /api/view/<name>` — one endpoint per view, returning the view as JSON. The caller passes its own filters as query parameters (e.g. `?context=home&tag=car&name=tyre`) — exactly the filters that view offers on screen, same semantics, nothing more. No parameters returns the complete view. Independent of the UI's filter state in both directions (see design.md, "The read API")

Nothing else. The read API is read only, and capture is the only way in.

## Keyboard

**Vim-style keys.** The UI is fully drivable without a mouse, and the frequent operations are single keystrokes:

- `j` / `k` move through the current list, `Enter` opens the selected item
- single-key commands act on the selection. Complete and pick-for-today are built; snooze, edit, tag and park/unpark are wanted and not yet built. The map is settled a view at a time as each is worked on, rather than declared up front
- `g`-prefixed jumps switch views, Vimium-style — see "Navigation" for the overlay and the exact letters — which is what makes "Next actions one keystroke away" (design.md, "Today") literally true
- `q`, and `g g` alongside the view jumps, open the capture dialog — see "Capture"
- `?` shows the full key map as an overlay. Together with the key bar (see "Screen layout") that is the whole discoverability story — no command palette, one way to do each thing
- **the panel lists implemented keys only.** It once carried three that did not exist (mark next, park, delete), left behind from a plan for them. That is worse than listing nothing: a key map is read as a promise, and a key that does nothing when pressed reads as a broken app rather than an unbuilt feature. A key earns its line when it works
- `/` toggles the filter panel open (see "Interface density") and focuses the name box; filters stay reachable and resettable from the keyboard, as design.md requires

## Capture

Capture is a dialog summoned on demand, not a control that is always present.
The first working version kept a text box in the nav bar of every page: it held
a corner of the chrome on all 13 views for something used a handful of times a
day, which is the same permanently-open-control problem as the filter panels
(see "Interface density").

- **three ways in, one dialog**: `q` and `g g` from the keyboard, and a `+`
  before the Inbox view's title for the mouse. The `+` is on Inbox alone —
  that is where the item lands, so that is where a mouse reaches for it —
  while both keys work from every view
- **the field is empty and unlabelled**: one large text box, no placeholder.
  Design.md's "Capture costs nothing" is about not demanding a decision; there
  is nothing to decide here, so there is nothing to read before typing
- **`Enter` adds, `Esc` discards**, and the dialog closes either way. Adding
  returns to the view you were on rather than to the inbox: a capture
  interrupts something, and should hand it straight back
- **neither an empty box nor a duplicate is an error.** `Enter` on an empty box
  closes without adding, and a text already sitting in the inbox is dropped
  silently (design.md, "Duplicate captures"). Neither is worth telling the user
  about, because in both cases what they wanted is already true. This is
  deliberately unlike `POST /api/capture`, which does report the two apart —
  a script cannot look at the inbox to see for itself (see "API authentication")
- **both keys are handled in the page's own key layer**, not left to the
  browser. A modal `<dialog>` closes itself on `Esc` and a lone text field
  submits itself on `Enter`, but those are user-agent behaviours with edge
  cases, and these two keys are the entire interaction. Centring, the backdrop
  and the focus trap are still the browser's

## Interface density

The first working version rendered every view's filter controls open, all the time, on every page — which meant scanning past a wall of checkboxes and selects to find the list itself. The fix is progressive disclosure on the filter controls, not on the item rows.

- **filter panels are collapsed by default**, one per view, expanding only on demand. Toggled by the `/` key, or a small visible control for the mouse. A collapsed panel is not the same as no panel: the controls and the persisted filter state (design.md, "Filters") are unchanged, only their visibility is
- **an item row keeps its full information** — title, context, duration, tags, due date, project, focus, parked/waiting state — shown inline, all at once. This was considered and deliberately kept as-is: density on a row is not the clutter problem, a permanently-open control panel above the list is
- **open question, not yet decided:** how a view signals it is filtered while the panel is collapsed. Design.md requires a filtered view to say so loudly and show how many items are hidden ("Views"); collapsing the panel must not quietly weaken that. To be settled in a follow-up before or alongside the collapse is implemented
- **open question, not yet decided:** the per-row controls (the complete-checkbox, the today pick-dot) were also flagged as clutter, present on every row whether or not it is about to be used. No direction chosen yet — noted here so it is not lost

## Screen layout

Three bands, borrowed from a TUI: a fixed nav line at the top, a fixed key bar
at the bottom, and the view's content scrolling between them. The chrome never
scrolls away, so which view you are in and what you can press are always on
screen, however long the list is.

- **the view's own title line is fixed too**, not just the nav — it carries the
  item count and the view's primary action (Inbox's "Process — Inbox Zero"),
  which are worth no less at item 200 than at item 1
- **the key bar is tinted away from the page colour** and separated by a rule.
  It is chrome, and must not read as the last row of the list
- **the bar offers only keys that will currently do something.** It is built
  from the page itself — is anything selected, are there rows to move through,
  does this view have a name filter — rather than from a per-view table that
  would drift from what the key layer actually does. Selecting a row adds
  open/done/today; a view with no rows never offers `j`/`k`
- **modes replace the bar rather than extending it.** While the capture dialog
  is up it reads `↵ add · esc cancel` and nothing else, because nothing else is
  reachable; the `g` overlay and the `?` panel do the same. A bar that listed
  unreachable keys would be worse than no bar
- this is the same progressive-disclosure argument as the filter panels (see
  "Interface density"), pointed the other way: the keys are always shown
  because they are always small, and always *true*

## Navigation

The nav bar lists all 13 views (design.md's "Views", plus the two implementation-level screens Audit and Settings) in one fixed order:

Inbox, Today, Next actions, Projects, Tasks, Waiting for, Calendar, Someday/Maybe, Scheduler, Review, Archive, Audit, Settings.

- **item-count badges** sit next to a view's label, for every view except **Archive**, **Audit** and **Settings** — those three are not open loops to work through, so a running count adds nothing actionable
- **a badge is omitted entirely when its count is 0**, never shown as a bare "0". A wall of empty badges is exactly the noise a badge exists to cut through
- **Inbox is the one exception to how the signal is carried**: when its count is non-zero, the nav *label itself* changes color, not just its badge. Design.md treats a non-empty inbox as the one state with a non-negotiable response ("Inbox Zero" run "regularly, and always as part of the weekly review"), so it gets a stronger signal than a small badge can give it

#### Keyboard view-jump overlay

Vimium-style. Pressing `g` overlays a one-letter tag near the top-left corner of every nav view's label; pressing that letter jumps to the view; `Esc` clears the overlay without navigating. Letters are unique across all 13 views, the view's own first letter where it is free, otherwise a distinct fallback:

| View | Key | View | Key |
|---|---|---|---|
| Inbox | `I` | Someday/Maybe | `S` |
| Today | `T` | Scheduler | `H` |
| Next actions | `N` | Review | `R` |
| Projects | `P` | Archive | `A` |
| Tasks | `K` | Audit | `U` |
| Waiting for | `W` | Settings | `E` |
| Calendar | `C` | | |

`g g` is the one `g` sequence that does not jump to a view: it opens the
capture dialog (see "Capture"). On the Inbox view the `+` control carries a
`G` tag of its own while the overlay is up, so the sequence is discoverable
the same way the jumps are.
