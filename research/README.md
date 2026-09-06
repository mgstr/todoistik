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
