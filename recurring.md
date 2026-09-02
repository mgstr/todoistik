# Recurring items - design discussion

Status: **open**. Nothing here is decided. This document exists so that the thinking is not lost, and so that [idea.md](idea.md) stays a description of what the app does rather than a list of what it might do.

## The problem
Nothing in the system repeats. Several things it already specifies are inherently recurring - the weekly review itself, and step 0 of that review (gathering from calendar, mail and messengers) is a chore that will exist forever.

At the same time, recurrence is the single biggest complexity magnet in todo applications: intervals, "every second Tuesday", what happens when something is completed late, whether the next instance counts from the due date or the completion date, catching up after three weeks away. It is easy for it to end up larger than the rest of the system, which would directly contradict the design principle of adding only what will actually be used.

## Five examples
Chosen because they behave differently, not because they are a representative list of chores.

1. **Run the weekly review** - every 7 days. Doing it on Wednesday instead of Sunday breaks nothing. No external consequence for lateness.
2. **Pay the rent, on the 1st of every month** - a real deadline with real consequences. Paying on the 3rd does not mean the next one is due on the 3rd.
3. **Water the plants every 3 days** - watering on day 5 means the next one is 3 days from then. Missed instances do not accumulate; you do not water twice to catch up.
4. **Swap summer and winter tyres, twice a year** - roughly October and April, driven by the season rather than by a fixed date or by when it was last done.
5. **Annual car inspection / insurance renewal** - driven by an expiry date, and it has to appear on the radar weeks ahead, because it takes several steps: find a garage, book a slot, go.

## What the examples reveal
These are not one feature. They are three.

**A. Interval from completion** (examples 1, 3). "Again, N after I last did it." Late completion simply shifts the next one. `snoozeUntil` already expresses this.

**B. Fixed calendar schedule** (example 2). "The 1st of the month, regardless of me." Lateness must not shift it, missed instances matter, and there is a genuine due date.

This one may need nothing at all. Step 0 of the weekly review already gathers from the calendar into the inbox, so rent-type obligations can live in a calendar with its own reminder and arrive through a path that already exists.

**C. Recurring outcome, not a recurring action** (examples 4, 5). These are not single actions - tyres means booking a garage and getting there, car inspection is three or four steps. Each instance also genuinely differs: this year the tyres might be worn and need replacing first. Re-instantiating last year's action list would be wrong.

## Candidate direction
Handle recurrence **outside the app**.

- scheduling is a separate concern from doing. A separate system submits items through the external capture API that already exists
- recurring items already have a known structure: if it has been done before, the actions and the metadata are known. So submissions would be **structured** - a ready standalone action, or a project with its actions and metadata - rather than raw text
- these are effectively templates, and the templates live in the scheduler, not in this app. The app never gains a template concept
- recurrence rules, catch-up and calendar arithmetic stay entirely out of the app

The appeal is that it costs almost nothing: the external capture API exists, Inbox Zero exists, and the complexity magnet stays outside.

## Blocking questions

**1. Auto-accept, or forced acceptance through Inbox Zero?**
The argument for routing scheduled items through the inbox is that the inbox is a forcing function - it makes you consciously accept the commitment. Auto-accepting from trusted sources removes exactly that.

Against auto-accept: a pre-structured item costs one keystroke to accept, since there is nothing to type, so the friction saved is near zero. What is lost is real - an auto-accepted recurring project appears on the project list without the commitment having been made for *this* cycle, and the stalled project check then starts shouting about something never agreed to. That is how loud signals get trained into noise. A misbehaving scheduler flooding an inbox is also visible and cheap to undo; flooding the commitment list is not.

**2. What happens to repeated submissions of the same recurring item while an earlier one is still unaccepted?**
Three weeks away should not produce twenty one identical inbox items. A candidate answer: a scheduled submission carries an identity, and a new instance supersedes a still-unaccepted earlier one rather than piling up. Skipped cycles become the scheduler's problem, not yours.

**3. How are structured submissions validated on arrival?**
If an external system can inject a fully formed project, the title-is-a-reference rule, the required DOD and the at-least-one-action rule have to be enforced at the boundary too. Otherwise the external path becomes a hole that lets malformed projects in, bypassing the discipline Inbox Zero exists to impose. A candidate answer: anything failing validation lands as plain raw text instead, and gets clarified by hand.

**4. Does the calendar really cover case B?**
Depends on whether rent-type obligations are already kept in a calendar in practice, or whether they would be expected to live in this app.

## Known cost of this direction
The app would keep no link between instances. It could not answer "when did I last change the tyres" or "when is this due again" beyond what happens to be in the audit log.
