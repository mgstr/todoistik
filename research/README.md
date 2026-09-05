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
