# CLAUDE.md

Read **README.md, "Working on this codebase"** before changing anything here.
Those conventions are binding, not advisory. The one easiest to skip is
repeated below, because skipping it is how the docs rot.

## Docs are part of the change, not a follow-up

Every bug fix and every UI change updates the docs in the same unit of work:

- **design.md** — what the app does and why. Update it when *behavior* moves:
  a rule, what a view contains, what a process does, what a field means.
- **implementation.md** — what it is built out of. Update it when a *technical
  or UI decision* moves: a keybinding, where a control lives, how a screen is
  laid out, a storage or protocol choice.

A UI change nearly always touches implementation.md, and touches design.md
whenever the rule behind the UI moved rather than just its presentation. A bug
fix touches whichever doc was describing the behavior that turned out to be
wrong — and if neither did, that silence is usually itself the bug.

If a change genuinely needs no doc update, **say so explicitly** rather than
staying quiet about it. That is how a reader tells "considered, not needed"
from "forgotten".

Both docs justify every rule rather than merely stating it. Match that voice:
a new bullet says why, not just what.

Doc updates go in their own commit, separate from the code commit that
implements them, matching this repo's existing history.
