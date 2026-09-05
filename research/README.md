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
