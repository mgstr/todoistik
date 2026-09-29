# CLAUDE.md

Read **README.md, "Working on this codebase"** before changing anything here.
Those conventions are binding, not advisory. The one easiest to skip is
repeated below, because skipping it is how the docs rot.

## Docs are part of the change, not a follow-up

Every bug fix and every UI change updates the docs in the same unit of work:

- **design.md** — what the app does and why. Update it when *behavior* moves:
  a rule, what a view contains, what a process does, what a field means.
- **implementation.md** — what it is built out of. Update it when a *technical
  or UI decision* moves: where a control lives, how a screen is laid out, a
  storage or protocol choice.
- **keys.md** — the keyboard. Update it when a *key* moves: which letter
  presses what, what a modifier is spent on, how a screen is left. A key never
  goes in the other two files; they point here for it. If the key is decided
  but not built, say so in that file's "What is built" rather than leaving the
  gap for a reader to trip over.

A UI change nearly always touches implementation.md, and touches design.md
whenever the rule behind the UI moved rather than just its presentation. A bug
fix touches whichever doc was describing the behavior that turned out to be
wrong — and if neither did, that silence is usually itself the bug.

If a change genuinely needs no doc update, **say so explicitly** rather than
staying quiet about it. That is how a reader tells "considered, not needed"
from "forgotten".

Both docs justify every rule rather than merely stating it. Match that voice:
a new bullet says why, not just what.

The docs go in the **same commit** as the code they describe. "Same unit of
work" is meant literally: a commit that changes behaviour and leaves design.md
saying the old thing is a commit that is wrong about itself, and splitting the
two guarantees at least one revision where the documentation and the code
disagree. Landed history is squash-merged pull requests, so a split is undone
on the way in anyway — it buys nothing and costs a window where `git show` on a
commit does not explain it.

This is the one convention that changed its mind. It used to ask for a
docs-only commit, on the reasoning that design discussion reads better apart
from implementation. That is true of a *branch*, which is where the separation
belongs, and it was never true of the commit that lands.
