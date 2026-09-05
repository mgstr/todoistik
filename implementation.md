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
- `p` processes the selected inbox item and `z` runs Inbox Zero over the whole inbox — see "Processing from the Inbox"
- `?` opens the view's own help, not a key map — the key bar carries the keys, and it carries only the ones currently live, which a static list cannot. See "View help"
- **nothing advertises a key that does not exist.** The `?` panel once listed three that were never built (mark next, park, delete), left behind from a plan for them. A key map is read as a promise, and a key that does nothing when pressed reads as a broken app rather than an unbuilt feature. The bar avoids this by construction, being derived from the page rather than written down
- `/` toggles the filter panel open (see "Interface density") and focuses the name box; filters stay reachable and resettable from the keyboard, as design.md requires

## Capture

Capture is a dialog summoned on demand, not a control that is always present.
The first working version kept a text box in the nav bar of every page: it held
a corner of the chrome on all 13 views for something used a handful of times a
day, which is the same permanently-open-control problem as the filter panels
(see "Interface density").

- **three ways in, one dialog**: `q` and `g g` from the keyboard, and a `+`
  leading the nav bar for the mouse. All three are global. The `+` first sat
  on the Inbox title, on the reasoning that a mouse reaches for the control
  where the item lands — but that made the one capture route a mouse can use
  the only one that was not global, and put a control inside a view's content
  that had nothing to do with that view. It belongs in the chrome, beside the
  other things reachable from anywhere
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

## Processing from the Inbox

The Inbox view holds a list and nothing else — no heading, no count of its own,
no button. The two things it can do are keys, and the key bar names both when
they apply.

- **`p` processes the selected item**, at `/process?item=<id>`, and returns to
  the list afterwards. **`z` runs Inbox Zero**, at `/process`, which takes the
  oldest item, comes back for the next one after each answer, and ends on the
  done screen. `z` is exactly `p` repeated: the same screen, fed the oldest item
  instead of the selected one
- **one flag distinguishes them**, `?one=1` on the branch form's action. Without
  it the run carries on to `/process`; with it the answer goes back to `/inbox`.
  The screen itself is the same either way, which is what keeps `z` from being a
  second implementation of processing
- **`esc` leaves the screen**, back to the inbox, identically whether you got
  there by `p` or by `z`. Nothing is written on the way out and the item stays
  exactly where it was, so abandoning a run costs only the run. Processing a
  someday/maybe item leaves to `/someday` instead — the screen says where it
  came from with `data-cancel`, rather than the key layer knowing
- **an `item` that is no longer in the inbox redirects to the list** rather than
  erroring. It means the item was processed already — in another tab, or by a
  back button — and the list is the honest answer to "then what?"
- the button this replaced ("Process — Inbox Zero") was the view's only control
  and sat on every visit whether or not there was anything to process. A key
  costs nothing when unused, and the bar already says when it is available
- **the bar lists them `z`, `j k`, then `p`**, and that order is the point.
  Design.md settles the run as the default way through the inbox and processing
  one picked item as a deliberate escape hatch for the case where holding the
  queue would do harm (see design.md, "Inbox Zero"). Both are one keystroke and
  neither is hidden, so the ordering is the only place the app can say which is
  which — the run first, the movement keys next, and acting on the item you
  moved to last. Views with no run to work down keep the ordinary order, acting
  on the selection first

## Interface density

The first working version rendered every view's filter controls open, all the time, on every page — which meant scanning past a wall of checkboxes and selects to find the list itself. The fix is progressive disclosure on the filter controls, not on the item rows.

- **filter panels are collapsed by default**, one per view, expanding only on demand. Toggled by the `/` key, or a small visible control for the mouse. A collapsed panel is not the same as no panel: the controls and the persisted filter state (design.md, "Filters") are unchanged, only their visibility is
- **an item row keeps its full information** — title, context, duration, tags, due date, project, focus, parked/waiting state — shown inline, all at once. This was considered and deliberately kept as-is: density on a row is not the clutter problem, a permanently-open control panel above the list is
- **open question, not yet decided:** how a view signals it is filtered while the panel is collapsed. Design.md requires a filtered view to say so loudly and show how many items are hidden ("Views"); collapsing the panel must not quietly weaken that. To be settled in a follow-up before or alongside the collapse is implemented
- **open question, not yet decided:** the per-row controls (the complete-checkbox, the today pick-dot) were also flagged as clutter, present on every row whether or not it is about to be used. No direction chosen yet — noted here so it is not lost

## Item lines

Every list in the app shares one row template, so this is one decision, not a
per-view one.

- **no rule under a row.** The age used to be pinned to the right edge, which
  needed a line under every item to carry the eye across the gap. The age sits
  beside its title now, so the line has nothing left to do, and an inbox of nine
  items stopped looking like a table with nothing in it
- **the age is a chip, not small grey text.** Grey text beside black text still
  parses as a continuation of the title — "call the dentist yesterday" reads as
  a phrase before it reads as two fields. The enclosing shape is what makes it a
  separate field, and it is the shape the app already uses for context, duration
  and tags. The cost, accepted: on a Next actions row the age is a fifth chip,
  the least important of them and the most constant — see
  `research/item-line-study.html` for the four treatments this beat
- **the today-pick keeps the right edge.** It used to be carried there by the
  age's `margin-left: auto`; now it has its own
- **a click selects a row, a double click opens it.** Selection used to be
  reachable only from `j`/`k`, which left the row keys the bar was offering
  unreachable without the keyboard — and on the Inbox, whose rows carry no link
  of their own, a mouse could not touch a row at all. Double click is the
  mouse's `Enter`: it follows the row's `data-href`, which on the Inbox means
  processing that item and elsewhere means opening it
- **controls inside a row keep their own jobs.** A click that lands on the title
  link, the complete checkbox or the today-pick does what that control does and
  does not also move the selection. Anything else in the row — a badge, the age,
  the gaps between them — selects

### Ages are written out, not coded

`3 weeks ago`, not `3w`. The scale, in days:

| Age | Reads |
|---|---|
| 0 | today |
| 1 | yesterday |
| 2–6 | `n` days ago |
| 7–13 | a week ago |
| 14–27 | `n` weeks ago |
| 28–59 | a month ago |
| 60–364 | `n` months ago |
| 365–729 | a year ago |
| 730+ | `n` years ago |

- **a month is 30 days and a year is 365.** Calendar-accurate arithmetic would
  make "2 months ago" cover different spans in different seasons, for no gain on
  a label that is approximate by design
- **two boundaries exist only to close gaps**, and both are easy to reintroduce
  by accident. The `n` days range starts at **2**, because "yesterday" covers
  only day 1. "A month ago" runs to **59** rather than to eight weeks, so it
  ends exactly where "2 months ago" begins. The month count also stops at 11:
  360 days is twelve thirty-day months but not yet a year
- every boundary is pinned in `internal/web/age_test.go`, which is the whole
  reason that file exists

## Screen layout

Three bands, borrowed from a TUI: a fixed nav line at the top, a fixed key bar
at the bottom, and the view's content scrolling between them. The chrome never
scrolls away, so which view you are in and what you can press are always on
screen, however long the list is.

- **the view's header line is fixed too**, not just the nav — it carries the
  item count and the view's primary action (Inbox's "Process — Inbox Zero"),
  which are worth no less at item 200 than at item 1. It no longer carries the
  view's *name*: the nav already says which view you are in, and saying it
  twice on every screen buys nothing. The name lives in the `?` panel now,
  which is also the only place the full name appears where the nav abbreviates
  it — "Next actions" for "Next", "Someday/Maybe" for "Someday"
- **the key bar is tinted away from the page colour** and separated by a rule.
  It is chrome, and must not read as the last row of the list
- **the bar offers only keys that will currently do something.** It is built
  from the page itself — is anything selected, are there rows to move through,
  does this view have a name filter — rather than from a per-view table that
  would drift from what the key layer actually does. Selecting a row adds
  open/done/today; a view with no rows never offers `j`/`k`
- **two groups, held apart by alignment**: the keys this view offers sit left,
  the three that work everywhere — `q`, `g`, `?` — sit right, both aligned to
  the content column rather than the window edge. The right half is then fixed
  furniture: only the left half has to be re-read when the view or the
  selection changes. A thin rule divides them, and disappears when the view
  has no keys of its own
- **modes replace the bar rather than extending it.** While the capture dialog
  is up it reads `↵ add · esc cancel` and nothing else, because nothing else is
  reachable; the `g` overlay and the `?` panel do the same. A bar that listed
  unreachable keys would be worse than no bar
- this is the same progressive-disclosure argument as the filter panels (see
  "Interface density"), pointed the other way: the keys are always shown
  because they are always small, and always *true*

## View help

`?` opens one centred panel: the view's full name and a single line on what
that view is for. It replaced the global key map, which the key bar had made
redundant — and unlike the map, it says something the bar cannot.

- **it holds what the header used to say.** Nine views carried a muted line of
  explanation under their title ("the ball is not in your court; age is the
  delegation date"). That is worth having and worth reading once, not on every
  visit forever, which is what a permanent line under the title amounts to
- **the text is one line per view, keyed by nav slug** in `viewHelp`
  (internal/web/ui.go), so a detail page shows the help of the view it sits
  under. A page under no view — a single action — has no entry, gets no panel,
  and the key bar drops `?` rather than offering a key that opens nothing
- **a screen with no nav slug of its own asks for its entry by name.** The
  process screen sits under the Inbox in the nav — that is where it is reached
  from and where the highlight belongs — but it is not the Inbox, and showing
  the Inbox's help there answered a question nobody had asked. `page.help()`
  overrides what the slug picked, so the nav highlight and the help panel can
  disagree where they should
- **centred, not tucked in a corner.** It is asked for by name, so it should
  land where the eye already is. The earlier corner placement was right for a
  reference card being consulted while working, and wrong for an answer to a
  question just asked
- the four views that had no such line got one written for them (Inbox, Next
  actions, Projects, Settings), each drawn from what design.md already says
  about that view rather than invented separately — the panel must not become
  a second, quietly diverging description of the app

## Navigation

The nav bar opens with the `+` capture control (see "Capture"), then lists all 13 views (design.md's "Views", plus the two implementation-level screens Audit and Settings) in one fixed order:

Inbox, Today, Next actions, Projects, Tasks, Waiting for, Calendar, Someday/Maybe, Scheduler, Review, Archive, Audit, Settings.

- **item-count badges** sit in the top-right corner of a view's label, iPhone-style, for every view except **Archive**, **Audit** and **Settings** — those three are not open loops to work through, so a running count adds nothing actionable. Cornered rather than inline because a count on the label's own baseline reads as a second word in the view's *name*, and because a badge outside the text flow cannot shift every label after it when its number changes. Outlined in the badge palette rather than filled with a colour: the nav already spends red on "the inbox needs emptying" and the accent on "this is the view you are on", and ten filled badges would spend both on something else — see `research/nav-badge-study.html` for the variants this was chosen from
- **the counts blank while `g` is held.** The jump letters land in the same strip of space, at the neighbouring item's top-left, so the overlay gets it to itself. Blanking rather than widening the nav to fit both: during the overlay you are choosing a destination, not reading counts, and the alternative pays horizontal space always to fix something visible only while a key is down
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
capture dialog (see "Capture"). The `+` at the head of the nav carries a `G`
tag of its own while the overlay is up, so the sequence is discoverable in the
same glance as the jumps, on every view.
