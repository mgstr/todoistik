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
- **a screen may declare keys on its own controls**, with `data-key` and
  `data-key-label` on the form, link or button the key presses. A declared key
  may ask for ctrl, written `^a` — the notation the bar already uses for
  ctrl-enter — and then it is live inside a text box too, which a bare letter
  can never be. That is what the modifier is for and the only reason to spend
  one: a screen whose controls sit around a form has to be reachable without
  leaving the field. Ctrl and not cmd, because cmd-a is select-all in every box
  on this machine and a screen key must not take that away. The key layer reads
  those off the page: the bar lists them in document order, and pressing one
  does exactly what clicking the control does — submit that form, follow that
  link. Nothing in the JS knows what any of them mean. This is the same
  construction as the row keys and buys the same guarantee, that a key cannot
  be advertised without working, extended to a screen whose controls are not
  rows. A declared key beats the standing map while that screen is up, which is
  what lets `t` mean trash on the processing screen and today everywhere else
- `ctrl-t` shows or hides the ages on every list, app-wide — see "Ages are
  hidden by default". The one key here that sets a flag rather than doing
  something, which is why the bar reads its state back out in the corner
- `ctrl-enter` submits the form being typed in — see "The meta line"
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

- **empty, it says "Inbox zero." and stops.** It used to add *"Nothing to decide
  about."*, which is the same sentence twice: the second half explains the first
  to someone who did not write the spec, and there is no such person here — the
  argument that leaves the write boxes without placeholders (see "The meta
  line")

- **`p` processes the selected item**, at `/process?item=<id>&one=1`, and
  returns to the list afterwards. **`z` runs Inbox Zero**, at `/process`, which
  takes the oldest item, comes back for the next one after each answer, and
  ends on the done screen. `z` is exactly `p` repeated: the same screen, fed the
  oldest item instead of the selected one. **`g z` is that same run from
  anywhere**, without stopping at the list on the way (see "Keyboard view-jump
  overlay")
- **one flag distinguishes them**, `?one=1`, carried on the screen's own URL and
  on the branch form's action. Without it the run carries on to `/process`; with
  it the answer goes back to `/inbox`. The screen itself is the same either way,
  which is what keeps `z` from being a second implementation of processing.
  The flag is stated rather than inferred from `item` being present, because
  stage two pins the item on the URL even during a run (see "Stage two") — and
  an inference that holds everywhere except one screen is worse than a
  parameter
- **`esc` leaves the screen**, back to the inbox, identically whether you got
  there by `p` or by `z`. Nothing is written on the way out and the item stays
  exactly where it was, so abandoning a run costs only the run. It is the same
  principle as processing out of order — an answer forced out of someone who
  does not have one yet is a wrong answer, not a decided item (design.md, "The
  protocol is followed, not enforced"). Processing a
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

## The processing screen

One captured line, and a menu of answers to "what is it?". The first working
version put the question in a heading, the run's position in a crumb above it,
and all eight branches on screen at once — three buttons and five forms in
`<details>`, every field of every branch one click from being visible.

- **the screen carries no prose.** The heading *"What is it?"*, the crumb
  (*"Inbox Zero · 4 left"*, *"Processing one inbox item"*) and the branch
  explanations were all removed. They are correct and they are read once: this
  is a screen worked through many times a day, and a sentence you have already
  learned is noise on the hundredth pass. What the screen is for is carried by
  the `?` panel, which is where every view's explanation lives (see "View
  help"), and the branch buttons keep their one-line `title` for the two that
  are not self-evident
- **the captured line is marked with an accent rule**, not boxed and not merely
  set large. Every control on the screen is a bordered, filled button, so
  unboxed text loses to its own menu; a card would have been a fifth boxed
  thing and would read as a control itself. A quotation rule marks the subject
  without enclosing it, holds a three-line capture as well as a one-line one,
  and spends an accent that nothing else on this screen is using. Chosen from
  five treatments in `research/process-subject-study.html`, which also settles
  why no *"N left"* came back with it
- **deciding and describing are two stages.** The question a branch answers is
  *what is this*, and that is one click. Everything a branch then needs — a
  title in valid form, a context, a definition of done — belongs to a second
  step, after the answer is given. The old screen mixed them: choosing "Project"
  and filling in a project were the same act, so the cost of *considering* a
  branch was reading its whole form. Stage two is not built yet; the branches
  that need one are the actionable ones, and they are the part still being
  designed
- **the branches are grouped into rows by what the answer costs**, one row per
  group, and the grouping is the only structure the screen has left now that
  the prose is gone:
  - **nothing changes but the audit log** — Trash, Reference material,
    Two-minute rule. The item leaves and no new object is created; the record
    that it existed is the audit entry
  - **it moves to a list, still raw** — Someday/Maybe from the inbox, Keep
    incubating for an item already there
  - **it is actionable** — Action and Project, the only two answers in the row
    and the only two that open a second stage (see "Stage two"). They carried
    *"— a step"* and *"— an outcome"* while the row was new; the gloss was
    removed once it had been read, on the same argument as the rest of the
    screen's prose. The distinction they name is in the `?` panel, which is
    where a thing that has to be explained belongs (see "View help")

  A link wearing `.button` is a button and looks like one to the pixel: two of
  these six answers navigate rather than post, and that is an implementation
  detail no one should be able to see. The class carries the same fill, hover
  and metrics as the element
- **Someday/Maybe is one click and carries the text as it stands.** design.md
  allows the text to be reworded and a `snoozeUntil` to be set at this point,
  and both were fields on the old form. Both are still reachable, on the
  someday item's own page (`/somedayitem/{id}`) — which is where you are sent
  by the item you just filed, and where you would edit it anyway a week later.
  Making them optional fields *here* charged every filing for a rewording that
  is usually not wanted
- **Keep incubating keeps its date box**, and is the one branch that is not a
  bare button. The branch *is* the new date (design.md, "Inbox Zero"), so a
  one-click version would either set nothing or silently clear the snooze the
  item already had. It is a candidate for stage two once stage two exists

### The keys

Six, one per answer, listed in the bar in the order the rows present them:
`t` trash, `r` reference, `2` two-minute, `s` someday, `a` action, `p` project,
then `esc`. Each is the branch's own first letter except the two-minute rule,
which is the rule's own number — `c` would have matched "done" elsewhere in the
app, but there `c` completes an action that exists, and this branch records
something done that never became one.

- **`t` is trash here and "pick for today" on the list views that offer the
  mark** (every one but "Out of time" — see "Item lines"), and that was
  chosen with the collision in view rather than around it. Every other branch
  gets its initial, and breaking the pattern for one of them costs more than
  the collision does: the screens are disjoint, the bar names the key on both,
  and trashing is recoverable from the audit log by recapturing (design.md,
  "Audit entry"). Worth revisiting if it ever fires by accident
- **there is no confirmation on `t`**, for the same reason there is none
  anywhere else — see design.md, "The protocol is followed, not enforced". The
  answer is recorded and recoverable, and a modal on the one screen worked
  hardest would be paid on every pass to protect against a rare slip
- **Keep incubating has no key**, alone among the branches. That branch *is*
  the new snooze date it carries, so a keystroke would submit whatever the date
  box happens to hold — empty, unless touched, which silently clears a snooze
  the item already had. It waits for a stage two that can ask for the date
- **stage two's action form takes no keys of its own.** Your hands are in a
  text field there and the key layer stands down while you are typing, which is
  correct. What the browser already gives is enough: `Enter` submits, `esc`
  blurs the field and a second `esc` goes back to stage one. **The project form
  is the exception**, because it grew a list: `a` adds an action and the row
  keys act on the one selected (see "Writing a project"). They are live for the
  same reason they are live anywhere — the moment your hands leave a field
- **the form opens with the title focused and the caret at its end.** The
  common answer by a wide margin is an action you will do yourself, standalone,
  under the wording the capture already has — and that answer should cost one
  `Enter`, not a walk through the controls that were right by default. Focus
  lands on the one field you might actually retype. `autofocus` does the focusing on
  both paths — the browser on a full load, htmx on a boosted one — but neither
  places the caret, and a pre-filled field opening at position 0 means the
  first thing typed lands in front of the text already there. `app.js` moves it
  to the end on settle: the one thing in that file which is not the keyboard
  layer, allowed because it decides nothing and stores nothing
- **the cost is paid on the way out**, and deliberately: with a field focused
  from the start, `esc` blurs before it navigates, so abandoning stage two is
  three presses rather than two. That is the right trade in a screen worked
  many times a day — the abandon path is rare and the typing path is not

## Stage two

Answering Action or Project opens a form on the same screen, at
`/process?src=&item=&as=action|project`. Server-rendered as its own page rather
than revealed in place: the second stage has to survive a reload and a back
button — it is where the typing happens — and a URL that names the stage is what
gives it that for free. It also keeps the rule that the server is the single
source of truth (see "Stack"), which a stage that only exists in the DOM would
quietly break.

- **`esc` and "back" both go to stage one**, not out of the screen. Leaving is
  still one press away from there, so abandoning costs at most two — and each
  press undoes exactly the last decision, which is what a stage-two `esc`
  landing on the inbox would not do. Nothing is written on either step
- **there is no "who does it" control on either form any more.** It was a row
  of two radios that grew a name box sideways, and it went the way every other
  field went when the meta line took them over: delegation is written
  `@waitingFor(who)`, on the action it belongs to, in the same notation
  everywhere. The project form was the last screen carrying one — its first
  action's owner is now that action's own line, like every other action's
- **the project is chosen from a picker, and the picker is the whole control.**
  Closed it shows the choice — `<standalone>` until you make one. `↓` opens the
  list of active projects, newest activity first; the rows carry the same
  `stalled` marker and open count they always did. It is built from the page
  rather than fetched, so filtering is instant and there is no endpoint behind
  it: a hundred projects is a couple of KB, and the htmx fragment this replaced
  was a round trip per keystroke to do less
- **letters filter, and the arrows move.** `j`/`k` cannot do both — they are
  letters, and project names start with them — so movement takes the form vim
  itself uses when the letters are spoken for: `↑`/`↓` and `ctrl-j`/`ctrl-k`.
  Filtering uses the app's own rule, every whitespace-separated word a
  substring in any order, so `winter car` finds *Winter-proof the car*
- **`esc` unwinds one step at a time** — the filter, then the choice — and then
  stops being the picker's key at all. Once the list is closed and nothing is
  chosen, the press is let through to the screen, or the form could not be left
  from that field
- **`c` opens the new-project dialog, and so does `enter` on `<standalone>`.**
  These are the only ways to create one from here, which is deliberate: text
  that matches nothing is a typo far more often than an intention, and the
  version this replaced turned a typo plus a definition of done into a
  duplicate project. `c` is a command only while the list is shut — an open
  list is being filtered, and every letter there belongs to the filter
- **the dialog's Create is disabled until it can be acted on**, and stays where
  it is. It first *hid* the button, which was wrong twice over: the screen
  jumps as it appears, and while it is gone nothing says that creating is what
  happens here at all. Disabled promises nothing false — it says "not yet" —
  and the key bar names what is still blank. `enter` while incomplete moves to
  the empty field rather than doing nothing, so the key is never a dead end
- **the new project is held, not created.** The dialog fills two hidden fields;
  the project and its first action are written together when the action form is
  submitted. Anything else would need a project with no actions, which
  `CreateProject` refuses and design.md argues against — and it means an
  abandoned form leaves nothing behind. Verified: cancelling the dialog and
  abandoning the form leave no empty project
- **the dialog stops both of its keys.** `esc` reaching the screen would close
  the dialog and leave the form in one press; this was a real bug, found by
  pressing it
- **there is nothing left to resolve on submit.** What is posted is an id, or a
  pending new project, or neither. The bounce that used to ask *which project
  did you mean* is gone with the text box that made the question possible
- **a form that comes back is not an error page.** It carries every value that
  was typed, the reason at the top, and the item still sitting in the inbox.
  This is the same non-answer as leaving the screen: the app asked a question it
  could not answer for you, and nothing was decided in the meantime
- **matching is on the project title only**, though the name filter over the
  Projects view also matches action titles. Right when searching for a project,
  wrong when naming the one an action should join — a stray hit on some action's
  wording would file it under a project you never named. `MatchProjects` says so
  where it is defined
- **park is not a control here.** It went with the other fields when the meta
  line took them over, and `#parked` is what writes it — refused while a project
  is being created, where every action written becomes a next action (see
  "Writing a project"), and accepted everywhere an action is written into a
  project that already exists

## Create buttons

One rule, applied wherever something is made:

- **a create button whose prerequisites are unmet is disabled, never hidden.**
  It still says that creating is what happens here and where the control is;
  hiding it moves everything below and leaves no sign the thing is possible.
  Disabled is not a control lying about what it will do — it is one saying
  "not yet", which is true and useful
- **which is why a control that can never work is absent instead.** Disabled
  means *not yet*, so it is only honest where filling something in would make
  the control work. Where the answer is *never* — removing a built-in name (see
  "The remembered lists") — the control is not drawn at all, because a dim one
  would promise a state that does not exist
- **the prerequisites are the form's own `required` fields**, so the rule needs
  no per-screen list and cannot drift from what the server will accept. A scope
  with no required fields is never gated
- **it applies to dialogs as well as forms.** A `<dialog>` is a scope like a
  form is; the button it gates is its submit button, or its primary one
- **the key bar reads the same state.** When the button can be pressed it
  offers `^↵` with the button's own words; when it cannot it says what is still
  blank, named from the field's label — *needs title and definition of done* —
  so the bar and the button never disagree and neither has to be re-checked
  against the other

## The remembered lists

The Settings page is where a name is learned and unlearned. Both lists are
shown as clouds rather than rows: a vocabulary is read as a set, and a set of
short names in a column wastes a screen saying nothing.

- **every name carries its count**, built-in ones included. The count comes
  from wherever the name actually lives, which for a built-in is a column
  rather than the tag table — `#short` counts actions whose duration is short,
  `#parked` counts open actions in a project with no `becameNextActionAt`.
  design.md says why that is worth showing rather than merely possible
- **built-in names are drawn differently and never removable**: dashed outline,
  muted, with the reason on the control. They are fields wearing a name
- **a name in use keeps its remove control, disabled; a built-in has none at
  all.** The two are different answers and are drawn differently: *not yet*,
  which the dim control explains — still carried by 3 items — and *never*,
  which no control says better than a dim one, since a dim one implies a state
  in which it would work. The dashed outline is what marks a built-in; the
  absent control is what stops you looking for the way to remove it
- **the server refuses independently.** A disabled button is a hint; the domain
  checks both conditions itself. This mattered: a structural name is carried by
  no row in `item_tags`, so the in-use check alone would have passed it and the
  delete would have quietly done nothing at all

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
- **the today-pick opens the row, beside the complete box.** It used to hold the
  right edge — first carried there by the age's `margin-left: auto`, then by its
  own. That put the one mark that answers *is this for today* as far from the
  title as a row can manage, and on a list of short titles it floated alone in
  an empty column with nothing between it and the words. In front of the
  checkbox it reads as what it is: the two things you do to a row, together, and
  a colour that scans straight down the list before any title is read
- **the "Out of time" rows carry no pick at all** — design.md, "Today" says why.
  It is dropped from the row rather than drawn disabled, and `t` goes with it:
  the key bar builds itself from the forms a row actually has, so that row
  simply never offers *today*. The rows in that group therefore start one dot
  narrower than the picked ones below them, which is left as it is — the column
  is missing because that list has no such column, and reserving the gap would
  claim a control is there and unavailable
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

### Ages are hidden by default

The flag design.md, "Views" asks for: ages off until `ctrl-t` turns them on,
one flag for the whole app.

- **`ctrl-t`, because `t` is the word and `t` is taken.** A bare `t` picks the
  selected row for today, so the flag asks for ctrl the way any declared key may
  (see "Keyboard"). That also makes it live inside a text box, which is right
  for a key that changes what the page shows rather than what is being written
- **it is a declared key on a real form**, `data-key="^t"` in the layout, so the
  key layer reaches it the way it reaches every other screen key and nothing in
  the JS knows what ages are. The form is hidden: the bar already says
  everything it would have to show
- **the bar's entry says which way the flag is set, not what the key does.**
  `ages shown` / `ages hidden`, in the right-hand group and last in it, which is
  the corner. Right, because the flag belongs to the app and not to the view;
  last, because it is the one entry whose label changes and it must not shift
  the keys beside it when it does. `data-global` on the control is what moves it
  there — the same read-the-page construction, one attribute wider
- **the class sits on the pane, not on `<body>` or `<html>`.** `hx-boost` swaps
  the body's `innerHTML`, so an attribute on either of those would still hold
  whatever it held on the first full load, and the flag would appear to stop
  working the moment you moved between views
- **one CSS rule against `.age` settles every list at once**, the "Archive" and
  the "Audit" included, where the age is closer to the record than to
  decoration. One flag with one meaning beats a flag with a list of exceptions
  nobody can remember, and it is one keypress back. The dates written into prose
  on a detail page — `captured 3 days ago`, `created … · next for …` — are not
  chips and are not touched: you went there to look
- **it is stored in `app_state` beside the per-view filter sets**, which is
  where this app already keeps remembered screen state, and the toggle writes
  the flip of what is stored rather than a value sent by the page — two presses
  in flight cannot leave the flag saying the opposite of what the last one meant

## Screen layout

A rail down the left, a fixed key bar along the bottom of what is left of the
window, and the view's content scrolling between them. The chrome never scrolls
away, so which view you are in and what you can press are always on screen,
however long the list is.

- **the nav is a rail rather than a line across the top.** Main caps its column
  at 62rem and the rail is 11.5rem wide, so on a window wider than about 76rem
  the rail costs nothing: it spends margin that was already empty. A bar across
  the top spent 3rem of height on every screen instead, and height is the axis a
  list is actually short of. Below about 76rem it becomes a trade of width for
  height, and below about 48rem it does not work at all — there is no second
  layout for that case, because design.md's "Design principles" say the app is
  used at a desk. See `research/left-nav-study.html` for the four variants and
  the arithmetic
- **the key bar sits beside the rail, not under it**, so its left edge lines up
  with the content whose keys it is naming. It answers "what can I press here",
  which is a question about what is on screen and not about where else I could
  go
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

- **the panel is also where a screen puts anything else it has to explain.**
  The description notation lives there rather than beside the box it describes
  (see "The meta line"), and that is the rule rather than the
  exception: one place per screen, reached by one key that is the same key
  everywhere. A second explanation somewhere on the page would compete with it
  and win, being nearer — and then the panel is furniture nobody opens

## Navigation

The rail opens with the `+` capture control (see "Capture"), then lists all 13 views (design.md's "Views", plus the two implementation-level screens Audit and Settings) in one fixed order, under five captions:

| | |
|---|---|
| Capture | Inbox |
| Do | Today, Next actions |
| Committed | Projects, Tasks, Waiting for, Calendar |
| Later | Someday/Maybe, Scheduler, Review |
| Records | Archive, Audit, Settings |

- **the captions say what a row could only imply.** The order inside them is the order the bar had and nothing collapses or hides: the grouping is a claim about what *kind* of place each view is, not a way to show fewer of them. It costs the height of five captions, which a column has and a row did not — a bar could only put the thirteen in a line and leave adjacency to do the work. "Records" earns its keep twice over, being the same three views that carry no count, for the same reason: they are not open loops to work through

- **item-count badges** sit at the right-hand end of a view's row, for every view except **Archive**, **Audit** and **Settings** — those three are not open loops to work through, so a running count adds nothing actionable. Outlined in the badge palette rather than filled with a colour: the nav already spends red on "the inbox needs emptying" and the accent on "this is the view you are on", and ten filled badges would spend both on something else — see `research/nav-badge-study.html` for the variants this was chosen from
- **the count sits on the row's right edge because the rail gives it one.** It used to hang off the label's top-right corner, and both halves of that reasoning were about a horizontal bar: an inline count there read as a second word in the view's *name*, and it shifted every label after it whenever the number changed. Rows one fixed width wide have neither problem, so the count can sit where a sidebar count belongs. The blanking rule went with it: the jump letters used to land in the same strip of space and had to be given it, and now they have a gutter of their own (see "Keyboard view-jump overlay")
- **where a row is both the current view and the alert, the alert wins the badge.** Standing on the Inbox is not the same as having emptied it, so its count stays red-on-white rather than turning accent — the label already resolves this way, and a badge disagreeing with the label beside it would be saying two things at once
- **a badge is omitted entirely when its count is 0**, never shown as a bare "0". A wall of empty badges is exactly the noise a badge exists to cut through
- **Inbox is the one exception to how the signal is carried**: when its count is non-zero, the nav *label itself* changes color, not just its badge. Design.md treats a non-empty inbox as the one state with a non-negotiable response ("Inbox Zero" run "regularly, and always as part of the weekly review"), so it gets a stronger signal than a small badge can give it
- **while the processing screen is up, the slot it was reached from reads "Processing…"** — the Inbox's for an Inbox Zero run or for a single picked item, the Someday/Maybe one for an item you decided to move on (design.md, "Inbox Zero"). The screen has no nav entry of its own and gets none: it is reached only from a list, and a fourteenth permanent entry for a mode you are either in or not would be furniture that is wrong most of the time. Saying nothing was worse though — the nav marked you as being *on* the Inbox while no inbox was on screen, and marked the Inbox even when the item being processed came from Someday/Maybe. A label the mode borrows costs no space and puts the phase in the one place that already answers "where am I"
- **that slot drops its badge and its red for as long as it reads "Processing…"**. The count means the inbox needs emptying and the red says it loudly (see the exception above); both are answered by the fact that you are emptying it at that moment, and an alarm about the thing you are currently doing is noise. Nothing else carries the number at the moment either: the line that read *"Inbox Zero · N left"* was removed with the rest of the screen's prose (see "The processing screen"), so a run currently counts down invisibly. Whether it comes back, and where, is the open question in `research/process-subject-study.html` — and "nowhere" is a live answer, because a count you cannot see is also a count you cannot be discouraged by. The slot stays a link with its `g i` intact: `esc` is the way out (see "Processing from the Inbox") and the nav must not be the one route that quietly stops working

#### Keyboard view-jump overlay

Vimium-style. Pressing `g` overlays a one-letter tag in the left gutter of every nav row — a strip the rail keeps permanently empty for it, so nothing has to move or blank to make room, which is what the top bar had to do to its counts; pressing that letter jumps to the view; `Esc` clears the overlay without navigating. Letters are unique across all 13 views, the view's own first letter where it is free, otherwise a distinct fallback:

| View | Key | View | Key |
|---|---|---|---|
| Inbox | `I` | Someday/Maybe | `S` |
| Today | `T` | Scheduler | `H` |
| Next actions | `N` | Review | `R` |
| Projects | `P` | Archive | `A` |
| Tasks | `K` | Audit | `U` |
| Waiting for | `W` | Settings | `E` |
| Calendar | `C` | | |

Two `g` sequences do not jump to a view:

- **`g g` opens the capture dialog** (see "Capture"). The `+` at the head of the
  nav carries a `G` tag of its own while the overlay is up, so the sequence is
  discoverable in the same glance as the jumps, on every view
- **`g z` starts an Inbox Zero run**, at `/process` — exactly `g i` then `z`,
  which is the pair pressed most often, the inbox being the one list the app
  asks to be emptied regularly and always at the weekly review (design.md,
  "Inbox Zero"). It gets no tag of its own, because there is nothing on screen
  to pin one to: the processing screen has no nav entry and gets none (see
  "Navigation"), and the Inbox slot is already wearing `I`. **The key bar
  carries it instead**, beside *"press a marked key"* while the overlay is up,
  and only while the inbox is non-empty — so the bar still never offers a key
  with nothing to do. That state is read off the nav's own alert, which is on
  every page

## The meta line

An action's form is a title, a project, a meta line and a description
(design.md, "Writing an action"). `internal/app/tokens.go` is the codec between
what is written on that line and the columns behind it; the description goes to
its column exactly as typed and is never read.

- **the meta line sits under the title, above the description.** On the
  processing screen the project control keeps its place between them: the first
  two fields are what this is and where it lives, which are the two decisions
  being made there, and metadata is not one of them
- **the box is set in the monospace face the notation is set in everywhere
  else** — the `?` panel's terms, the audit log. It also stops `@home` and
  `#short` reading as words in a sentence, which is half of what they were
  doing wrong inside the description
- **the columns stay the truth; the text is parsed into them and rendered back
  out of them**, not the other way around. The app changes those fields from
  outside the box — picking for today, a detach stamping a parked action, a
  delegation restamping the clock — and if the text owned them, every one of
  those would have to rewrite prose. This way each is a column update, and the
  box shows the new truth next time it is opened
- **`#parked` is derived, never stored**: it is "inside a project, with no
  `becameNextActionAt`". So it appears and disappears on its own when an action
  is attached or detached, with nothing to keep in step
- **`#today` and `#parked` are applied by comparison, not written over.** Both
  sit behind existing operations (`ToggleTag`, `SetNext`) rather than being
  fields on `ActionFields`, because writing them over would restamp a clock
  nothing asked to restamp — `SetNext` deliberately keeps the original stamp
  when an action is already next
- **a token has to start a word and carry a known name.** The word boundary
  alone already excludes `andres@home.example`; the vocabulary check excludes
  `invoice #12345` and everything else. They are design.md's anti-drift rule
  rather than a new invention — what has changed is what happens to the
  remainder
- **what the parser did not take is refused, and this is the one rule the split
  moved.** `ParseMeta` hands back a leftover and treats a non-empty one as an
  error, so nothing is saved. While prose and notation shared a box the
  leftover *was* the description, so an unknown name needed no answer; now
  there is nowhere for it to go that is not a lie. `parseTokens` stays
  underneath, returning what it could not read, because the round-trip test
  needs to look at a leftover without the refusal in the way
- **an unknown name still cannot be created from here.** design.md, "Contexts",
  says the app should offer to add it behind a confirm; it does not, and never
  did — `settingsAdd` is the only place a name is learned. The refusal names
  what it did not recognise, which is as far as this goes for now
- **dates use a third notation**, `due:2026-09-20` and `snooze:2026-09-20`.
  Neither is a name off a list, so neither is an `@` or a `#`; spelling the key
  out keeps them readable without a fourth sigil to learn
- **a project's line goes through the same codec, narrowed.** `ParseProjectMeta`
  runs the same parser and then refuses, by name, everything a project does not
  have — a context, a size, a due date, `@waitingFor`, `#focus`, `#today`,
  `#parked`. Narrowing after the fact rather than writing a second parser is
  what stops a project's line becoming a second dialect of the same notation,
  and `WriteProjectMeta` goes back out through the same writer for the same
  reason
- **one definition of the fields an action is written in.** The `actionfields`
  template is used by an action's own page, the processing screen, the
  add-action dialog on a project form and the box under a project's action
  list. The project control is the only difference between them — a picker, a
  fixed name, or nothing where the screen has already answered it — because
  four copies of a form is exactly how the meta line would have ended up on
  three of them
- **the meta line has a fixed order** — context, waiting-for, size, focus,
  parked, today, tags, then the dates. It is pinned by a test, because a codec
  that reorders on every save would churn the field forever
- **neither box carries a placeholder.** This app has one user, who wrote the
  spec: there is no first pass to onboard and no stranger to reassure, so a
  line of instruction under a control is read for the hundredth time by the
  person who decided the behaviour. Explanation goes in the `?` panel, which
  is opt-in, or nowhere
- **the notation is documented in the `?` panel**, not beside the box. Extra
  explanation has one home in this app, and a fold under the field was a second
  one — a screen that answers "how does this work?" in two places has neither
  answer where you look first. The panel carries the syntax and the remembered
  names together, because a name that is not on those lists stays prose: the
  list *is* the difference between metadata and text
- **an action's own page gained a help entry to carry it.** A detail page sits
  under no view and so had no panel at all (see "View help"), which was right
  while it had nothing of its own to say — it now holds the line an action's
  metadata is written on, and that is exactly what the panel explains
- **a refused line is handed back differently in the two places it can be
  written.** The processing screen bounces: everything typed comes back with
  the reason above it, nothing is written and the item is untouched, which is
  the same non-answer as leaving. An action's own page returns the error as a
  plain 400, which is what every parse error there has always done — the form
  is one browser Back away, so it has never been worth a second render path
- **`ctrl-enter` (or `cmd-enter`) finishes whatever is being written.** Plain
  Enter cannot: in a textarea it makes a newline, and the description box is a
  textarea, so without this the one key that finishes a form is unreachable
  from the field you spend the most time in. It is general rather than a
  process-screen key — the dialog if there is one, the form otherwise — and it
  does whatever that scope's own create button does, refusing when the button
  is disabled, so the key and the button can never disagree. It is not limited
  to being pressed from inside a field either: once a screen has a list, your
  hands leave the boxes to work it, and the key still has to mean "done with
  this form" there. The scope is then whatever form the selection or the focus
  sits inside — which on a list view is no form at all, since a selected row
  there is a link row and the little complete and pick forms live inside the
  row rather than around it. So the key reaches a project's draft list and
  nothing else. The bar names the
  button rather than guessing a verb
- **inside the project picker it belongs to the form, not the picker.** Plain
  `enter` there opens the list or takes a row; `ctrl-enter` takes whatever the
  list is showing as chosen and then finishes, so what is submitted is what is
  on screen. Without the distinction the universal key meant something local
  on the one screen it is most wanted

## Writing a project

A project and its actions are created in one submit, because until that submit
there is nothing for an action to belong to — design.md will not make a project
without one. So the screen holds the actions itself, as rows of hidden fields
inside the form.

- **an action written here is a row, not a saved thing.** Three hidden fields —
  `atitle`, `ameta`, `adescription` — zipped by index on the server. Plain form
  fields rather than state held in the keyboard layer, because that is what
  makes a refused form able to hand them back: the bounce re-renders the rows
  from what was posted, and nothing typed is lost to a rejected meta line
- **the rows are built from one definition.** The server renders them from the
  `draftrow` template on a bounce; the keyboard layer clones that same template
  from a `<template>` element when an action is added. A row it had to assemble
  out of parts would be a second answer to what a row is, and the two would
  drift the first time one changed
- **the create button is gated on there being an action**, through a required
  field with no box of its own that the list keeps in step. The gate already
  reads what is missing off a form's required fields (see "Create buttons"), and
  a project's missing action is missing in exactly that sense — so `ctrl-enter`
  and the button agree here the way they agree everywhere, with no second rule
- **the add-action dialog is the processing screen's own form**, with the
  project answered: `<this project>` in the same box the picker uses, read-only,
  because there is exactly one project it could belong to and a control that
  cannot change anything should still say what the answer is
- **`#today` on one of them is applied after creation**, by index against the
  actions that came back. It is not an `ActionFields` value — the tag is the
  app's to manage (see "The meta line") — and there is no action to hang it on
  until the project exists
- **the keys are the list's, not the screen's.** `ctrl-a` adds, declared on the
  button itself like any other screen key (see "Keyboard"), so the bar offers it
  because the button is there. It takes a modifier where the processing screen's
  branch keys do not, and for a reason that is particular to this screen: this
  is a form, your hands are in one of its boxes most of the time, and a bare
  letter would be a letter there. With a row selected — which means your hands
  have left the boxes — `enter` edits it in the same dialog, `u` and `d` move
  it, `r` removes it. `ctrl-enter` still creates the project from there:
  plain `enter` belongs to the row you are pointing at, and the modifier is
  what separates finishing the form from opening the thing under the cursor. `u` is offered only
  when there is something above and `d` only when there is something below —
  the bar cannot advertise a key that would do nothing
- **a draft row is dashed**, the way a parked badge is: it reads as a list row
  because it is one, and the dashes say that nothing about it is saved yet

## Specified, not yet built

Places where design.md states a rule the code does not yet apply. Kept as a
register so a gap reads as scheduled rather than as an oversight — a reader who
finds one of these and takes it for a bug will "fix" a decision that was made
on purpose.

- **an action's title must start with a verb and be self-descriptive**
  (design.md, "Inbox Zero", the Action branch). Today it is guidance rather
  than validation: the title field carries it as its placeholder — *"Starts
  with a verb, fully self-descriptive"* — and the only check is that a title is
  not empty. Validation is planned for the first release, once the end-to-end
  implementation is complete and improvement work begins; the wording will also
  grow into fuller help by then. The rule stays stated in design.md in the
  meantime, because it is the rule, and a spec that only describes what is
  already built is a changelog. Note that enforcing it is a deliberate,
  named exception to "The protocol is followed, not enforced" and not an
  oversight in that goal — design.md says why the exception holds and what
  would have to be true of a second one
- **a name can only be added in Settings.** design.md says a name that matches
  nothing should be offered for creation where it was written, behind an
  explicit confirm — "deliberate enough to stop drift, cheap enough not to
  fight capture" (see "Contexts"). What is built is the deliberate half only:
  writing `@garage` when `garage` is unknown leaves it as text, and the name
  has to be added on the Settings page first. The confirm belongs on the action
  forms and is the next thing this needs; until it is there, the round trip
  through Settings is the cost of the anti-drift rule
- **snooze, edit, tag and park/unpark have no keys yet** (see "Keyboard").
  Complete and pick-for-today are built; the rest are settled a view at a time
  as each is worked on

## Deferred, by decision

Rough edges that were looked at, understood, and left as they are for now. Kept
so the next pass starts from the reasoning rather than rediscovering it — and
so none of them reads as something nobody noticed.

- **the Scheduler's "New schedule" button.** The Inbox's equivalent was
  replaced by keys (`p` and `z`, see "Processing from the Inbox"); the
  Scheduler's is the same shape of control and has not been through that yet
- **the list views' header is now a bare number.** Dropping the duplicated view
  name (see "Screen layout") left just the item count above the list, which
  reads oddly on its own. It was left untouched on purpose: the header count is
  the **filtered** count while the nav badge is the unfiltered one, so what to
  do with it is part of the filtering pass, not a cosmetic fix. `nav.go` says
  why the badge deliberately ignores filters
- **the Audit view.** Its rows carry the item's identity as `inbox #7`, which
  reads as a count until you know it is an id, and the view has had none of the
  attention the others have. Its help line and its row layout are both first
  drafts

## Schema changes

There are no migration files and no version number. `migrate()` runs the
`CREATE TABLE IF NOT EXISTS` schema, then a short list of steps each written to
be a no-op the second time it runs — a column dropped only if `pragma_table_info`
still reports it, a value rewritten only where the old value is still there.

- **that is enough because there is one database.** A numbered migration table
  earns its keep when you cannot see every deployment; here there is one file
  on one machine, and a step that checks the database rather than a version
  counter cannot get out of step with it
- **the audit log is not rewritten, ever.** Its snapshots are JSON of the item
  as it was, so a field dropped from the schema is still there, in the entries
  written while it existed — which is the same property design.md relies on for
  recovering a trashed item, applied to a dropped field. Migrating out the
  project description did not destroy a single one; they are all still readable
  in `audit_log.snapshot`
- **`ALTER TABLE ... DROP COLUMN`** is used directly rather than the
  rename-copy-drop dance. SQLite has supported it since 3.35 and the driver is
  current

## Wanted, not specified

A third list, `todo.md`, and it is deliberately unlike the two registers above.
Those two are about the gap between this repo's documents and its code:
"Specified, not yet built" is a rule design.md states and the code does not yet
apply, and "Deferred, by decision" is a rough edge that was looked at and left.
Both are commitments — something is owed, and the entry says what.

`todo.md` owes nothing. It holds ideas for after the MVP — a verb checker, AI
help during processing, a TUI, light mode, themes, localization, configuration
in a file. None of them has been designed, argued for or promised, and design.md
is deliberately silent on all of them: writing a rule for something nobody has
decided to build would make the spec a wish list, and a spec that cannot be
trusted to describe the app is worse than a short one.

The point of keeping the three apart is that they are read differently. An entry
in a register is a thing to finish; a line in `todo.md` is a thing to consider.
An idea graduates by leaving `todo.md` — becoming a rule in design.md, or a
decision here — not by being implemented while still sitting in it.
