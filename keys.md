# keys.md

Every key in the app, and the rules that decide what a key may be.

## Why this is its own file

design.md says what the app does and implementation.md says what it is built
out of, and a key is stubbornly both at once: which letter presses Done is a
design decision, and whether that letter can fire while the caret is in a box
is a fact about the browser. Split across the two files the map drifted —
implementation.md's "Keyboard" listed the keys, the processing screen listed
six more of its own, "Doing" listed two, "The remembered lists" a seventh, and
nothing held them together, so the same letter could be spent twice without
either passage being wrong. This file holds the whole map in one place; the
two others keep the reasoning that belongs to their layer and point here for
the letter.

The build detail of *how* a keypress is resolved — reading the physical key
rather than the character, and how the layout marker works — stays in
implementation.md, "Which key is which" and "Which layout the keyboard is in".
That is machinery, not map.

Em dashes here, following implementation.md rather than design.md, because
most of what follows is rules and wiring.

## What a key is

**A key is a place on the keyboard, not a letter** — design.md, "Panels" gives
the rule and the reason. Everything below names a key by the letter printed on
it in Latin, and means the place.

**Nothing advertises a key that does not exist.** The key bar is derived from
the page rather than written down: it lists only keys whose control is present
and can actually be pressed. That is the promise every rule here has to keep,
and it is why the map can change without a static help screen going stale.

## One letter, one button

A button gets one letter, and that letter means that button everywhere it
appears. This replaces the older arrangement, which let a declared key beat
the standing map so that `t` could mean trash on the processing screen and
today everywhere else. That was chosen deliberately, with the collision in
view, and it was sound while the screens were disjoint and few. It stopped
being sound once the same eight buttons appeared on fourteen screens: a key
you have to re-learn per screen is a key you press wrong, and the cost lands
on Delete, which is the one button where being wrong is expensive.

The corollary is the harder half, and it has to be stated carefully: **one
letter, one meaning — and the meaning is the noun, not the control.** `d` is
Done on a list row, Done on an action's page, and "Reviewed" on the weekly
review's rows; three controls, one idea, so one letter is right. `t` meaning
trash on one screen and today on another was two ideas on one letter, and one
of them destroys — that is the collision worth spending letters to remove, and
"the screens are disjoint" is not an answer to it.

Stated as "one letter means one thing" the rule cannot be satisfied by any map
at all, and a rule nobody can keep gets quietly dropped rather than argued
with.

## The three modes

A bare letter cannot be a command where the hands are — it would be typed. The
app has always split the keyboard on that fact, and the split is the thing
worth arguing about:

- **`command`** — every button is a bare letter. The caret being in a box is
  what stops them firing, which makes the screen modal in the vim sense. The
  app already arranges this correctly without saying so: screens you arrive at
  to *write* focus a field (the processing branches, a new action, promote),
  and screens you arrive at to *act on* do not (an action's page, a project's
  page), so the button bar is live on arrival exactly where the buttons are.
- **`modifier`** — every button is ctrl and a letter, live inside a box too.
  Nothing is modal, and every command costs a chord. Viable here only because
  the macOS readline bindings in text fields (`^a`, `^e`, `^k` and the rest)
  are not used on this machine; on a keyboard where they were, this mode would
  have to take most of them away, which is the opposite of what it is for.
- **`hybrid`** — bare letters, with ctrl on exactly the controls that have to
  fire mid-typing: Save, Create, Add. This is what the app does today, with
  the drift taken out.

**Hybrid reads the control, not the letter.** A control that is pressed with
the caret in a box says so, and hybrid is the only mode that looks. Keyed on
the letter instead, it is wrong the moment two buttons share one: `a` is Add
inside a project form *and* the Action branch on the processing screen, which
has no box on it at all — and a modifier there protects nothing while costing
a chord on the app's commonest answer. Two buttons may share a letter
(see "One letter, one button"); they do not thereby share a reason to spend a
modifier.

`keys.mode` in the settings file chooses one. It exists because the question
is not answerable in the abstract — it is a question about hands, and the only
honest way to settle it is to work in each for a while and notice which one
you reach past. Both other modes stay buildable for as long as the flag does,
which is the price of finding out; if one wins outright the flag can go.

**A control declares its letter, never its modifier.** `data-key="d"`, and the
mode decides whether that fires as `d` or `^d`. A template that wrote `^s`
would be encoding a policy decision in twelve places, which is how the two
halves of the split drifted apart in the first place.

What a control may declare is `data-key-typing`: that it is reached with the
caret still in a box. That is a fact about where the control sits, not a
choice of chord — `command` and `modifier` ignore it entirely, and only
hybrid asks.

## What the mode governs

Only buttons.

**Navigation is always bare, in every mode.** `j` `k` `o` `↵` `g` `z` `p` `w`
move a cursor or go to a screen, and you are never typing while doing that —
which is the entire reason a modifier exists. The one case where you are is
already answered: `^j` / `^k` hand you from the filter line into the list it
narrows.

**The globals are always ctrl, in every mode.** They were reserved when ctrl
meant "not a screen key", and they stay reserved: they do not belong to the
screen, so they cannot take part in a flag that is about how a screen's own
controls are pressed.

This rule is what keeps `o` and `^o` apart. `o` opens the selected row — the
item's own page, inside the app. `^o` follows a link written *inside* that
item, out to a browser tab (design.md, "Following a link"). They are opposite
directions and both live on the same row at the same moment, so they can never
be the same chord. Under a flag that modified everything, they would have
been.

## The map

**Buttons** — the letter is declared, the mode supplies the modifier.

| Key | Button | Where |
| --- | --- | --- |
| `s` | Save | action, project, someday item, schedule |
| `c` | Create | the processing branches, new action, promote, new schedule, settings, the draft and new-project dialogs, scheduler |
| `a` | Add | project, promote, the project branch of processing |
| `b` | Back | every screen that can be left — see "Leaving a screen" |
| `d` | Done | every list row, action, project, the completion request, doing, and the review step's "Reviewed" |
| `t` | Today | every list row that carries the mark, and an action's page |
| `⌫` | Delete | every row that carries one, action, project, schedule, a capture on the processing screen, a draft row |

**The processing branches** are six answers to one question rather than six
controls on a screen — see implementation.md, "The processing screen". They
obey the same rule as everything else here; three of them had to move to do
it.

| Key | Answer |
| --- | --- |
| `⌫` | delete it |
| `r` | reference material |
| `2` | the two-minute rule |
| `y` | someday/maybe |
| `a` | make it an action |
| `p` | make it a project |

`a` stays although Add also has it: Add adds an action and this branch makes
one, so it is one noun and one meaning, and the two are never on a screen
together — stage one has no Add, stage two has no branches. They do not share
a modifier, though: Add carries `data-key-typing` and this does not, because
stage one has nothing on it to type into.

`p` is free because the row key `p` is gone. It only ever aliased `↵`: an
inbox row's link already goes to the processing screen, which is why the bar
showed the two sharing one label. An alias is the cheapest thing in the map to
spend, and dropping it makes nothing unreachable.

`y` is what is left of "Someday/Maybe" once every other letter of its own name
is spent — `s` Save, `o` open, `m` the jump, `e` the time, `d` Done, `a`
Action, `b` Back. It is a poor mnemonic and the only letter that is free both
bare and under ctrl, which is what a branch key has to be to survive all three
modes. The branch that had to give way is the right one: Save is on four edit
screens and is one of the three keys that must fire while a box is being typed
in, and this one is pressed once per idea you decide not to commit to.

**A key that is not a letter takes no modifier, in any mode.** `⌫` and `2` are
themselves in all three, because there is nothing for a mode to change about
them — which also keeps the delete key identical everywhere, deliberately.

| Key | Goes to |
| --- | --- |
| `j` `k` | through the current list; `^j` `^k` do the same from inside the filter line |
| `↵` `o` | open the selected row |
| `g` + letter | a view — see implementation.md, "Navigation" |
| `g g` | the capture dialog |
| `z` | Inbox Zero over the whole inbox |
| `w` | the selected action, alone on the doing screen |

**Globals** — ctrl in every mode.

| Key | What |
| --- | --- |
| `^e` | show me the time: the ages on every list, and the timer on the doing screen |
| `^o` | follow a link in the item under the cursor |
| `^m` | jump to a control on the screen already open |
| `^f` | the filter line, and a second press takes it and every filter away |
| `^v` | the panel chooser; a second `^v` presses zen |
| `^↵` | submit the form being typed in |
| `?` | the view's own help |
| `esc` | unwind one step — see below |

`^e` is *elapsed*, which is the one word that covers both halves of what the
key means: an age is elapsed time and a timer counts it. It has to cover both,
because "show me the time" must never be two keys or one key with two answers
— it took `^t` for that reason, and gave it up because Today is pressed many
times a day and a display flag is pressed occasionally.

## Buttons that get no letter

Six buttons appear on one or two screens each and are rare, deliberate acts:
**Parked / Next**, **Detach**, **Promote**, **Undone**, **Inbox** on a someday
item, and **Recapture** on the audit. They are reached with `^m` and a letter,
which is what `^m` is for — `g` goes to a view, `^m` goes one level in.

What made that tier untrustworthy was not that it existed but that its letters
moved: `^m l` deleted an action inside a project and `^m e` deleted one
standing alone, because Detach vanished from the screen and every letter after
it shifted up. So **a control may declare its jump letter**, and a declared
letter is claimed before any computed one. The six above declare `n` `x` `p`
`u` `i` `c` and keep them whatever else is on the screen. Everything that does
not declare one still takes the first free letter of its own name, which is
what makes the unlettered controls guessable — see implementation.md,
"Jumping to a control".

## The bar while you are typing

With the caret in a box, every bare letter is a character rather than a
command. The bar narrows to what a chord can still reach — the `^` keys, the
"needs …" line, and `esc leave the box` — and everything else goes.

It goes rather than being greyed out or annotated. The bar's one promise is
that what it lists works now; a key shown with a note saying it does not is
still a key shown, and `b back` beside a box you must press `esc` to get out
of is a plain lie about what one press does. There is nothing to add to make
that clear — there is something to remove.

Which makes the bar the mode indicator the modal design would otherwise need.
In `command` almost the whole bar empties as the caret enters a box and comes
back on `esc`, so the two states are visibly different without a word being
written to say so. In `modifier` almost nothing changes, because almost
nothing was bare. That difference, watched for a week, is most of what the
trial is for.

For the same reason `^↵` drops out of the bar when the form's own submit
button has a chord of its own: `^s save` and `^↵ save` are one answer said
twice. In `command` mode the declared key is a bare letter and dead in a box,
so there `^↵` is the only way to finish a form from inside one and it stays.

## Leaving a screen

**`esc` unwinds one step and never navigates.** The suggestion list, then the
box; a dialog; the help panel; the caret, back to the keys. When there is
nothing left open it does nothing at all, however many times it is pressed.

That last clause is the change. `esc` used to close things *and*, when there
was nothing left to close, follow the screen's way out — two meanings on one
key, which is tolerable when the key is pressed occasionally and dangerous the
moment the keyboard is modal, because leaving a box is then the commonest
press in the app and one extra tap abandoned a half-written action. The
unwinding half was already the documented rule for the project picker and the
filter box; this only finishes applying it.

**`b` leaves.** Back rather than Cancel, because that is what the thing does
and what the code has always called it — `data-cancel` holds a URL, the
template variable is `Back`, the handler is `back()`. On an action's page or a
project's page nothing is being abandoned; you are returning to the view you
came from. Cancel stays the right word only where something is genuinely being
given up, which is the screens that create.

**A form with unsaved work costs a second `b`.** The first press does not
leave: the fields that differ from what was saved are marked, and the bar
reads `s save · b discard`. The second press leaves and discards. No dialog
and no confirmation, for the reason there is none anywhere else — design.md,
"The protocol is followed, not enforced" — and no third state to learn: the
same key, pressed again, still means leave.

The marking borrows the dashed border a draft row already wears, because that
is the app's existing way of saying "this is not saved yet", and it must not
borrow the error styling: nothing is wrong, it simply is not written down.

Only screens that edit something get the marks. A screen that *creates* is
unsaved wholesale — everything typed into it would be lost — so marking every
filled box there would mark the form and say nothing. Those screens still cost
the second `b`.

## What is built

All of it. `keys.mode` defaults to `hybrid`, which is what the app did before
any of this, so nothing about the trial is a one-way door.

Three mechanisms carry the whole map, and each one exists so that a key can
never be advertised without working:

- **`renderKey`** is the single place a declared letter becomes a chord.
  Everything that reads a `data-key` goes through it — the handler and the key
  bar both — so the two cannot come to disagree about what a screen offers.
- **`kb-complete`, `kb-pick`, `kb-delete`** are now screen forms as well as
  row forms. `d`, `t` and `⌫` press the selected row's if there is one and the
  screen's if there is not, which is how an action's page and a row of the
  list it was opened from answer the same key. A row under the cursor is never
  stepped over: with no such form on it the key does nothing rather than
  reaching past it.
- **`data-jump`** lets a control name its own `^m` letter, claimed before any
  computed one, which is what stops Delete moving between `l` and `e`.

What is left, and deliberately:

- **the project picker still takes a bare `c`** for "new project" while its
  list is shut. It owns the keyboard the way a dialog does, so nothing
  collides, but it is a second `c` in a file that argues against second
  meanings. Worth revisiting once the trial has settled which mode wins.
- **the panel chooser keeps `t` `n` `k` `z`.** A dialog is a menu of its own
  letters and the jump layer already stands down inside one, so these are not
  the screen's keys to normalize.
- **`^↵` still submits the form being typed in**, alongside `c`. It is not a
  button's key — it is "finish this", from inside a box, whatever the button
  happens to be — so it survives the map rather than being replaced by it.
