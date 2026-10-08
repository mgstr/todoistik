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
  not.** `j k move`, `g go to`, `m jump`, the "needs …" line: these are how
  you get somewhere rather than things you press, and the bar must not invite
  a click on them. The promise "everything listed works" would otherwise be
  true of the letters and false of the targets.
- **every control needs a letter now, including the rare ones.** A button with
  no letter used to be reachable anyway, by `m` and a hint hung on the button
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

## The two modes

A bare letter cannot be a command where the hands are — it would be typed. The
app has always split the keyboard on that fact. It used to split it three ways
and let a settings line choose between them; it splits it one way now, and it
is the way vim does:

- **command mode** — every key is the app's, and every key is bare. `d` is
  Done, `s` is Save, `f` is the filter line. A screen is in this mode whenever
  the caret is not in a box.
- **insert mode** — the caret is in a box, and every key is the box's. A
  letter is a character and a digit is a digit, and nothing the app owns
  fires: not Save, not the filter line, not a snippet. The one key that is
  still the app's is `esc`, which takes the caret out of the box and gives the
  letters back.

**The mode is where the caret is, and nothing else.** There is no flag behind
it and no key that sets it: putting the caret in a box *is* entering insert
mode, however it got there — a click, `tab`, the `m` jump, or a screen that
opens on the box it wants written in. A mode kept as a variable can come to
disagree with the screen, and that disagreement is the classic modal bug: the
letters going somewhere other than where the eye says they will. A mode that
is only ever read off the caret cannot have it.

The app already arranged arrival correctly before it had a name for this:
screens you arrive at to *write* focus a field (the processing branches, a new
action, promote), and screens you arrive at to *act on* do not (an action's
page, a project's page), so the keys are live on arrival exactly where the
keys are what you came for.

**Nothing is a chord.** Ctrl used to be spent twice: on the controls that had
to fire in the middle of typing — Save, Create, Add — and on the keys that
belonged to the app rather than to a screen — the filter line, the jump, the
panels, the nine under the digits. Both were answers to one question, *how is
this reached without leaving the box*, and the answer now is that the box is
left: `esc`, then the letter. That is two presses on the home row against one
that takes the hand off it, and the trade is the right way round because of
which keys were paying. A chord is the uncomfortable press, and it was sitting
on the keys pressed most — saving a form, opening the filter — while the bare
letters went to the ones pressed least.

So a key that arrives with ctrl, cmd or alt held is never the app's, and is
left to the browser and the system. That hands back every readline binding in
a text box (`^a`, `^e`, `^k`) and every accelerator the browser has, without
the app having to know which of them this machine uses — the older scheme was
viable only because those bindings happened not to be in use here.

**The page says which mode it is in by changing its ground.** In insert mode
the background of the whole page turns warm; in command mode it is the
theme's own. Not a marker in a corner, because a marker is read only when it
is looked for, and *will this letter be typed or obeyed* is a question asked
without looking — the ground is the one thing in view wherever the eye is.
And not a drastic change: it has to be told apart at a glance and then worked
on for a paragraph, so it is as far from the ordinary ground as paper is from
a screen and no further (implementation.md, "Keyboard"). The bar says the same
thing a second time, by emptying — see "The bar while you are typing".

**What still works in a box is what a box does by itself.** `↵` in a one-line
box submits its form, as it does in any browser, and adds in the capture
dialog, where `shift-↵` is the newline. `↓` `↑` `↵` and `tab` work a token
box's completion list, and `↵` on the filter line asks about a name it does
not know. None of these is a shortcut of the app's: they are the box being a
box, and a text box that did not answer enter would be broken rather than
modal. The rule is that insert mode has no key *of the app's* in it but
`esc`, and these were never the app's.

**A dialog with boxes in it has no command mode.** `esc` in a dialog closes
the dialog — that is the dialog's own step of unwinding, and it comes before
the caret's — so there is no state in which a letter could be pressed at one.
A dialog is finished with `↵` from any one-line box, and from a multi-line
one with `tab` to its button and `↵` there. That last case is what `^↵` used
to be for, and it is the one place its going costs a press that `esc` does
not give back; it is paid in the add-action and new-project dialogs, in the
one box of each where enter has to stay a newline.

**A control declares its letter, and that is all it declares.** `data-key="d"`.
There was a second attribute, `data-key-typing`, for a control that had to be
reached with the caret still in a box; no control is reached that way now, so
there is nothing for it to say.

**What each chord became.** Written down once, because fingers that knew the
old map will look for it here:

| Was | Is | |
| --- | --- | --- |
| `^s` `^c` `^a` | `s` `c` `a` | Save, Create and Add, from command mode |
| `^e` | `e` | show me the time |
| `^o` | `l` | follow a link — see below for why not `o` |
| `^m` | `m` | jump to a control |
| `^f` | `f` | the filter line |
| `^v` | `v` | the panel chooser, and `v` again for zen |
| `^0` | `0` | the nine of them, on the screen |
| `^1`…`^9` | `1`…`9` | go to a bookmark, or write a snippet; *keeping* a filter moved into the `0` list |
| `^j` `^k` | — | from the filter line into its list: `esc`, then `j` |
| `^↵` | — | finish the form from inside a box: `esc`, then `s` or `c` |

**`l` and not `o`, because `o` was already a key.** `o` opens the selected row
— the item's own page, inside the app — and following a link goes the other
way, out to a browser tab, from the very same row at the very same moment
(design.md, "Following a link"). While one of them was a chord they could
share a letter; bare, they cannot. The link takes the new letter because it
is the one that had a modifier to lose, and `l` is the first letter of what
it follows and was free everywhere.

**The bar says `o`, and `↵` still works.** A row's entry used to read
`↵ open`, which advertised the worse of the two keys and hid the better: `↵`
is a reach off the home row and `o` is under a finger, next to the `j` and `k`
that got the cursor there. The bar has room for one, so it shows the one worth
learning — on every row entry, `o open`, `o process`, `o edit`,
`o new project` and `o pick` alike, since it is the same key doing the same
thing. `↵` is not taken away, because a list row answering Enter is what
anyone expects without being told. Inside a box or a dialog the bar still
says `↵`: there `o` is a letter being typed.

**`keys.mode` is gone.** It chose between `command`, `modifier` and `hybrid`,
and it existed because the question was about hands and could only be settled
by working in each. It has been settled, and further than any of the three
went: `command` was bare letters for a screen's own buttons with the app's
own keys still on ctrl, and this takes ctrl off those as well. Keeping the
flag would have meant keeping two keyboards buildable, each with its own
answer for every row of the table above. A settings file that still carries
the line is refused at startup, like any other setting the app does not know.

## The map

**Buttons** — each one declares its letter.

| Key | Button | Where |
| --- | --- | --- |
| `s` | Save | action, project, someday item, reference item, schedule |
| `c` | Create | the processing branches, new action, promote, new schedule, settings, the draft and new-project dialogs, scheduler |
| `a` | Add, and Action | project, promote, the project branch of processing — the actions *after* the first, which is open on the form and not added. It opens the dialog and writes a row into the plan on all three: adding an action never leaves the screen. On the processing question it is the Action branch, which is the same noun doing the same thing — see "The processing branches" |
| `b` | Back | every screen that can be left — see "Leaving a screen" |
| `d` | Done, and Undone | every list row, action, project, doing — on a project's page it finishes the next action while there is one and the project once there is not. On a completed action's page and a completed project's it is Undone, which brings the item back |
| `r` | the review mark | a row of a weekly review step |
| `n` | make this the next action | a row of the plan on a project's page, and nowhere else |
| `t` | Today | every list row that carries the mark, an action's page, a project's next action, and every screen that *writes* an action — the processing forms, the screen a project is created on, Create action, promote, the add-action dialog, and a stalled project's empty boxes, where there is no action yet to post against and the key flips `#today` in the line being typed (design.md, "#today") |
| `⌫` | Delete | every row that carries one — including a Reference row, which is the one list view that carries a delete — action, project, reference item, schedule, a capture on the processing screen, a draft row |
| `x` | Detach | an action's page, inside a project |
| `p` | Promote | a standalone action's page — the same `p` as the Project branch below |
| `i` | Inbox | a someday item's page |
| `h` | Theme | the Settings screen's theme row |
| `#` | the project's tags, onto its next action | a project's page, on the "Next action" heading |

**Undone is `d`, and it was `u`.** It moved to make room: `u` is Undo now, on
every screen (see "The app's own keys" below), and one letter could not be the
last press taken back everywhere and a completed item brought back on the two
pages that hold one. Where it went is the letter it arguably always belonged on.
Done and Undone are one mark said from both sides — "a state toggle is named
for the state, not for the act" is implementation.md's rule for the pair
("Button labels") — and that is what `r` already is for the review mark and
`t` for the pick: one key, and the bar says which way this press goes,
`d done` or `d undone`. The two are never on a screen together, because a
completed item's page is the one place an item has no Done.

- **it is still a declared key, not a row key.** `d` asks the row under the
  cursor for a Done first and the screen second, as it always has, and a
  completed item's page has neither, so the ask comes back empty and the
  declared key takes it — the ordering that already lets `t` be the task
  branch (see "What is built")
- **pressing it lands you on a screen where `d` is Done**, the same item, open
  again. That is the one hazard the move brought with it, and it is answered
  where the others of its kind are: see "A key that does nothing, on purpose,
  for a sixth of a second"

**`n` is spent at last, and on the thing it used to mean.** It held Parked /
Next and went when `#parked` did, and was left free rather than reused while
the fingers that knew it were still finding that out (see "Buttons that get no
letter"). What it presses now is the other half of the same idea: the old key
said *this action is available*, and this one says *this action is the one*. A
project has one next action and it is chosen (design.md, "Project"), so there
is a press to make, and `n` is the first letter of the only word for it.

- **it is a row's key and never a screen's.** It acts on the row under the
  cursor, which is how a plan is walked — `j`/`k` down the list, `n` on the one
  that should be next — and there is no screen-level answer to it, because what
  a screen-level `n` would be about is the action already in the boxes at the
  top of the page. That action is next. There is nothing to press.
- **it is offered only where it would do something**, which here means three
  things at once: the row is in a plan, it is not the action already next (that
  one is not in the list at all), and it is not snoozed — a snoozed action
  cannot be started, so *next* is a claim the snooze contradicts. The mark is
  drawn or it is not, and the bar reads the mark, so the key and the offer
  cannot come to disagree.
- **`g n` is still the Next actions view**, and that is the prefix doing its job
  rather than a collision: pressing `g` puts the keyboard in a state where only
  a jump can follow. It is the fifth letter shared between a button and a jump,
  and the pair is the friendliest of them — both mean "the next action", one
  showing you the list and one deciding what is on it.
- **the panel chooser's own `n` is untouched.** A dialog is a menu of its own
  letters and the row keys stand down inside one, which is the same rule that
  keeps the unknown-name dialog's `n` for *create it*.

**`r` is one of the three letters in the map that are spent twice, and it is
worth saying why rather than pretending otherwise** — `t` and `a` are the
others, and both are argued for in "The processing branches" below, by this
same paragraph's test. It is the processing screen's
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
step whose number is printed beside it (design.md, "Weekly review"). The
bookmarks are under the same digits and are offered only where a screen has a
filter line, which this one has not. `1` is Gather and presses nothing, because Gather has no screen —
the line keeps its number, since the number is its place in the order, and the
bar simply does not offer a key for it. That is the standing rule doing its
job rather than an exception to it: nothing advertises a key that does not
exist.

**The bar shows them as a range, `2…7 step`**, the shape the match list's
`1…3 copy` and the bookmarks' `1…9` already have — an entry that says how to
steer rather than one that presses. The numbers are printed on the lines, which
is where a numbered list's keys are read; the bar listing all six would be the
screen written out a second time, in the one place that is supposed to say what
is *not* on the screen. It starts at 2 and not at 1 for the reason above: a
range is a claim about what can be pressed.

**The Dashboard's digits.** `1`, `2` and `3` open its three forms — Traffic,
Queue and Inbox zero (design.md, "Dashboard") — and the number is printed
beside each name on the line at the top of the screen, which is the review's
arrangement used a second time. They are free for the reason the review's are:
the bookmarks and the snippets live under the same digits and are offered only
where a screen has a filter line or a meta line, and this one has neither. The
bar shows them as `1…3 form`.

`j` and `k` step through the same three and wrap at the ends. Both ways of
getting to a form are kept because they are different questions — a digit is
"that one", and `j` is "the next one" — and the second is the one asked when the
forms are being read through in turn.

**`#` is the one button in the map that is not a letter, and it is the
notation it writes.** The mark it presses puts the project's tags on its next
action (design.md, "Editing items"), and what a tag is written as is `#name` —
so the key is the thing it does, which is a stronger claim to a place on the
keyboard than the first letter of a word describing it. `m`, for Meta, was the
obvious letter and the weaker one: it names the field, and the field is not
what moves.

Nothing collides. The digit under it is spoken for twice already — the
review's `1`…`7` and the nine bookmarks and snippets under `1`…`9` — and all
of those are pressed without shift, so the shifted place was free.

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

**Detach, Promote, Undone and Inbox are the tier that used to have no
letters**, with Parked / Next the fifth while there was one. They are rare,
deliberate acts and they were reached with `m` and a declared letter, which
worked only while the button was on the screen to hang a hint on. Three of them
share a letter with something already on the map, and all three share the
*noun*, which is what the rule asks: `p` is Promote here and the Project branch
on the processing screen, and both mean "make this a project"; `i` is Inbox,
and the Recapture button on the audit means the same thing — send this to the
inbox; `d` is Undone and Done. No pair is ever on one screen. `n` and `x` were
free, and so was `u`, which Undone held until Undo needed it.

**The processing branches** are seven answers to one question rather than
seven controls on a screen — see implementation.md, "The processing screen".
They obey the same rule as everything else here; three of them had to move to
do it.

| Key | Answer |
| --- | --- |
| `⌫` | delete it |
| `r` | reference material |
| `d` | the two-minute rule |
| `y` | someday/maybe |
| `t` | make it a task |
| `a` | make it an action |
| `p` | make it a project |

**`t` is Task and `a` is Action, and they are two answers because they make
two different things** — a standalone action and a step of a plan (design.md,
"Inbox Zero"). They were one branch, `t`, with a project box inside its form;
`a` had been that branch's letter while it was called Action, and it comes
back to the same screen now that there is an Action to press again.

The letters follow the nouns, which is the whole of the rule: a standalone
action is a *task* everywhere it is afterwards read — the view it lands in, the
nav entry, the word design.md uses for it (design.md, "Tasks") — and one inside
a project is an *action*. Both are the first letter of their own word, on the
one screen whose job is naming what a thing is.

`t` is the second letter in the map spent twice, and it passes the test the
`r` paragraph sets: Today and Task are never on a screen together — stage one
carries no row with the mark on it, and every screen that does carries no
branches — neither destroys anything, and both are the first letter of their
own word. There is a sharper thing to say about it, since this file spends two
paragraphs on `t` being the collision that had to go: what had to go was `t`
meaning *trash* here and *today* elsewhere, and what made that one expensive
was that one of the two destroys. This is the same letter coming back to the
same screen with the other half of the objection absent.

**The two are now one press apart, and that is still not the collision.**
Pressing `t` on the question means Task and lands you on the form it opens,
where `t` means Today — two meanings a keystroke apart in time. What the rule
forbids is two meanings a keystroke apart *on a screen*, where the hand has to
know which one it is about to get; here the screen has changed underneath it,
the bar says which of the two it is offering, and the second press is a tag on
the very item the first press decided to make. Neither destroys anything, so
the worst a slip costs is a dot you press again.

**The alternative was `k`, and it is taken.** Tasks is `g k` in the navigation
map, which is the same collision answered the other way — `g t` was already
Today, so the view took the next letter of its own name. The two maps are
separate namespaces and each gets the letter that is free in it: here `k` is
the cursor, and stage one has a match list to move
through, so `k` is not free. Consistency between the two maps would have cost
the guessable letter in the one place a letter is guessed at.

**`a` is the third letter in the map spent twice, and it is the cheapest of
the three.** It is Add on a project form and Action on the processing
question, and unlike `r` and `t` the two are barely two nouns: both make an
action, one into a plan that exists and one into a plan being chosen. They are
never on a screen together — stage one carries no form to add a row to — and
neither destroys anything. The test the `r` paragraph sets is passed on every
clause at once, which is what "one letter, one *noun*" was written to allow.

**`r` opens a form now, and the letter did not move.** The branch used to post
and be done with it: the item left the app, and there was nothing to write
because nothing was kept. Reference material is kept in the app now (design.md,
"Reference item"), so the press lands on a form with the material and its area
on it, and `c` creates — the shape `y` already has. Nothing here changes: a key
presses a control, and whether that control posts or opens the next screen is
not something the map has an opinion about. What it does change is which keys
are on the screen after the press, and they are the form's: `c` create, `esc`
back to the question.

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
finished item into the branch's form.

- **the bookmarks are under the same digits and are not here.** Those are
  offered only where a screen has a filter line, and this screen has none; the
  snippets want a meta line, and stage one has none of those either. A
  declared key is read before either would be, so the list's numbers mean the
  list even if that ever changed (see "A digit a screen declares for itself
  wins.")
- **the bar shows the range and not the nine**, `1…3 copy`, which says how to
  steer rather than pressing anything — the shape the bookmarks' `1…9` has.
  Nine entries saying "copy" would bury the six answers under a list of keys
  the screen has already drawn beside the rows they act on

`p` is free because the row key `p` is gone. It only ever aliased `↵`: an
inbox row's link already goes to the processing screen, which is why the bar
showed the two sharing one label. An alias is the cheapest thing in the map to
spend, and dropping it makes nothing unreachable.

`y` is what is left of "Someday/Maybe" once every other letter of its own name
is spent — `s` Save, `o` open, `m` the jump, `e` the time, `d` Done, `a`
Action, `b` Back. It is a poor mnemonic and the letter that was free. The
branch that had to give way is the right one: Save is on four edit screens,
and this one is pressed once per idea you decide not to commit to.

| Key | Goes to |
| --- | --- |
| `j` `k` | through the current list. On a screen with no list they move the window through its sections instead, a heading at a time — one key for "the next thing down", whether the next thing is a row or a screenful. On the Dashboard, which has neither, they go to the next and the previous form |
| `↵` `o` | open the selected row — a row of a plan that is not saved yet opens in the dialog it was written in, since there is no page for it to have, and the project picker's last row opens the dialog a new project is named in, since there is no project for it to open |
| `g` + letter | a view — the fifteen are the table below |
| `g` + `1`…`9` | the bookmark kept under that digit, view and filter both |
| `g g` | the capture dialog |
| `z` | Inbox Zero over the whole inbox |
| `w` | the selected action, alone on the doing screen |

**The fifteen jump letters live here, and lived in implementation.md until
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
| Today | `t` | Reference | `f` |
| Next actions | `n` | Scheduler | `h` |
| Projects | `p` | Review | `r` |
| Tasks | `k` | Archive | `a` |
| Waiting for | `w` | Audit | `u` |
| Calendar | `c` | Dashboard | `d` |
| | | Settings | `e` |

**Reference is `f`, and it is the third letter of its own name.** `r` is Review
and `e` is Settings, both spent long before this view existed, so it takes the
next letter of "reference" that is free — the derivation `y` and `h` already
have, and the same one this table's own sentence describes. `m` for "material"
was the other candidate and the weaker one: the view is called Reference
everywhere else in the app, and a jump letter is guessed from the name on the
rail.

The letter is the view's own first where that was free and a distinct fallback
where it was not, and the fifteen are unique among themselves. They are
written lowercase here because lowercase is what is pressed; the overlay draws
them as uppercase badges, which is a styling decision and is argued for in
implementation.md, "Navigation".

**A jump letter may collide with a button letter, and the prefix is what is
supposed to make that safe.** Five of the fifteen are also buttons — `d` is
Done and the Dashboard, `t` is the Today mark and the Today view, `r` is the
review mark and the Review view, `w` is doing and Waiting for, and `n` is the
next-action mark and the Next actions view. The rule that
allows it is that the two are never offered in the same breath: pressing `g`
puts the keyboard in a state where only a jump can follow. Without the prefix
these fifteen would have had to come out of the letters the buttons had not
already taken, and there are not fifteen of those.

**They did not behave that way until the map was written down here, which is
the argument for this file making itself.** The row commands were read before
the pending `g` was, so with a row under the cursor `g d` completed it and went
nowhere, and `g t`, `g r` and `g w` went the same way. Nothing on the page
was wrong and no other test could see it; what made it visible was putting the fifteen
letters next to the buttons they share, in the one file whose job is to notice
that a letter is spent twice. The pending `g` is now answered before every
other key on the page, which is where the `m` jump was already read and for
the same reason.

**A chord pressed while the overlay is up is not a jump, and does not press the
button either.** No key of the app's is a chord, so `^d` after `g` is not the
Dashboard; it is spent taking the overlay away and doing nothing else, which is
the answer `m` already gives a chord pressed into its hints. Holding shift to
reach a key is not an answer at all and leaves the overlay standing.

**The app's own keys** — the ones that do not belong to a screen. Bare, like
every other key, and so live in command mode only.

| Key | What |
| --- | --- |
| `e` | show me the time: the ages on every list, and the timer on the doing screen |
| `l` | follow a link in the item under the cursor |
| `m` | jump to a control on the screen already open |
| `f` | the filter line, and a second press takes it and every filter away |
| `1`…`9` | the nine under the digits: go to a bookmark where there is a filter line, write a snippet where there is a meta line |
| `0` | the nine of them, on the screen — whichever nine this screen's digits mean |
| `v` | the panel chooser; a second `v` presses zen |
| `u` | undo: asks about the last thing done, and takes it back on `↵` |
| `?` | the view's own help |
| `esc` | unwind one step — see below |

**`u` is Undo, on every screen, and it opens a question rather than doing
anything** (design.md, "Undo"). It is the first letter of its own word and the
letter every editor has taught the hand, which is why it was worth moving
Undone off it rather than taking `q`, the one letter the map had left.

- **it is the app's key and not a screen's**, because what it acts on is the
  last press, wherever that was made. So it sits in the bar's right half with
  the keys that do not change from screen to screen, and it is the same letter
  on the Inbox, on an action's page and on the Dashboard
- **the question has the two keys every question here has.** `↵` takes it back
  and `esc` leaves everything as it is, and nothing else is live while it is
  up — a letter pressed at the question does not reach the page behind it.
  Unlike the unsaved-work question, `esc` here means what it means everywhere:
  one step of unwinding, and nothing lost (see "Leaving a screen")
- **more than one step is the same two keys again.** `u` `↵` `u` `↵` goes back
  two, each question reading out the step it is about. There is no key that
  takes back several at once, and no redo
- **it is offered only when it would do something**, like every entry in the
  bar: not while there is nothing to take back, and not on a screen holding
  unsaved work, where the way on is `s` or `b` first
- **`g u` is still the Audit view**, which is the prefix doing its job, as it
  does for `g d` and `g t`. The pair is close kin, too: one shows what was
  done and the other takes the last of it back
- **`m u` is unaffected.** The jump hands out its own letters to the controls
  of the open screen, after its own prefix

**A bare digit goes to a bookmark and never makes one.** `3` on a list view
opens what is kept under 3 — its view, with its filter on it — whether or not
a filter is up. Keeping the filter that is up is done in the `0` list: `0`,
then the digit.

It was one key with two answers, `^3` keeping the filter when one was up and
going to the bookmark when none was, and that was sound while the key was a
chord: a chord is pressed on purpose. A bare digit is one stray keystroke, and
with a filter up that keystroke would have written over a slot without
showing what was in it. So the half that destroys something moved to where
the nine are on the screen and the slot is read before it is written. It
costs nothing in presses — `0` `3` is two keys, and `^3` was two keys held
together.

**A digit a screen declares for itself wins.** The review's `1`…`7` and the
processing screen's match list are bare digits too, and neither screen has a
filter line or a meta line, so nothing is taken from either side. The order
is still stated, because it is what would decide if a screen ever had both:
a declared key is read before the standing ones.

**A bookmark names its own view, so it is also a `g`.** `g 1`…`g 9` open the
bookmark under that digit from anywhere, the Inbox and the review included —
the bare digit works only where the screen has a filter line, which is where
the bar offers it. A digit is free after `g` because every jump is a letter.

**A digit on an empty slot does nothing and says nothing**, bare or after
`g`. No message, no empty view: a key with nothing to do is never offered,
and "there is nothing under 4" is not news to whoever pressed 4. The bar says
`1…9 go to a bookmark` only when some slot is full, so the offer and the
answer cannot disagree.

**`0` is a list, and inside it a digit means the slot.** With a filter up it
keeps that filter there; with none up it goes to what is there. `j` `k` move,
`↵` is the digit of the row under the cursor, `⌫` empties a slot, and `esc`
closes. All nine are shown whatever is in them — design.md, "Bookmarked
filters" says why the empty ones are part of the answer — and a full row reads
as the view it opens and then the line it opens it with.

**On a screen with a meta line, the digits are the snippets.** `3` writes the
run of names kept under 3 onto the meta line, and `0` is the nine of them
(design.md, "Snippets"). That is not a second meaning for the digit: it is the
same idea the bookmarks state, applied to the other line the app has. A
filter line asks a question and a meta line writes something down, both are
typed daily, and both have half a dozen answers that come round again and
again.

**A snippet is pressed from command mode, like everything else.** It was the
one key whose whole job happened in the middle of typing — `^3` with the
caret in the line — and it is `esc`, then `3`, now. The line it writes onto
is the screen's meta line; where a screen has two, it is the first, which is
the project's (implementation.md, "Snippets"). What that costs is real and
is the price of the rule having no exceptions: one chord left in the map is
one key the hand has to remember is different, on a keyboard whose point is
that none is.

Which of the two nines a digit means is decided by the screen and never by a
mode. No screen carries both lines — an edit screen has no filter bar and a
list view has no meta box — so there is nothing to disambiguate and nothing to
remember: the digits do the thing the screen you are standing on is for. The
bar says which, because it says every key by reading the page.

**A snippet is written in the `0` list, and a digit in there opens its
slot.** `j` `k` move, `↵` opens the row under the cursor into a box — or its
own digit does, from wherever the cursor is — `⌫` empties a slot, and `esc`
closes: first the box, then the dialog. One row is open at a time. All nine
are shown whatever is in them, for the reason the bookmarks are (design.md,
"Snippets").

Inside an open row the box is the app's own token box and keeps its own keys:
`↓` `↑` through the completions, `↵` to take one. `↵` with no list up is done
with the row, and `esc` is the same — nothing is thrown away by either, since
a slot is written the moment the row is left. A name the app does not know
stops the row from closing and asks about it, exactly as the filter line does.

`e` is *elapsed*, which is the one word that covers both halves of what the
key means: an age is elapsed time and a timer counts it. It has to cover both,
because "show me the time" must never be two keys or one key with two answers.
`t` was the obvious letter and is Today, which is pressed many times a day
where a display flag is pressed occasionally.

## Buttons that get no letter

One, now: **Recapture** on the audit. It is reached with `m c`, which is what
`m` is for — `g` goes to a view, `m` goes one level in.

The other four — **Detach**, **Promote**, **Undone** and **Inbox** on a someday
item — are in the map above. They moved because the tier stopped working: `m`
hangs its letters on the controls of the open screen, and with the buttons
drawn in the bar rather than on the form there is no longer a button to hang
one on. A letter each was the honest answer, and it cost less than it looked
like it would — `x` and `u` were free, and the two that were not turned out to
share a noun with the letter that held them. Undone has since moved from `u`
to `d`, when Undo took `u`, and shares a noun there as well (see the map).

There were five. **Parked / Next** held `n`, and it went when `#parked` did:
what it toggled was an action's availability said as a bare state, and what
says it now is a snooze on the meta line, which names the reason — a date, or
the sibling this action comes after. A button cannot name a sibling. So `n` was
free, and was left so rather than spent on something else while the fingers
that used it were still finding that out (design.md, "Time fields"). It is
spent now, and on the nearer half of what it used to mean — see the map above.

Recapture is the one that could not follow them, and the reason is worth
writing down: it is a control **on a row**, one per line of the audit, and the
audit's rows carry no cursor. A standing letter means "do this to the thing
under the cursor", and there is no cursor here to mean it about — so it stays
a button on its row, where the pointer and `m` can both reach it. That is the
same reason every list row keeps its own Done and Today buttons: a row control
is per-item, and the bar's entries are per-screen.

What made the old tier untrustworthy was not that it existed but that its
letters moved: `m l` deleted an action inside a project and `m e` deleted one
standing alone, because Detach vanished from the screen and every letter after
it shifted up. So **a control may declare its jump letter**, and a declared
letter is claimed before any computed one. That escape hatch stays, unused by
all but Recapture today, because it costs nothing and it is the thing that
stopped the shifting. Everything that does not declare one still takes the
first free letter of its own name, which is what makes the remaining jump
targets — boxes and lists, mostly — guessable; see implementation.md,
"Jumping to a control".

## The bar while you are typing

With the caret in a box, every key is a character rather than a command. The
bar narrows to the one key that is still the app's — `esc leave the box` —
and the "needs …" line, which is a reason rather than a key. Everything else
goes.

It goes rather than being greyed out or annotated. The bar's one promise is
that what it lists works now; a key shown with a note saying it does not is
still a key shown, and `b back` beside a box you must press `esc` to get out
of is a plain lie about what one press does. There is nothing to add to make
that clear — there is something to remove.

Which makes the bar the second thing that says the mode, under the ground
(see "The two modes"). The ground says *which*; the bar says *what that
means*, by showing the whole map in one mode and one key in the other.

Where a box has keys of its own the bar says those instead: the completion
list's `↓↑ move`, `↵ take`, and on the filter line `↵ ask about …` while there
is a name to ask about. A dialog's bar offers `↵` with its button's own words
from any box where enter finishes it, and drops the entry in the one where
enter is a newline.

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

The unsaved-work question below is the one exception, and it is worth naming
as one: there `esc` discards and goes. It is not really a second meaning —
that dialog exists *because* a press was already on its way out, so closing
the question and letting the press through is still one step of unwinding.
But it is the one place where `esc` ends on a different screen, and the one
place where it can cost something, so it is written down here rather than
left to be discovered.

**`b` leaves.** Back rather than Cancel, because that is what the thing does
and what the code has always called it — `data-cancel` holds a URL, the
template variable is `Back`, the handler is `back()`. On an action's page or a
project's page nothing is being abandoned; you are returning to the view you
came from. Cancel stays the right word only where something is genuinely being
given up, which is the screens that create.

**A form with unsaved work asks before it is left.** The press does not
leave: a dialog comes up with the two answers there are — keep it, or lose it
and go. Keeping presses the screen's own button, so the answer reads Save,
Create or Promote depending on what screen you are on, and is offered only
when that button could be pressed.

**Both answers are a key, and they are the two obvious ones.** `↵` keeps,
`esc` discards — the same pairing every dialog in the app already has, where
enter takes the primary button and esc takes the way out. The question had
been the exception: its buttons were reachable by mouse or by tab and nothing
else, which in a keyboard app means the one interruption there is was also the
one place the keyboard stopped working. Both keys leave the screen, because
both are answers to a press that was already aimed somewhere; the question
resolves and the movement finishes.

Staying has no key any more. That is what giving `esc` to discarding costs,
and it is the right trade: the question is only ever asked on the way out, so
its answers are about the work and not about whether to move. But a modal
whose every answer navigates is a trap, so staying keeps the gesture that
needs no button — a click on the backdrop closes the question and leaves the
screen standing, boxes untouched. It is deliberately the slow way to answer:
the two presses are for the two decisions that were actually being made, and
changing your mind about leaving at all is the rare one.

It is the same question however the screen was being left — `b`, a link in the
rail, a row opened with `enter`, a `g` chord. Only one of those is a key, but
the rule is a key's business all the same, because `b` is the way out the map
names and the question is now part of what pressing it does.

The fields that differ from what was saved are marked while the question is
up, and the marking borrows the dashed border a draft row already wears,
because that is the app's existing way of saying "this is not saved yet". It
must not borrow the error styling: nothing is wrong, it simply is not written
down. A screen that *creates* gets no marks — it is unsaved wholesale, so
marking every filled box would mark the form and say nothing — and asks all
the same, because there it is everything typed that would be lost.

This replaced a second `b`: the first press used to mark the boxes and turn
the bar's entry into `b discard`, and the second went. Two presses of one key
were cheap to learn and quiet, which is why they were chosen — and they were
wrong about which of the two answers is the expensive one. Discarding is the
press that cannot be taken back, and it was the one the arming made easier;
keeping the work still meant noticing the bar had changed, going back to the
boxes and pressing Save. What the dialog costs is an interruption. What it
buys is that both answers are in front of you and the one that destroys
something is the one you have to reach for.

None of this is the app enforcing anything — see design.md, "A screen with
unsaved work on it says so, and asks before it is left." for why that is a
different question from "The protocol is followed, not enforced".

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

**Undone is in it, from the other side.** `d` on a completed item's page brings
the item back, and the screen that answers is the same item with Done on the
same key — the answer looking like the question again, and a doubled press
would finish what was only just reopened, stamping it completed today. Nothing
leaves the screen, so there is no motion to play; what the press gets is the
window alone, on the page that arrives: the three keys are deaf there for
`anim.ms`, and not at all when that is 0.

`u` needs none. Its second press opens a question, and the question's `↵` is
answered by a screen with no question on it.

## What is built

All of it.

A handful of mechanisms carry the whole map, and each one exists so that a
key can never be advertised without working:

- **one line decides the mode.** The key handler asks whether the event came
  from a box that takes text; if it did, `esc` blurs it and every other key is
  left alone, and nothing below that line is read. Everything below it is
  command mode, and the first thing it does is drop any key with a modifier
  held. So "insert mode has no key of the app's but `esc`" and "nothing is a
  chord" are each one line of code rather than a property every key has to
  remember to have (implementation.md, "Keyboard").
- **the ground is read off the same question.** A class on the document says
  the caret is in a box, set by the function that redraws the bar — which is
  already called on every move of the focus and every swap of the page — so
  the ground, the bar and the handler cannot come to disagree about which mode
  this is.
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
  next action while there is one, and the project once there is nothing open at
  all. Not two Dones told apart by whether a row is selected — the project's own
  Done is not even on the screen while it has open work, because internal/app
  refuses to complete a project that has any (design.md, "Completing a next
  action"). So the letter is not spent twice and never was: there is at most one
  Done on that page at any moment, and completing the last action is what
  changes which one.

  **At most one, because there is a state with none.** A project whose every
  open action is snoozed has no next action to finish and may not be finished
  itself, so neither Done is drawn and `d` presses nothing there — which is the
  bar telling the truth rather than a key going missing (design.md, "Stalled
  projects").
- **and that is what lets `t` be the task branch.** The three are read before
  the declared keys, so `t` asks for a `kb-pick` first and only then for a
  control on the page. Stage one of processing carries no such form — the
  match list is rows of text and a digit, with no mark to press — so the ask
  comes back empty and the branch takes the key. Nothing about the branch is
  special-cased: the ordering was already there, and a screen with a Today
  mark on it could not have taken `t` for anything else, which is the same
  thing as saying the two are never on a screen together.
- **`kb-next` is a fifth row form, and the only one that is a row's alone.** `n`
  presses it, and it is not in `pushScreenKeys` and never will be: the three
  above answer for the screen when no row is selected because an action's page
  and a project's are *about* an item, and "make this the next action" has no
  such reading — the item those pages are about is either already next or is
  not in a plan. So the key does nothing with nothing selected, and the bar does
  not offer it, which is the standing rule rather than an exception to it.

  It leaves the screen, unlike the review mark: the answer is which action is in
  the boxes at the top of the page, so the page is drawn again with the row and
  the boxes swapped. An ordinary submit, an ordinary redirect back to where it
  was pressed, and no deaf window — nothing is being destroyed, and pressing it
  twice on the same row means the same thing the second time.
- **the Reference view is the first list whose rows carry `kb-delete`**, which
  is what makes `⌫` mean "take this out of the pile" while the pile is being
  read. Nothing new was needed for it: `deleteHere` has always asked the
  selected row for its own delete form before falling back to the screen's, and
  the bar has always offered the key only where such a form exists. What the
  view adds is the first row that answers — see design.md, "Reference" for why
  a pile that only grows is the one list where pruning belongs on the row, and
  the audit entry is what makes the press recoverable.
- **`kb-review` is a fourth row form, and the one that does not leave.** `r`
  presses it, and it is the only one of the four whose answer is a row redrawn
  where it stands rather than a screen replaced — so it takes no deaf window
  (there is no motion to cover) and no screen form (no screen presses it).
  Both the key and a click on the mark go through the same flip, for the
  reason every other pair does: one path, so the letter and the pointer cannot
  come to mean different things. See implementation.md, "The weekly review
  screens".
- **`data-jump`** lets a control name its own `m` letter, claimed before any
  computed one, which is what stopped Delete moving between `l` and `e`. Only
  Recapture still uses it, and it stays for the reason above.
- **the bar's entries are the controls.** An entry that presses something is a
  real button wired to the control the letter presses, through the same
  `press()`; one that steers is not a button at all. `keys.bar_style` in the
  settings file chooses how loudly the difference is drawn, from `plain` to
  `button`, and defaults to `chip` — see implementation.md, "The key bar is
  the buttons".
- **a numbered list is one entry in the bar, not nine.** A control marked
  `data-key-quiet` still answers its key and is left out of the bar, and a run
  of them is drawn as the range they cover — `1…3 copy` — which presses
  nothing, exactly as the bookmarks' `1…9` does. The run ends at the first ordinary key, so
  the bar can never claim a range that is not one (implementation.md, "The
  match list").

- **`h` is on the answer, not on the row.** The Settings screen renders one
  form per theme and hangs the letter on the one the next press gives, so the
  bar's entry is derived from the page like every other entry and says the
  answer it lands on. Nothing in the key layer knows what a theme is
  (implementation.md, "Theme").

- **the Dashboard's keys change its form, and nothing else.** Nothing on it
  can be completed, picked or deleted, because nothing on it is an item
  (design.md, "Dashboard") — so it has no row keys and no screen keys, and the
  five it does have are all the same verb: `1`, `2` and `3` open Traffic, Queue
  and Inbox zero, and `j` and `k` go to the next and the previous. It was
  written here as "the first view with no keys of its own", and that was true
  while it was one page.
- **the digits are the forms' numbers, printed on the line they press.** That
  is the weekly review's arrangement and it is offered by the review's rule: a
  bare digit goes to a bookmark only where a screen has a filter line, and to a
  snippet only where it has a meta line, and this screen has neither. So the
  bar says `1…3 form`, a range that says how to steer, and the names are read
  where the numbers are.
- **`j`/`k` on it move by form, not by row or by section**, and they wrap,
  since three is few enough that the far end is nearer backwards. They keep
  their meaning, "the next thing", and the next thing here is a form. They
  used to move this screen by section, when it was several windows tall; it
  fits the window now, so there is nothing below to move to. The bar says
  which it is offering — `j next`, `k previous`, each an entry of its own
  because each presses a link, rather than `j k move` or `j k by section`.
- **moving by section is still built and no screen uses it.** A screen with no
  rows that marks its sections is moved through them by `j`/`k` (design.md, "A
  screen taller than the window is read from the keyboard too"). The Dashboard
  was the only one, and no longer marks any.

- **`u` is a control in the layout, declared like the ages flag.** A hidden
  button carrying `data-key="u"` and `data-global`, disabled while there is
  nothing to take back — so the bar's entry and the key are the standing
  mechanism and nothing new, and a key with nothing to do is not offered. What
  it opens is fetched at the press (implementation.md, "Undo")
- **Undone is the same declared key it was, with a different letter on it**,
  plus `data-deaf-after` on its form, which is what asks the arriving page for
  the deaf window

- **`b` no longer arms, it asks.** `leave()` hands every way out to one
  function, and that function puts the question when the screen has unsaved
  work on it — so the key, a link in the rail, `↵` on a row and a `g` chord
  all stop at the same place, and there is no route out that forgets to ask.
  The bar's entry reads `b back` in both states now: there is no armed state
  left for it to say (see "Leaving a screen").

What is left, and deliberately:

- **the project picker still takes a bare `c`** for "new project" while its
  list is shut. It owns the keyboard the way a dialog does, so nothing
  collides, but it is a second `c` in a file that argues against second
  meanings.
- **`esc` with the filter line up goes back into the line.** From the list,
  `esc` drops the cursor and puts the caret in the filter box — which is
  command mode's own key landing you in insert mode. It predates the two modes
  and is the only way back to an open line that does not close it (`f` closes
  it and takes the filters away), so it stays until something better is
  decided; the ground changing is what keeps it from being a surprise.
- **the panel chooser keeps `t` `n` `k` `z`.** A dialog is a menu of its own
  letters and the jump layer already stands down inside one, so these are not
  the screen's keys to normalize.
