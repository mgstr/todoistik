# Things to implement in the todoistik app

Ideas for after the MVP. This is a holding pen, not a plan and not a spec:
nothing here has been designed, argued for or promised, and design.md stays
silent on all of it until one is taken up properly. Anything that graduates
leaves this file and becomes a rule in design.md or a decision in
implementation.md — see implementation.md, "Wanted, not specified".

- AI(?) helper that analize inbox item during processing and make suggestion for a proceeding step.
- AI(?) helper that analyze the archived items and identifie recurring actions and suggest a schedule creation for it.
- create a TUI version of app, that will be running in the terminal.
- move configuration into .config file, so it is easy to change configuration using AI
- more palettes than the two: the app is light or dark and the choice is built (design.md, "Theme"), but a third scheme is a third palette nobody has drawn, and `theme` would have to take a name rather than one of three words.
- add localization. Note this is about the interface's language and nothing else: the language an *item* is written in is settled per item, wherever it matters, which is the shape the verb check already took (design.md, "Verbs").
- undo across the app: one universal `u` that reverts the latest operation, whatever view it happened in. **Undo is not itself an operation — it is a way of forgetting one, as if it never happened — so it leaves no audit entry of its own and consumes the one it reverses.** That keeps the log honest rather than bending it: the log is the record of what happened and still stands, and an entry reading "trashed" for an item sitting safely back in the inbox would be worse than no entry at all. The machinery is largely there already, since design.md requires every entry to carry a snapshot of the item as it was, which is exactly what reversing needs — a trashed item is re-inserted from it, an edit is restored to it, a two-minute-rule completion puts the item back in the inbox it came from. Left to settle: how far back `u` reaches — the last operation only, or a stack.
- replace "Next" with "Working..." when I am starting to work on particular Next action.
