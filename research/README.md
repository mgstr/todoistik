# research

Design options that were built and compared before a decision, kept so the
decision can be revisited without redoing the comparison.

A study here is a self-contained HTML page: open it in a browser, no server and
no build. Each renders the real thing with the app's own stylesheet rather than
a mockup of it, so what you are looking at is what would ship.

These are **records, not specs**. Where a study disagrees with design.md or
implementation.md, those two are right and the study is simply older. What a
study is for is the part the docs do not carry: the options that lost, and why.

---

## item-line-study.html — the item line, and how an age is worded

**2026-09-05 · decided: D, the age as a chip beside the title.** Implemented in
`internal/web/static/style.css` (`.row`, `.age`) and `internal/web/server.go`
(`humanAge`).

The age was pinned to the right edge of the row, which needed a rule under
every item to carry the eye across the gap — nine items, nine horizontal lines,
on a page that should read as a short list. Five treatments were rendered, each
in both themes:

| | Variant | Outcome |
|---|---|---|
| A | Right-bound, ruled | The starting point |
| B | Inline, muted grey | Rejected: grey text beside black text still parses as a continuation of the title — "call the dentist yesterday" reads as a phrase before it reads as two fields |
| C | Inline, small-caps mono | Runner-up; the typeface alone did the separating |
| **D** | **Inline chip** | **Chosen.** Unambiguously its own field, in the badge shape the app already uses elsewhere |
| E | Right column, rules removed | Rejected: a column answers "what has sat here longest?" at a glance, but strands badly once titles are long enough to wrap |

The page's second half settled the **age vocabulary**, replacing the old codes
(`1d`, `3w`, `4mo`) with words. Two gaps in the first draft of the scale were
found and closed there: an age of **two days** had no rule, and days **57–60**
fell between "a month ago" (ending at 8 weeks) and "2 months ago" (starting at
about 61). The boundaries as shipped are pinned by `internal/web/age_test.go`.

Known trade-off, accepted: on a Next actions row the age is a fifth chip beside
context, duration, tags and due date — the least important of them and the most
constant. Revisit here if the row starts to feel crowded.

---

## nav-badge-study.html — where the per-view item counts sit

**2026-09-05 · decided: D, outlined corner pill.** Implemented in
`internal/web/static/style.css` (`nav .badge.navcount`).

**Superseded 2026-09-07 by `left-nav-study.html`.** The nav is a rail now: the
count sits at the row's right edge rather than the label's corner, and the
blanking rule below is gone with the collision it existed for. What follows is
the record of what a *horizontal* bar had to solve, which is still the reason
the badge is outlined and still the reason red is spent only on the Inbox.

The counts sat inline, on the same baseline as the view's label, which read as
a second word in the view's name rather than as a marker attached to it. Four
placements were rendered side by side, each in both app themes:

| | Variant | Outcome |
|---|---|---|
| A | Inline pill | The starting point. Widest bar, and every label shifts sideways when a count changes |
| B | Filled corner badge | The literal iPhone badge. Rejected: ten filled accent circles spend the two colours the nav already reserves for "the inbox needs emptying" and "this is the view you are on" |
| C | Bare superscript | No enclosing shape, so the number still reads as part of the label — close to the problem in A |
| **D** | **Outlined corner pill** | **Chosen.** Corner position, badge palette, red reserved for the Inbox and the accent for the active view |

Also settled here: corner counts collide with the `g` jump hints, which sit at
the neighbouring item's top-left. **The counts blank while `g` is held** — during
the overlay you are choosing a destination, not reading counts — rather than
widening the nav permanently to fix something visible only while a key is down.

If you want to change the badge style after living with D, start from this page:
the four variants are still in it, each as a CSS block that can be lifted
straight into `style.css`.

---

## process-subject-study.html — the processing screen, and where the eye lands

**2026-09-06 · decided: D, the accent rule.** Implemented in
`internal/web/static/style.css` (`.process .subject`, `.capture-text`).

Stripping the screen of its crumb and its *"What is it?"* heading left the
captured line as an ordinary `h1` above a menu of bordered, filled buttons —
so the loudest thing on a screen about one sentence was the menu. Five
treatments were rendered as the whole screen, in both themes, at three capture
lengths:

| | Variant | Outcome |
|---|---|---|
| A | As built — `h1` 1.5rem | The starting point, and the problem |
| B | Type only — 2.1rem, age as a chip | Runner-up; size alone does win the page, but a three-line capture at 2.1rem takes the screen over |
| C | Card | Rejected: another box among boxes, competing with the buttons rather than outranking them |
| **D** | **Accent rule** | **Chosen.** Emphasis with no enclosing shape, so it cannot be mistaken for a control; the accent is otherwise unspent on this screen, and a rule holds a one-line and a three-line capture equally |
| E | The list's selected-row styling, carried over | Rejected: implies the item is still selectable, and `j`/`k` have nowhere to go here |

Also settled here: **the run counter stays gone.** Removing the crumb removed
the only *"N left"*, and the nav badge is suppressed while the screen is up
(implementation.md, "Navigation"), so a run counts down invisibly. Two ways
back were rendered — the nav badge restored, and a `4 left` chip on the subject
— and neither was taken: a count you cannot see is also a count you cannot be
discouraged by on the fourteenth item, and the screen is meant to hold one
decision at a time. Both toggles are still in the page if living with it
changes the answer.

---

## process-actionable-study.html — the third row of the processing screen

**2026-09-06 · decided and built.** Implemented in `internal/app/someday.go`
(`ProcessAction` taking a project and a park flag), `internal/app/views.go`
(`MatchProjects`), `internal/web/ui.go` (`processActionBranch`, `bounce`) and
the two `process_action.html` / `process_project.html` templates.

The actionable answers — action, waiting-for, project — written out as a
sequence of questions run five deep: action or project, who does it, standalone
or filed, an existing project or a new one, next action or parked. The page
argues that only the first is a question and the rest are fields and defaults,
and it renders the argument rather than asserting it: the project box is live,
so the three states it has to carry (empty → standalone, matched → filed there,
unmatched → create it) can be tried before any of it is written.

The green line under each form says what pressing the button would create. It
is there because that is the claim the page has to make good on — that fewer
questions did not mean less said.

The **who** control is one row that grows rather than two rows that appear:
pressing "Someone else" reveals the name box to its right, on the same line,
and puts the cursor in it. An extra row opening underneath pushes everything
below it down, and on a form you are reading top to bottom that costs a
re-read; growing sideways costs nothing. Switching back to "I do it" clears
the box, so a name cannot be left behind on an action that is yours.

Two more it settles by rendering them: a **new project's first action cannot
be parked** (a project's first action is its next action by definition, so the
park control is absent in that state), and the **who** control belongs on the
action wherever an action is being written, including inside the project form.

Three changes it would need from design.md are listed on the page.

What shipped has since moved past this page in two ways, and the page is kept
as the record of the reasoning rather than of the result:

- **the project is chosen, not typed.** The study's box resolves a typed name
  on submit; the app now has a picker — the list is in the page, letters filter
  it, `↑↓` and `ctrl-j`/`ctrl-k` move, and creating a project is an explicit
  choice rather than what an unmatched name means. See implementation.md,
  "Stage two"
- **the form is three controls.** Everything the study drew as a field —
  who does it, size, focus, context, tags, dates — is now written in the
  description box instead (design.md, "Writing an action"), so `park` and the
  `who` row are gone from the form entirely

The **who** row shipped exactly as rendered, growing sideways rather than
opening a row — and was then removed with the rest of the fields when the
description became the form. The `:has()` technique it used survives in the
picker.

---

## left-nav-study.html — the nav as a rail down the left

**2026-09-07 · decided: C, the grouped rail.** Implemented in
`internal/web/templates/_layout.html` (the rail, its captions and the `.pane`
the key bar moved into) and `internal/web/static/style.css` (`nav`, `nav .grp`,
`nav .badge.navcount`, `.ghint`).

Four whole Next actions screens in both themes, because turning the nav is not
a change to the nav — it is a change to how much room everything else gets.

| | Variant | Outcome |
|---|---|---|
| A | Top bar, as built | The starting point. 3rem of height always, 5.4rem once it wraps |
| B | Rail, straight translation | Rejected: it leaves the thirteen in one undifferentiated column, which is the bar's own weakness carried over into a shape that no longer forces it |
| **C** | **Rail, grouped** | **Chosen.** Capture / Do / Committed / Later / Records — five kinds of place a row could only imply by adjacency |
| D | Rail, grouped, Records pinned to the foot | Not taken, though the page recommended it. The difference is one `margin-top: auto`; C's version keeps every row at a position that does not move when the window's height does |

The page's real argument is the arithmetic. Main caps its column at 62rem and
the rail is 11.5rem, so above about **76rem of window the rail is free** — it
spends margin that was already empty and hands 3rem of height back to the list.
Between 48 and 76rem it is a straight trade of width for height. Below 48rem it
does not work at all, where the bar wrapped and survived.

Both things the page said a rail would reopen were reopened rather than waved
through, and both are now decided:

- **the nav badge.** Half of nav-badge-study's reasoning was about a horizontal
  bar — an inline count reading as a second word in the view's name, and labels
  shifting sideways when a number changed. Neither pressure survives in a column
  of fixed-width rows, so the count moved from the label's corner to the row's
  right edge, and the "counts blank while `g` is held" rule was deleted: the
  hints have a gutter of their own now and nothing collides. That study's
  variants still stand for what a *bar* had to solve
- **whether this is a desktop app.** Answered yes, in design.md's "Design
  principles", rather than by a breakpoint that would have meant two navs to
  keep true. The phone was always a capture device reaching the app through the
  capture API, and that had simply never been written down

One bug surfaced while building it, older than the rail: on the Inbox's own row
the count was drawn accent-on-red and could not be read, because `a.on` and
`a.alert` set the same property and `a.on` came second. The alert wins it now —
standing on the Inbox is not the same as having emptied it.

---

## doing-timer-study.html — where the timer sits in doing mode

**2026-09-08 · decided: F, the bottom-right corner, at `opacity: .15`.**
Implemented in `internal/web/static/style.css` (`#doing .timer`) and
`internal/web/static/app.js` (`startTimer`), behind `doing.show_timer`.

Doing mode puts one action alone on the screen (design.md, "Doing one action").
The timer counts the minutes since it went up — `07`, then `1:04` past the
hour — and exists only to build a feel for how long work takes; it is never
stored. The constraint set before the page was drawn: the title's own size, so
it never reads as a lesser kind of information, held back by opacity alone.

| | Variant | Outcome |
|---|---|---|
| A | Above the title | Rejected: the eye meets the clock before the work, which is the opposite of the point |
| B | Below the title | Runner-up: reads as a caption, but the whole block shifts up to make room, so the title is not where it sits without a timer |
| C | Leading, same line | Rejected: two digits in front of a phrase read as an index number, and the title goes off centre |
| D | Trailing, same line | Rejected: same off-centre problem, plus the title drifts as the digits change width |
| E | Top right corner | Close second, and the wall-clock instinct is real; the top edge is simply more in the way of a centred block than the bottom is |
| **F** | **Bottom right corner** | **Chosen.** The corner the eye visits last. The title sits exactly where it would with no timer at all, and nothing moves when `59` becomes `1:00` |

The page's second half was the decision under the decision: **how faint**, at
`.20`, `.30` and `.45` in both themes. `.15` was chosen from it — below the
range drawn, deliberately, because every rendered value still pulled at the eye
on a screen whose whole purpose is that nothing does. `tabular-nums` finishes
the job the corner started: the digits do not change width either.

Known trade-off, accepted: with `doing.show_keybar = true` the timer sits
directly above the bar's right-hand end. Rendered on the page so the collision
was chosen rather than discovered.

---

## action-page-study.html — the edit-action screen, and where a field's name sits

**2026-09-09 · decided: B, the name in a gutter beside its box.** Implemented in
`internal/web/templates/_layout.html` (the `actionfields` and `projectfields`
partials), `internal/web/templates/process_action.html` (the new-project
dialog), `internal/web/static/style.css` (`.stack label.gutter`) and
`internal/web/static/app.js` (`missing`).

An action's page was too tall for what it holds. Five layouts were rendered as
whole screens in both themes, each carrying the same 4 fields, 2 dates and 7
buttons — nothing was removed from any of them — and the page measures its own
specimens on load rather than asserting heights:

| | Variant | Height | Outcome |
|---|---|---|---|
| A | As built | 429 px | The starting point |
| **B** | **The name in a gutter** | **341 px** | **Chosen.** Four label lines that were nothing but a word, gone, with every field, name and reading order untouched |
| C | The project joins the dates line | 329 px | Not taken, though the page recommends it: it turns a `readonly` box into a chip, which is a change to what the screen *contains* and not just to how it is laid out |
| D | Two columns | 337 px | Rejected by its own measurement — see below |
| E | No labels at all | 242 px | Not taken. It is B plus C plus deleting the four field names; the shortest, and the only one that removes words from the screen. Still open if B turns out not to be enough |

The page's real finding is the ledger of where the height actually goes: the
description box 110 px, the four label words 88 px, the button row 65 px, the
project field 61 px, the two boxes you came to edit 61 px. **D was built
expecting to win and lost.** The form caps at 34rem inside a 62rem column, so
half the page is empty for its whole height and filling it looks like the
obvious fix — but a rail holding seven buttons is itself as tall as the form
beside it, so the two columns come out level. It buys less than C and spends a
decision that is written down ("Save stands in the same row as Complete and
Delete") to do it.

Settled while implementing B, and not visible on the page:

- **the gutter is one width for the whole app**, 7.5rem, rather than sized per
  form. `project.html` shows a project's fields and its add-action box on one
  screen, and two forms whose boxes start in different places read as a
  mistake. 7.5rem is what the longest field name in the app needs —
  "Definition of done" — so no name wraps anywhere and every box on every
  screen starts at the same place
- **the stack keeps its 34rem**, so the trade is 7.5rem of box width for four
  lines of height. That is the same trade the rail made and for the same
  reason: height is the axis these screens are short of
- **the schedule forms and the someday item still stack their labels.** They
  were outside what was asked for, and their names all fit the same gutter, so
  joining them is one class each

One thing noticed and not acted on: the title box still carries
`placeholder="Starts with a verb, fully self-descriptive"` and a project's
title box `"A reference to the outcome, not what to do"`. implementation.md's
"neither box carries a placeholder" is about the meta and description boxes, so
these are not a contradiction — but they are the same sentence that rule was
written against. They cost no height, so they were no part of this.
