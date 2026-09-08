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
- **the selection survives acting on the row.** A row key posts a form and the
  answer is a whole new page — boosted or not, the list is rebuilt and the
  class marking the selection goes with the old one, so pressing `t` used to
  end with the cursor gone and `j` pressed to get back to the row you were
  already on. The row is handed to the page the response renders and claimed
  once, on arrival. Three rules keep that from selecting things nobody pointed
  at: it is claimed only on the screen it was handed from — which is read off
  the nav's own highlight, since on a boosted post the new page is in the DOM
  before htmx has finished with the URL — only for a row that was selected when
  the key was pressed, and only once, so a later `g` jump never arrives with
  something already selected. When the row itself is gone, which is what `c`
  does to it, the selection stays at that *position* instead: the item that
  moved up is under the cursor and a list can be worked straight down. This is
  the one thing the key layer keeps across a page load, and it keeps it in
  `sessionStorage` rather than on the server, because it decides nothing and
  survives nothing — losing it costs a keystroke (see "Stack")
- single-key commands act on the selection. Complete, pick-for-today and doing are built; snooze, edit, tag and park/unpark are wanted and not yet built. The map is settled a view at a time as each is worked on, rather than declared up front
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
- `d` opens the selected action alone on a screen of its own — see "Doing"
- `ctrl-v` opens the panel chooser: title bar, navigation, key bar, zen mode,
  one letter each, and a second `ctrl-v` presses zen — see "Panels"
- `ctrl-t` is *show me the time*: the ages on every list, app-wide (see "Ages
  are hidden by default"), and the timer on the doing screen (see "Doing"). The
  one key that sets a flag rather than doing something, which is why the bar
  reads its state back out rather than naming an action. Two flags and one key,
  because the screen with a timer on it has no ages and every screen with ages
  has no timer — they can never both want it at once, and the doing screen
  renders no ages control at all so the collision cannot even be built
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

### The Next view's controls, rebuilt one at a time

The panel of checkboxes and selects that this section is about was widest on
the one screen the app is actually used from, and it sat between the nav and
the list on every visit. It came off that page whole, and what belongs there
is going back one control at a time. The context filter is the first, and so
far the only one.

- **taking the form off changed no filtering.** The per-view filter set still
  persists in `app_state`, `/next?context=home&tag=car` still narrows the page,
  and the read API never saw the screen's state anyway (design.md, "The read
  API"). Nothing about what the view *returns* moved with the furniture
- **contexts are radios and tags are checkboxes**, which is the difference
  between the two filters made visible: an action has one context and any
  number of tags (design.md, "Filtering by context" and "Filtering by tag").
  Both live in one `<form method="get">` with `data-autosubmit`, so choosing
  either keeps the other and there is no Apply, and the `f=1` marker on it is
  what saves the pair as this view's filter set
- **"all" is a radio with an empty value**, which `parseFilters` drops, so
  turning the context filter off is the same act as turning it on and there is
  no second control to find. Tags have no such answer — several can be on at
  once, so "none of them" is not one of the choices — which is why they get a
  clear button of their own
- **the rows are a two-column grid**, label then chips, so "Context:" and
  "Tags:" line up and the chips of both start at the same place. The labels are
  the only words on these rows: a row of `@`s and a row of `#`s say what they
  are, but which is which is worth one word each when they sit under one
  another
- **the clear button unchecks and submits**, rather than being a link to the
  same page without the tag parameters. It is what it says it is — the boxes
  are unchecked in front of you — and unchecking a box from script fires no
  change event, so the submit is explicit. It is disabled while there is
  nothing to clear, the same argument as the create buttons (see "Create
  buttons"): it says what the control is for without promising something false
- **both rows are built from the view's own items**, `ContextsOf` and `TagsOf`
  over the actions the page has just loaded, each one with *its own* filter set
  aside and the others still applied. Set aside, or picking one context would
  leave one context to pick from; the others applied, because a tag with
  nothing under the context you are in is an answer that leads nowhere. A
  selected tag is added back to its row whatever the rest of the filters do to
  it (`withSelected`), or the only sign that it is on would be the list being
  short. This replaced `ContextsInUse`, which asked the database for every
  context on an open action anywhere; the tag rows on the views whose panels
  have not been rebuilt still use `TagsInUse`, which is the same wide answer —
  `page.TagCloud` is that list and `page.TagsInView` is this one
- **the other filters are still off the page**, so design.md's "every filter is
  reachable and resettable from the keyboard" is not true of this screen yet:
  the contexts answer to Tab and the arrow keys, which is the browser's doing
  and not a key of the app's, and name, tags, duration, focus and order have no
  control at all. Written down because the gap is otherwise invisible — a
  filter set left on from before narrows the view with nothing on the page
  offering to turn it off
- **the filtered line stays**, and is load-bearing while that is true: it is
  the confession design.md, "Views" requires, and its "Reset all filters" link
  is the only way back from a filter the page cannot show
- **the header holding the count went with the form**, on the same argument:
  the working view is read by looking down it, not by being told how far it
  goes. It was the first view to lose one and every other view has since
  followed, because the title bar says the count now (see "Panels") — which is
  where this note stops being about the Next view in particular

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

## Doing

design.md, "Doing one action" asks for the selected action alone on an
otherwise empty screen. `d` opens it, `c` completes, `esc` leaves.

- **it is a page: `GET /doing/{id}`.** The first version built it in the
  browser out of the row, on the argument that it was not a view and held
  nothing the row did not — and a `/doing/{id}` route would be a second way to
  complete an action. Both halves stopped being true. Zen mode gives every view
  the bareness that was the whole point of the mode (see "Panels"), so there is
  nothing left for a mode to be; and completing here posts the *same* form to
  the same handler as a row does, project check and all, so there is one way to
  complete an action and this is a screen that uses it. What the URL buys is
  what a DOM-only mode could not have: a reload, a back button, and a name that
  `zen.views` can put in a settings file
- **`d`, and it collides with nothing.** The only other `d` in the app moves a
  draft action down its list, and a draft is an action that does not exist yet
  — there is nothing there to do. The two share the case and can never both
  apply to one row
- **the row says whether it can be done**, with `data-doing` on the shared row
  template, written only for an action that is not completed — and it now
  carries the URL rather than being a bare marker, so the key layer navigates
  to what the page said rather than assembling a route of its own. Read from
  the page like every other key, so the bar offers `d doing` exactly where it
  works. This is why the test is an attribute and not "has a complete form":
  the weekly review's rows have one too, and there `c` means *reviewed*, which
  is not what this screen would be advertising
- **where "back" goes rides on the URL**, `?from=/next`, and not on the
  Referer: the URL is the part that survives a reload, which is the whole
  reason this is a page. The key layer writes it from the path it was pressed
  on, the screen carries it as `data-cancel`, and the complete form posts it as
  a `back` field so that finishing the action lands where leaving it would —
  `back()` prefers an explicit destination to the header. A `from` that is not
  a local path falls back to `/next`
- **`c` and `esc` are declared keys, not a special case.** The complete form
  carries `data-key="c"` and the section carries `data-cancel`, so the bar
  reads `c done · esc back` off the page and the handler presses the control.
  The label on the way out is the screen's own (`data-cancel-label`), because
  "back" is what this one is
- **the app's keys keep working**, which is the visible half of the mode going
  away. It used to swallow every unmodified key; now `q`, `g`, `?` and the rest
  are live, and the bar's right-hand group is populated like anywhere else.
  Nothing here is being typed and nothing is half-written, so there was never
  anything for the swallowing to protect
- **the selection is handed back.** `d` stores the row the way a row key does
  (see "Keyboard"), and a page with no rows neither claims that handover nor
  swallows it — so `esc` lands on the list with the same row under the cursor,
  and `c` lands on it with the next one, since the row it was showing is gone
- **it opens in zen because the settings file names it**, not because the
  screen has furniture settings of its own. `doing.show_nav` and
  `doing.show_keybar` are gone: they were this one screen's private version of
  a question every screen has, and `zen.views = doing, processing` is the
  general answer (see "Settings file")
- **the timer is always ticking, and `ctrl-t` is what shows it.**
  `zen.show_timer` decides how the screen opens; the key flips it after that,
  and the bar reads back `^t timer shown` / `^t timer hidden` the way the
  global entry reads back the ages. The element exists either way, because a
  timer created on demand would start counting from the moment it was asked
  for — which is not the number anyone means by "how long have I been on this".
  The flip is a variable in the keyboard layer: it outlives every boosted
  navigation and a reload puts the settings file back in charge
- **this screen renders no ages control at all**, which is what lets `ctrl-t`
  mean one thing. Two hidden controls declaring the same key would be a race
  decided by document order; the layout skips the ages form when the page says
  it counts its own minutes (`page.Timer`), and the timer's own hidden button
  is then the only `^t` on the page. The bar picks it up as a global key
  because the button says `data-global` — nothing in the key layer knows what a
  timer is beyond flipping the element it points at
- **the timer keeps the bottom-right corner**, in the title's own size and at
  `opacity: .15` — the size says it is not a lesser kind of information, the
  opacity keeps it from being read unless it is looked for. A corner and not a
  line under the title, so the title sits exactly where it sits with no timer at
  all and nothing moves when the digits change width (`tabular-nums` finishes
  that job). It ticks on a one-second interval that writes only when the minute
  has actually turned, and the interval is cleared when the page is swapped
  away. Six placements and three opacities were rendered before this one —
  `research/doing-timer-study.html`
- **the format comes from the file and is applied in the browser.** The pattern
  rides on the pane as `data-zen-timer-format` and the key layer renders it;
  the server checks it and otherwise passes it through, which keeps the one
  place that knows what a minute looks like next to the one thing that counts
  them

## Settings file

The choices that are not items, not screen state and not worth a screen: read
once at startup from a `key = value` file (`internal/conf`).

```
# todoistik.conf
zen.views = doing, processing  # these screens open with every panel off
zen.show_timer = false         # the timer starts hidden; ctrl-t shows it
zen.timer_format = auto        # or a pattern: H:MM, HH:MM, M
```

- **one pair per line, `#` to the end of the line for comments, and nothing
  else** — no sections, no nesting, and the one list there is is written with
  commas on one line. A comment may trail a value,
  since no value this format can hold contains a `#`. A setting is then one line found by grep and rewritten in
  place, by a person or by an agent, which is the whole reason this is a file
  and not another screen
- **missing is fine, wrong is fatal.** No file at all is the ordinary case and
  gives the defaults. A file that exists and has an unknown key, a line without
  an `=`, or a value that is not `true`/`false` stops startup, naming the file,
  the line number and what was wrong. It is read exactly once, so a line quietly
  ignored would look set for as long as the process lives — the one failure this
  format can have, and the reason it is loud
- **every key is written as what you get, never as what is taken away.**
  `zen.show_timer` says when the timer is up and `zen.views` says which screens
  are bare, so no setting has to be read through a negation to know what it
  does. The panel keys are gone from this file entirely: what a screen wears is
  screen state now, remembered where screen state is remembered (see "Panels"),
  and the file only says which screens start with none of it
- **the defaults are doing and processing bare, and no timer.** Both are
  screens you are in the middle of one item on, where the rail is a list of
  other places you could be and the bar a list of other things you could press;
  and a clock on the wall is a thing you ask for, not a thing a screen for
  concentrating on one job should volunteer. Both are one line from the opposite
- **an empty list is an answer.** `zen.views =` means no screen opens bare,
  which the defaults cannot say — a key left out falls back to the default, so
  "none" has to be writable
- **`zen.views` is checked against the app's screens, but not here.** conf
  checks the shape and `internal/web` checks the names, which is where the list
  of screens lives (see "Panels"). Wrong either way still stops startup
- **a setting that takes words brings its own check.** `true`/`false` checks
  itself; a string does not, and a settings file read once at startup is exactly
  where an unchecked typo lives forever. `zen.timer_format` is either the word
  `auto` — minutes while there are only minutes, `H:MM` after that, which no
  single pattern can express — or a pattern in which uppercase `H`/`HH` is the
  hours and `M`/`MM` the minutes, within the hour when the pattern asks for
  hours and the whole elapsed time when it does not. Everything else is literal,
  so `H:MM`, `HH:MM`, `M` and `H h MM` all work. Any *other* capital is refused
  rather than printed: a capital in a pattern reads as a field, and `HH:NN`
  quietly rendering as `01:NN` is the failure this file cannot afford
- **`-config`, or `TODOISTIK_CONFIG`, defaulting to `todoistik.conf` in the
  working directory**, like every other setting the app takes. The file is
  git-ignored: it is one machine's answer, the same way the database is
- **nothing is written back to it.** Everything the app itself remembers —
  filter sets, the ages flag — lives in `app_state` in the database. A file the
  app rewrites is a file you cannot keep comments in

## Screen layout

A rail down the left, a title bar across the top of what is left of the window,
a fixed key bar along the bottom of it, and the view's content scrolling between
them. The chrome never scrolls away, so where you are and what you can press are
always on screen, however long the list is — and each of the three can be taken
off it (see "Panels").

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
- **a view's header line, where it still has one, is fixed too**, not just the
  nav — it carries the view's primary action (Inbox's "Process — Inbox Zero"),
  which is worth no less at item 200 than at item 1. It carries neither the
  view's *name* nor its count any more: the title bar says both on every screen
  (see "Panels"), and the `?` panel is where the full name is spelled out where
  the nav abbreviates it — "Next actions" for "Next", "Someday/Maybe" for
  "Someday"
- **a count of zero is not written.** `0` beside a heading reads as a number
  worth looking at, and it is never the answer to anything: the empty line under
  it already says the list is empty, in words that also say *why* it being empty
  is fine. It lives in one partial in `_layout.html` (`count`), so the places
  that still carry a count of their own — the Today sections, a review step —
  cannot drift apart on it, and it is the rule the nav badges have always
  followed (see "Navigation")
- **no view carries a count header of its own any more.** Seven of them had one
  holding nothing else, and the title bar now says the view's name and its count
  on every screen (see "Panels"). Two headers saying the same number is one too
  many, and the one that goes is the one that only some screens had. The partial
  that rendered them (`counthead`) went with them; `pagehead` is left to the
  headers that carry something else, like the Scheduler's "New schedule"
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

## Panels

design.md, "Panels" asks for three pieces of chrome that can each be taken off
the screen, and one answer that takes all three. This is how they are built.

- **a panel that is off is not on the page.** The template renders the rail,
  the title bar and the key bar only when the state says so, rather than
  hiding them with a rule. Nothing then has to be written twice — no
  `display:none` per panel, no `:has()` reaching from the pane to a sibling —
  and a page that is not carrying a panel cannot be read as one that is
- **the state lives in `app_state`, beside the filter sets and the ages flag**
  (`internal/web/panels.go`). It is remembered screen state of exactly the same
  kind, and a single-user app has one place for that. It is one row holding one
  query string, so a press writes it in one go: three flags, zen, whether zen
  was the app's idea, and which zen screen is currently open
- **zen is a state, not a fourth panel.** The three flags say what is on in the
  ordinary way of working and are untouched while zen is up; `shown()` resolves
  the pair into what the templates get. That is the whole of "leaving zen gives
  the same screen back", and it is four lines rather than a saved copy to keep
  in step
- **asking for one panel while zen is up ends zen**, and then does what was
  asked. "Show me the title bar" and "show me nothing" cannot both be true, and
  the newer answer wins. The other two come back as they were, so a panel key
  is the zen key plus one panel
- **the chooser is four real forms in a dialog**, one per answer, each carrying
  `data-key` — so pressing `t` there is pressing the control, the same read-off-
  the-page construction as every other declared key (see "The keys"), and the
  bar in the dialog lists them without knowing what a panel is. The post writes
  the new state and comes back to the page it was pressed on, which is what
  closes the dialog. No client-side state anywhere in it
- **a key a dialog declares is live exactly while that dialog is open, and
  while one is open no key outside it is.** This had to be made explicit: the
  chooser sits in the layout, so its `t`/`n`/`k`/`z` are on every page in the
  app, and without the rule `t` would have stopped meaning "pick for today" the
  moment the chooser existed. It is the rule dialogs already followed in the
  key handler, moved down to where keys are found so it holds for the bar as
  well
- **`ctrl-v`, and a second `ctrl-v` presses zen.** Ctrl because a bare letter
  would collide on half the screens in the app and this key has to work on all
  of them, `v` for *view*, and not cmd because cmd-v is paste in every box on
  this machine. The second press answers with the option wanted most often,
  which keeps the whole of "clear the screen" at two presses of one key
- **it is centred, tinted like the key bar and one size up from it.** The key
  bar's left edge lines up with the content column because it names the keys
  for what is in that column; the title bar names the *screen*, so it belongs
  over the pane rather than over the list — centred, it reads as a caption and
  cannot be taken for the first row. The tint is the same argument the key bar
  makes (see "Screen layout"): it is chrome, and must not read as part of the
  page. The extra size is the one thing it does not share with the bar, because
  it is read at a glance and the bar is read on purpose
- **the title bar is a trail, and the server builds it.** Every page carries a
  list of steps (`page.Trail`): the view — with the same count the nav badge
  shows, read from `NavCounts.For` so the two numbers cannot come to differ —
  then whatever is being done inside it. `newPage` writes the first step from
  the view slug and a handler adds the rest with `step()`, which is why the
  processing screens read "Inbox / Processing / Action"
- **the inbox count is red here too.** It is the one count design.md asks the
  app to say loudly, and the rail was the only place saying it — which stops
  being enough the moment the rail is a thing you can turn off. Every other
  crumb count is the outlined badge the nav uses (see "Navigation")
- **a step that is a screen carries its slug; a step that is an item does
  not.** The slug is what `zen.views` names, so "processing" is a name the
  settings file can use and the project title in "Projects / Winter-proof the
  car" is not. It also means the stages of processing inherit the answer given
  for the run: zen is decided by *any* step of the trail matching, and a screen
  reached from inside a zen screen is the middle of the same one thing
- **the separators are drawn by CSS**, not written into the markup, so a step
  the browser adds is punctuated like the ones the server wrote. A dialog that
  is a step rather than a question says so with `data-crumb` and the key layer
  appends it while it is open — that is where "Inbox / Processing / Action /
  Create project" comes from, and adding another one is an attribute
- **the screen only gets its say on arrival.** `zen.views` is applied when the
  trail's screen is not the one already recorded as open, so turning zen off by
  hand on a screen the settings file names stays off — through stage two, a
  reload, a bounced form. Without that, "zen can be toggled manually" would be
  false exactly where the setting applies. Leaving for a screen the file does
  not name puts the panels back, but only if the app was the one that took them
  away: a zen you asked for is yours
- **the decision happens in `render`**, not in `newPage`, because a handler
  adds its steps in between and the trail is what the answer is read from
- **unknown names in `zen.views` stop startup.** `internal/conf` checks the
  shape of the list and `internal/web` checks the names against the screens it
  has, which is where that list actually lives. Splitting it that way keeps
  `conf` from holding a copy of the app's screens, and still fails loudly — a
  settings file is read once, so a name that quietly matched nothing would look
  set forever
- **the page says which view it is on the pane**, `data-view`, because the key
  layer needs the answer and the nav can be off. It is also the more honest
  source: on a boosted post the new page is in the DOM before htmx has finished
  with the URL, so the address bar is a step behind at exactly the wrong moment

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

- **doing does not borrow the slot; it highlights the view it was opened
  from.** It used to read "Doing…" there, taken and put back in the browser,
  because the mode was built out of the row and had nowhere else to say what it
  was. It is a page now, and there is a panel whose whole job is saying where
  you are: the title bar reads "Next actions / Doing" and the rail says which
  view that is (see "Panels"). One place answers it, and the nav is left saying
  the thing it always says
- **the slot stays a link.** Every key works on both screens now — the
  processing screen and doing alike — so `g n` is a way out of either, and the
  rail is another. A screen that could only be left one way would be a trap the
  moment that way was forgotten

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
- **the list views' header is still a bare number when there is one.** Dropping
  the duplicated view name (see "Screen layout") left just the item count above
  the list, which reads oddly on its own. Half of it is settled: at zero there
  is no number and no header (see "Screen layout"). What that number *is* still
  is not — the header count is the **filtered** count while the nav badge is the
  unfiltered one, and saying so is part of the filtering pass rather than a
  cosmetic fix. `nav.go` says why the badge deliberately ignores filters
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
