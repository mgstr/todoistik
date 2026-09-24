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

**And the bar is now where the control is.** The row of buttons under a form
is gone; every control a screen has is drawn in the bar, pressed by the letter
beside it or by pointing at it (design.md, "Panels"). Nothing in this file
changes because of that — a key still presses a control, and the control is
still the thing being pressed — but two consequences land here:

- **an entry that presses a control is pressable, and an entry that steers is
  not.** `j k move`, `g go to`, `^m jump`, the "needs …" line: these are how
  you get somewhere rather than things you press, and the bar must not invite
  a click on them. The promise "everything listed works" would otherwise be
  true of the letters and false of the targets.
- **every control needs a letter now, including the rare ones.** A button with
  no letter used to be reachable anyway, by `^m` and a hint hung on the button
  itself. With the button off the page there is nothing to hang a hint on, so
  the six below became five letters — see "Buttons that get no letter".

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
Done on a list row and Done on an action's page; two controls, one idea, so
one letter is right. `t` meaning
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
the letter instead, it is wrong the moment two buttons share one — and the
pair that made the point was `a`: Add inside a project form, and the Task
branch on the processing screen while that branch was still `a`, on a screen
with no box on it at all. A modifier there protected nothing while costing a
chord on the app's commonest answer. That pair has since come apart, because
the branch is `t` now (see "The processing branches"), and no letter in the
map currently carries both a typed control and an untyped one. The rule
outlives the example on purpose: `data-key-typing` is a fact about where a
control sits, so the next shared letter brings the case straight back, and a
scheme that had meanwhile keyed the modifier to the letter would be wrong
again the day it did. Two buttons may share a letter (see "One letter, one
button"); they do not thereby share a reason to spend a modifier.

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
| `a` | Add | project, promote, the project branch of processing — the actions *after* the first, which is open on the form and not added |
| `b` | Back | every screen that can be left — see "Leaving a screen" |
| `d` | Done | every list row, action, project, doing — on a project's page it finishes the next action while there is one and the project once there is not |
| `r` | the review mark | a row of a weekly review step |
| `t` | Today | every list row that carries the mark, an action's page, and a project's next action |
| `⌫` | Delete | every row that carries one, action, project, schedule, a capture on the processing screen, a draft row |
| `x` | Detach | an action's page, inside a project |
| `p` | Promote | a standalone action's page — the same `p` as the Project branch below |
| `u` | Undone | a completed action's page, a completed project's page |
| `i` | Inbox | a someday item's page |
| `h` | Theme | the Settings screen's theme row |
| `#` | the project's tags, onto its next action | a project's page, on the "Next action" heading |

**`r` is one of the two letters in the map that are spent twice, and it is
worth saying why rather than pretending otherwise** — `t` is the other, and it
is argued for in "The processing branches" below, by this same paragraph's
test. It is the processing screen's
*reference material* branch and it is the review step's *mark* — two nouns, not
one, which is exactly what the rule above forbids. What buys the exception is
what the rule is actually protecting against: `t` was trash on one screen and
today on another, on screens you move between all day, and one of the two
destroys. These two are never on a screen together, neither destroys anything,
and both are the first letter of their own word, which is the thing that makes
a letter guessable. The honest alternative was `d`, which used to press the
review's "Reviewed" button — and `d` is Done, which on a list of actions means
*the action is finished*. A key whose worst misfire completes the item you were
only reading was the more expensive letter to keep.

**The mark is one key and it goes both ways.** `r` on a marked row takes the
mark off — the bar says which, reading `r reviewed` or `r unreviewed`
depending on the row under the cursor, the same way the theme row's entry says
the answer it lands on. Not two keys: it is one claim ("I walked this") said
from whichever side you are standing on, which is the same argument the digits
below make for the bookmarks.

**The review's digits.** On the weekly review's own screen, `1`…`7` open the
step whose number is printed beside it (design.md, "Weekly review"). Bare, like
every other key that is not a letter, and free to be bare there: the bookmarks
are `^1`…`^9` and are offered only where a screen has a filter line, which this
one has not. `1` is Gather and presses nothing, because Gather has no screen —
the line keeps its number, since the number is its place in the order, and the
bar simply does not offer a key for it. That is the standing rule doing its
job rather than an exception to it: nothing advertises a key that does not
exist.

**The bar shows them as a range, `2…7 step`**, the shape the match list's
`1…3 copy` and the bookmarks' `^1…9` already have — an entry that says how to
steer rather than one that presses. The numbers are printed on the lines, which
is where a numbered list's keys are read; the bar listing all six would be the
screen written out a second time, in the one place that is supposed to say what
is *not* on the screen. It starts at 2 and not at 1 for the reason above: a
range is a claim about what can be pressed.

**`#` is the one button in the map that is not a letter, and it is the
notation it writes.** The mark it presses puts the project's tags on its next
action (design.md, "Editing items"), and what a tag is written as is `#name` —
so the key is the thing it does, which is a stronger claim to a place on the
keyboard than the first letter of a word describing it. `m`, for Meta, was the
obvious letter and the weaker one: it names the field, and the field is not
what moves.

Nothing collides. The digit under it is spoken for twice already — the
review's `1`…`7` and the bookmarks' `^1`…`^9` — and both are pressed without
shift, so the shifted place was free. Had the key been `m` instead, it would
have had to be argued against `^m`, the jump prefix; that argument is winnable,
because the globals are always ctrl and `m`/`^m` would have stayed apart the
way `o`/`^o` do, but it did not have to be had.

**Being a non-letter costs it one thing, and the cost is worth naming**: a key
that is not a letter is itself in all three modes, because there is nothing for
a mode to change about it — which means it can never be a `data-key-typing`
key, so it cannot fire while the caret is in a box. From inside the meta line
it is `esc` and then `#`. That is the same bargain `⌫` already makes, and the
alternative was `^#`, which is ctrl-shift-3 and is not a key anybody presses.

**It is a place on the keyboard and not a character, like every other key
here.** Shift-3 prints `#` in Estonian and `№` in Russian, so `keyOf` answers
for the place and not for what was printed — the rule that already keeps `?`
working, which is the same physical key under a layout that prints `,` where
the other prints `?` (see "Which layout the keyboard is in"). Without that, the
one mark on the screen written in notation would have been the one key that
stopped working in Cyrillic.

**`h` is what is left of "theme" once `t` is spent.** Today is pressed many
times a day and a palette is chosen when the light in the room changes, so the
letter goes to the one that is pressed, and the other takes the next free
letter of its own name — the same derivation `y` has for Someday/Maybe. It
presses the *next* answer rather than a control called Theme: the row offers
all three at once and the letter sits on whichever comes after the one in
force, which is why the bar reads `h theme dark` and not `h theme`. There is no
key that means "the theme row" — that would be a key that opens a menu, and the
row is already on the screen.

**Delete is last in the bar, beside Back.** Not a letter decision but a
position one, and it belongs here because it is about the same key: `⌫` used
to follow the row keys, which put it in the middle once the buttons moved in
and the screen's own answers came after it. It is the one control where being
wrong is expensive, and an entry a pointer can reach is worse to have among
the keys pressed all day than a letter was.

**The five above are the tier that used to have no letters.** They are rare,
deliberate acts and they were reached with `^m` and a declared letter, which
worked only while the button was on the screen to hang a hint on. Two of them
share a letter with something already on the map, and both share the *noun*,
which is what the rule asks: `p` is Promote here and the Project branch on the
processing screen, and both mean "make this a project"; `i` is Inbox, and the
Recapture button on the audit means the same thing — send this to the inbox.
Neither pair is ever on one screen. `n`, `x` and `u` were free.

**The processing branches** are six answers to one question rather than six
controls on a screen — see implementation.md, "The processing screen". They
obey the same rule as everything else here; three of them had to move to do
it.

| Key | Answer |
| --- | --- |
| `⌫` | delete it |
| `r` | reference material |
| `d` | the two-minute rule |
| `y` | someday/maybe |
| `t` | make it a task |
| `p` | make it a project |

**The task branch is `t`, and it was `a` while it was called Action.** The
letter moved because the noun did: this answer makes a standalone action, and
a standalone action is a *task* everywhere it is afterwards read — the view it
lands in, the nav entry, the word design.md uses for it (design.md, "Tasks").
The screen that names the thing was the one place still calling it the other
word, which meant the commonest answer in the app was written under one noun
and found under another.

`t` is the second letter in the map spent twice, and it passes the test the
`r` paragraph sets: Today and Task are never on a screen together — stage one
carries no row with the mark on it, and every screen that does carries no
branches — neither destroys anything, and both are the first letter of their
own word. There is a sharper thing to say about it, since this file spends two
paragraphs on `t` being the collision that had to go: what had to go was `t`
meaning *trash* here and *today* elsewhere, and what made that one expensive
was that one of the two destroys. This is the same letter coming back to the
same screen with the other half of the objection absent.

**The alternative was `k`, and it is taken.** Tasks is `g k` in the navigation
map, which is the same collision answered the other way — `g t` was already
Today, so the view took the next letter of its own name. The two maps are
separate namespaces and each gets the letter that is free in it: here `k` is
the cursor, always bare in every mode, and stage one has a match list to move
through, so `k` is not free. Consistency between the two maps would have cost
the guessable letter in the one place a letter is guessed at.

`a` is now Add and nothing else, which is a small simplification and is not
what the move was for.

**`d` is the two-minute rule, and it was `2`.** The number was the rule's own
number and the argument for it was that `d` is Done app-wide, so the two would
read as the same answer. They *are* the same answer: this branch records
something finished, which is what Done means on every row, every action's page
and the doing screen. One noun, one meaning, and it is the rule `a` is already
kept by — the pair is never on one screen, because stage one carries no rows
for a row key to act on. What moved it is the match list needing the digits
(below); what makes the move right is that the letter was always the honest
one.

**The digits `1`…`9` press the match list**, where the processing screen shows
what a capture looks like (design.md, "Matches while processing"). They are
the list's own numbering, drawn on its rows, and pressing one copies that
finished item into the branch's form. They are bare and not chorded:

- **a digit is not a letter, so no mode touches it**, which is what a key on a
  numbered list has to be — the numbers are printed on the rows, and a list
  whose keys changed shape with `keys.mode` would be printing something that
  is not true in two of the three
- **`^1`…`^9` are the bookmarks and stay the bookmarks.** Those are offered
  only where a screen has a filter line and this screen has none, so the
  chords press nothing here — but a second meaning for them was still the
  wrong answer while a bare digit was free
- **the bar shows the range and not the nine**, `1…3 copy`, which says how to
  steer rather than pressing anything — the shape `^1…9` already has. Nine
  entries saying "copy" would bury the six answers under a list of keys the
  screen has already drawn beside the rows they act on

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
| `g` + letter | a view — the fourteen are the table below |
| `g` + `1`…`9` | the bookmark kept under that digit, view and filter both |
| `g g` | the capture dialog |
| `z` | Inbox Zero over the whole inbox |
| `w` | the selected action, alone on the doing screen |

**The fourteen jump letters live here, and lived in implementation.md until
now.** A letter that opens a view is spent exactly as hard as a letter that
presses a button — `t` is Today's jump and Today's mark, `d` is the Dashboard's
jump and Done, `r` is Review's jump and the review mark, and not one of those
collisions is visible from a file holding only one half of the pair. That is
the argument this file was split out on ("Why this is its own file"): while the
map was in two places, the same letter could be spent twice without either
place being wrong. A map that stopped at the nav rail was the same mistake one
level down. implementation.md keeps how the overlay is drawn and why the rail
carries an empty gutter for it; which letter is which is a key, and a key is
written down here.

| View | Key | View | Key |
| --- | --- | --- | --- |
| Inbox | `i` | Someday/Maybe | `s` |
| Today | `t` | Scheduler | `h` |
| Next actions | `n` | Review | `r` |
| Projects | `p` | Archive | `a` |
| Tasks | `k` | Audit | `u` |
| Waiting for | `w` | Dashboard | `d` |
| Calendar | `c` | Settings | `e` |

The letter is the view's own first where that was free and a distinct fallback
where it was not, and the fourteen are unique among themselves. They are
written lowercase here because lowercase is what is pressed; the overlay draws
them as uppercase badges, which is a styling decision and is argued for in
implementation.md, "Navigation".

**A jump letter may collide with a button letter, and the prefix is what is
supposed to make that safe.** Four of the fourteen are also buttons — `d` is
Done and the Dashboard, `t` is the Today mark and the Today view, `r` is the
review mark and the Review view, `w` is doing and Waiting for. The rule that
allows it is that the two are never offered in the same breath: pressing `g`
puts the keyboard in a state where only a jump can follow. Without the prefix
these fourteen would have had to come out of the letters the buttons had not
already taken, and there are not fourteen of those.

**Those four do not yet behave that way, and writing the map down here is what
showed it.** In `hybrid` and `command` modes the row commands are read before
the pending `g` is, so with a row selected `g d` completes it instead of
opening the Dashboard, and `g t`, `g r` and `g w` go the same way. In
`modifier` mode the four are correct, because the buttons want ctrl there and a
bare letter falls through to the jump. This is the collision the file was split
out to catch — while half the map sat in implementation.md, no one place held
both meanings of `d` at once — and it is a bug in the code rather than a
decision to revisit: the rule above is what the app is supposed to do. It is
listed again in "What is built".

**Globals** — ctrl in every mode.

| Key | What |
| --- | --- |
| `^e` | show me the time: the ages on every list, and the timer on the doing screen |
| `^o` | follow a link in the item under the cursor |
| `^m` | jump to a control on the screen already open |
| `^f` | the filter line, and a second press takes it and every filter away |
| `^1`…`^9` | the nine bookmarks: keep the filter on the screen, or go to the one kept |
| `^0` | the nine of them, on the screen |
| `^v` | the panel chooser; a second `^v` presses zen |
| `^↵` | submit the form being typed in |
| `?` | the view's own help |
| `esc` | unwind one step — see below |

**The digits are one key with two answers, and the filter line decides
which.** With a filter up, `^3` keeps it; with no filter up, `^3` goes to what
is kept there. That is not two meanings on one key — which the file argues
against everywhere else — but one idea said from whichever end you are
standing at: this digit and this filter belong together. Two keys would have
cost eighteen chords and a rule about which is which, to say the same thing.

They are the app's rather than a screen's, so they take ctrl like every other
global — and a digit is not a letter, so no mode touches them (see "The map"
above, and the rule that `⌫` and `2` are themselves in all three). The
processing screen's `2` therefore stays bare in every mode, which is what
keeps it out of the bookmarks' way: a `^2` there would have been the same
chord twice.

Both are offered only where the screen has a filter line, because the chord's
other half is keeping the filter that is up and there is none on the Inbox —
the same rule that stops the bar advertising anything else that would do
nothing.

**A bookmark now names its own view, so it is also a `g`.** `g 1`…`g 9` open
the bookmark under that digit — its view, with its filter on it — and that is
the one half of the pair that needs nothing from the screen it is pressed on,
so it works everywhere, the Inbox and the review included. A digit is free
after `g` because every jump is a letter — one per view, and there is no
fourteenth view wanting a number — so nothing had to be given up for it.

The two are not two meanings: `^3` with a filter up *makes* a bookmark and is
pressed with your hands in the filter line, `g 3` *goes to* one and is pressed
with your hands anywhere. Making one where there is nothing to make it from is
not a thing to spend a second key on, and going somewhere is the app's most
ordinary move and already has a key — so each half sits under the key its own
half of the job already belongs to.

**A `g` on an empty slot does nothing and says nothing.** `g 4` with slot 4
empty spends the press: no message, no empty view. That is the standing rule
rather than an exception to it — a key with nothing to do is never offered,
and "there is nothing under 4" is not news to whoever pressed 4. While `g` is
armed the bar says `1…9 a bookmark` only when some slot is full, so the offer
and the answer cannot disagree.

**`^0` is a list, and inside it the digits are bare**, meaning exactly what
they mean outside: keep this filter here, or go to what is here. `j` `k` move,
`↵` is the digit of the row under the cursor, `⌫` empties a slot, and `esc`
closes. All nine are shown whatever is in them — design.md, "Bookmarked
filters" says why the empty ones are part of the answer — and a full row reads
as the view it opens and then the line it opens it with.

`^e` is *elapsed*, which is the one word that covers both halves of what the
key means: an age is elapsed time and a timer counts it. It has to cover both,
because "show me the time" must never be two keys or one key with two answers
— it took `^t` for that reason, and gave it up because Today is pressed many
times a day and a display flag is pressed occasionally.

## Buttons that get no letter

One, now: **Recapture** on the audit. It is reached with `^m c`, which is what
`^m` is for — `g` goes to a view, `^m` goes one level in.

The other four — **Detach**, **Promote**, **Undone** and **Inbox** on a someday
item — are in the map above. They moved because the tier stopped working: `^m`
hangs its letters on the controls of the open screen, and with the buttons
drawn in the bar rather than on the form there is no longer a button to hang
one on. A letter each was the honest answer, and it cost less than it looked
like it would — `x` and `u` were free, and the two that were not turned out to
share a noun with the letter that held them.

There were five. **Parked / Next** held `n`, and it went when `#parked` did:
what it toggled was an action's availability said as a bare state, and what
says it now is a snooze on the meta line, which names the reason — a date, or
the sibling this action comes after. A button cannot name a sibling. So `n` is
free, and deliberately left so rather than spent on something else while the
fingers that used it are still finding that out (design.md, "Time fields").

Recapture is the one that could not follow them, and the reason is worth
writing down: it is a control **on a row**, one per line of the audit, and the
audit's rows carry no cursor. A standing letter means "do this to the thing
under the cursor", and there is no cursor here to mean it about — so it stays
a button on its row, where the pointer and `^m` can both reach it. That is the
same reason every list row keeps its own Done and Today buttons: a row control
is per-item, and the bar's entries are per-screen.

What made the old tier untrustworthy was not that it existed but that its
letters moved: `^m l` deleted an action inside a project and `^m e` deleted one
standing alone, because Detach vanished from the screen and every letter after
it shifted up. So **a control may declare its jump letter**, and a declared
letter is claimed before any computed one. That escape hatch stays, unused by
all but Recapture today, because it costs nothing and it is the thing that
stopped the shifting. Everything that does not declare one still takes the
first free letter of its own name, which is what makes the remaining jump
targets — boxes and lists, mostly — guessable; see implementation.md,
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

## A key that does nothing, on purpose, for a sixth of a second

**`d`, `⌫` and `b` are deaf while the moment they started is being shown.**
They answer the press and do nothing with it. This is the only place in the
map where a live key is deliberately inert, and it is worth its own rule
because "the key works but did nothing just then" is exactly the shape of a
bug — it has to be a decision written down rather than a thing discovered.

What it is for: those three each leave a screen and bring another one of the
same shape back, so the answer looked like the question and the press got made
again — and on the two that destroy something, the second one landed on an item
nobody had read (design.md, "A moment that shows itself"). The motion is what
stops the press from being *wanted* twice. The deaf window is what makes it
harmless when it comes anyway, which is the half that has to be true whether or
not anybody was looking.

How long: `anim.ms` from the settings file, and then on until the answer has
replaced the page — a leaving effect ends with the item invisible but still on
the page, and a press landing while the request is in flight would find the
same form and post it a second time. `anim.ms = 0` means no motion and no deaf
window either: that line is the app as it was.

`t` is not in this rule and must not be. It changes a tag on an item that stays
exactly where it is, nothing leaves the screen, and pressing it twice is a
thing you meant.

## What is built

All of it. `keys.mode` defaults to `hybrid`, which is what the app did before
any of this, so nothing about the trial is a one-way door.

**Except the prefix guard on four letters.** `g d`, `g t`, `g r` and `g w` are
claimed by the row commands before the pending `g` is looked at, so with a row
selected they press the button instead of jumping — in `hybrid` and `command`
modes, which includes the default. `rowCommand` is called ahead of the
`gPending` branch in the key handler, and it has to be, because in `modifier`
mode `d`, `t` and `b` arrive with ctrl held and the ctrl guard below would drop
them. The fix is for the pending `g` to be answered first, not for either map
to give up a letter — see "The map" for why the collision is allowed at all.

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
  reaching past it. Which puts the weight on the cursor only ever being
  somewhere you put it — a row the app selected on your behalf would silently
  take the screen's own keys off the bar. That is what the row handover is
  careful about, and where it was not careful enough is written up in
  implementation.md, "Keyboard": a project page reached by completing an
  action used to arrive with the completed action under the cursor, and so
  with no `d done` on it at all.

  **A row may also be what the screen is about, and then its forms are the
  screen's too.** It says so with `data-kb-subject`, and the project's page is
  the one screen that has one: the next action is shown open in its own boxes
  there (design.md, "Editing items"), with the ☐ and the ● it carried as a
  row on the heading above them. Those marks answer `d` and `t` with nothing
  selected as well as with the heading under the cursor — walking onto a
  heading to tick off the one action the page opened in order to show you is a
  press that asks the cursor for permission.

  It is still a row, and that is the point of keeping it one: `j`/`k` reach it,
  `↵` opens the action's own page, where Detach, Promote and Delete live, and
  `w` starts doing it. What `data-kb-subject` adds is that the screen's keys no
  longer have to wait for the cursor to arrive.

  **And `d` means one thing on that page: finish what is in front of you.** The
  next action while there is one, and the project once there is not. Not two
  Dones told apart by whether a row is selected — the project's own Done is not
  even on the screen while it has open work, because internal/app refuses to
  complete a project that has any (design.md, "Completing a next action"). So
  the letter is not spent twice and never was: there is one Done on that page
  at any moment, and completing the last action is what changes which one.
- **and that is what lets `t` be the task branch.** The three are read before
  the declared keys, so `t` asks for a `kb-pick` first and only then for a
  control on the page. Stage one of processing carries no such form — the
  match list is rows of text and a digit, with no mark to press — so the ask
  comes back empty and the branch takes the key. Nothing about the branch is
  special-cased: the ordering was already there, and a screen with a Today
  mark on it could not have taken `t` for anything else, which is the same
  thing as saying the two are never on a screen together.
- **`kb-review` is a fourth row form, and the one that does not leave.** `r`
  presses it, and it is the only one of the four whose answer is a row redrawn
  where it stands rather than a screen replaced — so it takes no deaf window
  (there is no motion to cover) and no screen form (no screen presses it).
  Both the key and a click on the mark go through the same flip, for the
  reason every other pair does: one path, so the letter and the pointer cannot
  come to mean different things. See implementation.md, "The weekly review
  screens".
- **`data-jump`** lets a control name its own `^m` letter, claimed before any
  computed one, which is what stopped Delete moving between `l` and `e`. Only
  Recapture still uses it, and it stays for the reason above.
- **the bar's entries are the controls.** An entry that presses something is a
  real button wired to the control the letter presses, through the same
  `press()`; one that steers is not a button at all. `keys.bar_style` in the
  settings file chooses how loudly the difference is drawn, from `plain` to
  `button`, and defaults to `chip` — see implementation.md, "The key bar is
  the buttons".
- **`renderKey` leaves a key that is not a letter alone**, in all three modes.
  It did not: `modifier` turned every declared key into a chord, so a declared
  digit became `^`-something there — which the map above already said it
  should not, and which would be the bookmark key as well. The map was right
  and the code was wrong; nothing else in it changed. It is what lets the
  match list draw its numbers on its rows and mean them.
- **a numbered list is one entry in the bar, not nine.** A control marked
  `data-key-quiet` still answers its key and is left out of the bar, and a run
  of them is drawn as the range they cover — `1…3 copy` — which presses
  nothing, exactly as `^1…9` does. The run ends at the first ordinary key, so
  the bar can never claim a range that is not one (implementation.md, "The
  match list").

- **`h` is on the answer, not on the row.** The Settings screen renders one
  form per theme and hangs the letter on the one the next press gives, so the
  bar's entry is derived from the page like every other entry and says the
  answer it lands on. Nothing in the key layer knows what a theme is
  (implementation.md, "Theme").

- **the Dashboard is the first view with no keys of its own**, and that is
  the answer rather than a gap. It holds no rows to move through — `j`/`k`
  have nowhere to go — and nothing on it can be completed, picked or deleted,
  because nothing on it is an item (design.md, "Dashboard"). So its key bar is
  the globals and the steering entries and nothing else, which is the bar
  doing exactly what it is supposed to: nothing advertises a key that does not
  exist. The one thing on the screen that *is* pressable is a link — a row of
  "the oldest thing in each view" — and it is reached the way every other link
  on a screen with no cursor is reached, with `^m`.

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
