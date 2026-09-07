# todoistik
Describes todo application, intended for my purposes only. This app is not intended as a generic todo app.

The document is organised in four parts:
- **Principles** - what I want out of this, and the rules that decide what gets built
- **Items** - the objects I work with, and their fields
- **Views** - the ways items are shown to me. Every list the app shows is one of these
- **Processes** - the rituals that guide me through items

## Principles

### Goals
- **Nothing is held in my head.** Every open loop lives in the app, so that remembering is not a job I have to do.
- **Nothing dies silently.** The failure mode worth designing against is not a forgotten task, it is a commitment that stays on screen looking alive while nothing about it moves.
- **The views are trustworthy.** A view that is not trusted to be complete is a view that stops being used. The weekly review is what keeps them trustworthy, and everything else in this document is bookkeeping in service of it.
- **Capture costs nothing.** Deciding what something means is a separate deliberate act, performed later. Friction at capture time is what makes a system get abandoned.
- **The protocol is followed, not enforced.** GTD is a discipline kept by the person. The app's job is to make the state of things impossible to misread and the right move cheap - never to be the thing that compels it. It says loudly that the inbox is full and never blocks the way past it; it asks for the weekly review and withholds nothing until one has been done; it lets an item be processed out of turn, and lets the processing screen be left without an answer given. A rule the software enforces is a rule you learn to work around, and working around your own system is how you stop trusting it, which costs more than the rule was worth.

  This is about **when and whether you practise**, and not about what gets recorded. The app does refuse things, and the line is that it declines to record what would make a view lie: a project with no definition of done, completing a project whose actions are still open, removing a tag that items still carry. Those refusals serve "The views are trustworthy" and are a different question from this one. Where the two could be confused, the rule that decides is whether the thing being refused is a step of the practice or a false statement about the world.

  There is one deliberate exception, named here as an exception rather than smuggled in as an integrity rule: **an action's title has to start with a verb** (see "Inbox Zero"). That is the app pushing rather than helping, and it is allowed because of *where* the push lands. Nothing compels you to make the item an action, or to decide about it now, or to decide about it in any particular order - only to say what doing it looks like, once you have already chosen to make it one. The push is on the writing, never on the deciding.

  It is also the shape of push worth having, which is the general test for adding another. Complying is cheaper than avoiding it, so there is nothing to work around. It teaches something that transfers: after a few weeks the verb arrives without being asked and the check stops firing, which is a rule that removes itself. And getting it wrong is expensive later in a way that is invisible now - a "Next actions" view full of "milk" and "the bank" is a list you cannot act from, which is "The views are trustworthy" again. A push that stays costly after the habit has formed, or that teaches nothing, does not qualify.

### Design principles
- add only functionality that I will use, don't add anything for future development
- app should be fast, it usage should not be obstacle
- the keyboard only support should be provided
- **the app is used at a desk.** Every screen assumes a window wide enough to hold the navigation beside the list rather than above it, and keyboard only assumes a keyboard. The phone is a capture device and nothing else - the share sheet posts to the capture API (see "External capture"), which is the one way in that was ever meant to be used away from the desk. Reading, processing and reviewing are not designed for a small screen, and designing for one would mean a second layout of every screen that has to be kept true against the first, for a way of working I do not do
- the app should be AI friendly, so AI could get info from it for analysis and control the info send to it (using inbox) - see "The read API" and "External capture"
- the design of the app should allow to follow principles described in David Allen's book "GTD - Getting Things Done"

## Items
The objects the app works with:
- **inbox item**: a raw, unprocessed capture, that has not been decided about yet
- **someday/maybe item**: a raw capture that is worth revisiting some time, but not now
- **action**: a single non-breakable task, that can be done and have visible output effect
- **project**: when end result can't be achieved in result of single action it is called a project, it contains a list of actions, and has a "definition of done"
- **schedule**: a piece of text and a rule for when to put it in the inbox
- **audit entry**: the record that something happened

An item is stored as itself. Nothing is stored "inside" a screen - every screen is a query over these items, see "Views".

### Inbox item
Raw, unprocessed capture. Deliberately near-schemaless - the point is zero friction at capture time.
Capture must never require a decision: deciding what an item actually means is a separate deliberate act (see "Inbox Zero"), performed later.
An inbox item is never something to be done, it is something to be decided about.
Inbox item has following fields:
- Text: (required) free-form, whatever was captured
- Creation date: (required)

An inbox item can not be snoozed - see "Time fields".

#### External capture
Adding items to the inbox must be possible from outside the app.
The app exposes a simple consuming API for this - a single endpoint accepting a text payload - so that captures can arrive from scripts, CLI, a mobile share sheet, email or any other tool without opening the app.

#### Duplicate captures
A capture whose text is exactly identical to an item **already sitting in the inbox** is dropped. This holds for every way in: typed by hand, the capture API, a schedule.

- the comparison is against the open inbox only, never against history. Checking everything ever captured would mean a repeating chore arrived once and never again, because the first one was processed weeks ago
- matching is exact and not fuzzy, for the reason the name filter is exact: it should always be obvious why something matched
- nothing is lost when a duplicate is dropped, since the loop it describes is already in the inbox waiting to be decided about. It is not audited, for the same reason
- the capture API says so in its response rather than simply succeeding, so that a script cannot mistake "silently vanished" for "accepted" - see "External capture"

This is what makes capture **idempotent**, which is worth having on its own: a script retrying after a timeout, a share sheet tapped twice, and a schedule replaying the occurrences missed during three weeks away all stop being able to flood the inbox. Emptying the inbox is the one rule with no exceptions, so anything able to pile up in it without limit is a threat to that rule.

Collapsing is lossy wherever instances genuinely count - two months away would otherwise mean "Pay the rent" firing twice and being seen once. That is what a schedule's suffix is for: it makes each occurrence produce a different string, so nothing collapses that should not. The default is to collapse, and saying otherwise is one field - see "Schedule".

### Schedule
A piece of text and a rule for when to put it in the inbox. It exists so that the things which have to come back - a chore that repeats, or an obligation that has to be looked at weeks before it falls due - are not held in your head in the meantime.

A schedule is not a commitment and never becomes one by itself. What it produces is a **capture**: raw text arriving in the inbox, decided about by hand in Inbox Zero like anything else. Nothing reaches "Projects", "Tasks" or "Next actions" without having been accepted there. This is also what covers recurring actions and projects.

Fields:
- Text: (required) free-form, what will land in the inbox. It is a capture, so it stays raw - not a title, not an action, not a project
- When: (required) either a single **date**, or a **cron expression** at day granularity - day of month, month, day of week, and no times. Nothing in this app has an hour, so neither does this. The three fields are the calendar fields of standard cron, with standard syntax and semantics - `*`, lists, ranges, steps, and the standard rule that when both day-of-month and day-of-week are restricted, either one matching fires. Standard and not invented, so that the behaviour of any expression can be looked up rather than guessed. The expression is validated when the schedule is saved, and the "Scheduler" shows it in readable form
- Suffix: (optional, empty by default) appended to the text when the capture is made. `YYYY`, `MM` and `DD` are replaced with the date of the occurrence being fired; everything else is literal, including any leading space. An empty suffix makes every occurrence produce the same string, which is what collapses a repeated chore to a single inbox item; ` YYYY-MM` on the rent makes each month produce its own
- Creation date: (required)
- lastFiredAt: (optional) when it last put something in the inbox, empty until it first does. It is what shows at review time that a schedule is actually working
- lastReviewedAt: (required) see "Time fields"

Rules:
- **it fires lazily**, on the first use of the app on a day whose occurrence has passed. This is the rule `#today` clearing already uses, for the same reason: a scheduler that works only while a process happens to be running is one that cannot be trusted, and a firing that did not happen is invisible
- **a single date fires once, and the schedule then deletes itself.** A one-shot left in the list forever would turn the "Scheduler" into a graveyard of things that already happened. The deletion is audited like any other
- **a cron schedule persists** and keeps firing
- **every missed occurrence fires**, oldest first. After an absence a schedule does not have to be careful about how many went by: without a suffix the captures are identical and collapse in the inbox to one item, and with one they stay distinct, because a suffix is how you said the instances count - see "Duplicate captures"
- **an occurrence only counts if the rule was in force when it fell.** Occurrences are counted from the creation date, so a schedule created today does not back-fire for dates before it existed, and editing "When" restarts the count from the edit: past occurrences of a rule that was not yet in place were never missed
- it is edited and deleted like anything else - see "Editing items"

The three parts compose deliberately, and each stays dumb on its own. The schedule fires per occurrence and knows nothing else. The inbox drops a capture identical to one already waiting. The suffix is the one place where you declare that instances are distinct, and it is visible in the text that arrives, so two rent items say which month each is for. Nothing anywhere tracks instances.

The cost lands where you put it: a suffix on a daily schedule is how you ask to be told about every single day you were away. Use one only where the instances genuinely count.

Instances are not linked to each other. A schedule knows when it last fired and nothing about what became of what it produced, so "when did I last change the tyres" is answered by searching the "Archive" for the action, not by asking the schedule. That is the right place for it: what you did is a completed commitment, and the schedule only ever made the reminder.

A schedule can not express "again three days after I last did it". Cron describes the calendar and not your last completion, and that case is already covered without it: complete the action, and put a `snoozeUntil` on the next one.

### Someday/maybe item
A raw idea that is worth looking at some time, but that you are not ready to work on now.

A someday/maybe item is not a project and not an action - it is the same raw, unclarified capture as an inbox item. Clarifying it would mean defining an outcome and a next action for something you have deliberately decided not to commit to, which is wasted work and is exactly the friction that makes a someday/maybe go unused. It therefore stays raw until you decide to move on it.

Fields:
- Text: (required) free-form, the idea as captured, editable
- Creation date: (required) it shows the age of the idea
- lastReviewedAt, snoozeUntil - see "Time fields"

Rules:
- items become someday/maybe items from the inbox, as one of the outcomes of Inbox Zero
- when you decide to move on an item, it is processed exactly the same way as an inbox item (see Inbox Zero)
- `snoozeUntil` excludes the item from the weekly review requirement until that date, so that a long someday/maybe view stays reviewable

### Action
An action is a single (non-breakable into smaller parts) task, that should be done in order to move to the desired goal.
The action should have visible effect. So "thinking about design" is not an action. Use "Write draft a MD with design" instead.
Action has following fields:
- Title: ideally it should start with a verb and be fully self-descriptive, avoiding letting something to be in context. So when looking at the action title you don't have to think before you start doing it.
- Context: (optional) the physical prerequisite for doing the action, at most one - see "Contexts"
- Duration: (optional) how big the action is, as one of three sizes: `short`, `medium`, `long`. Sizes and not minutes, and deliberately with no unit named anywhere: a bucket labelled with a number is still asking how long something takes, which is a question with no honest answer and one you have to stop and work out - where "is this a small thing or a big thing" is something you already know when you look at it. Three and not more for the same reason: the moment two buckets sit next to each other on the same scale, choosing between them is a decision, and this field is only worth having if it costs nothing to fill in. `long` also carries the original meaning: do not start this unless there is enough time to finish it in one sitting, like reading a long article.
- Needs focus: (optional) marks an action that can not be done while tired. Deliberately a single flag rather than a low / normal / high scale - having to grade the energy of every action puts pressure on capture, which is exactly the friction worth avoiding.
- Description: (optional) any extra materials needed to be referenced (like URL, link to email, reference to PDF etc) that could be useful during the action.
- Tags: (optional) zero, one or several labels - see "Tags"
- Assigned to: (optional) free text. If not set, it is assumed that you are the one who should do it. If set, the action is waiting on somebody or something else, and appears in the "Waiting for" view.

#### Standalone actions
An action does not have to belong to a project, and most do not. A single action that fully achieves its outcome stands on its own and is never wrapped in a project just to give it a parent - that bureaucracy is what makes a system get abandoned.

An action that belongs to no project is a **standalone action**. Standalone actions are shown together in the "Tasks" view, which is simply where every parentless action is found. Tasks is a view and not a container: nothing is moved into it, an action is in it exactly as long as it has no project.

A standalone action is **always** a next action: `becameNextActionAt` is stamped whenever an action is created without a project, or loses the one it had (see "Reshaping items"). The parked state exists only inside a project, where an action written down in advance is part of a plan that the project keeps visible and the stalled project check watches over. Outside a project there is neither plan nor check, so "parked" would mean nothing beyond "I am not going to look at this" - and that is what `snoozeUntil` is for, or someday/maybe if it is not a commitment yet. There is no third state. The stalled project check does not apply to standalone actions.

### Project
Project is a desired result, that requires more than one step to complete.
Project has following fields:
- Title: name that helps to reference the result
- DOD (definition of done): required when the project is created, since it helps to define what is the expected outcome of the project, and used during review and decision what is the next action. An existing project can be left without one, which puts it in an error state - see "Editing items"
- Tags: (optional) zero, one or several labels - see "Tags"
- Actions: a list of actions required to complete a project. In most cases it is enough to have only one next action to move the project forward. But in some cases listing more steps in advance during the planning phase is helpful.

A project has no due date. A deadline belongs to an action - see "Deliberate omissions".

A next action is not a property of the project. It is a property of the action - see `becameNextActionAt`. A project can therefore have several next actions at the same time, which is what a parallel project looks like (booking the flight, renewing the passport and asking for time off are all available at once), while a sequential project simply happens to have one.

Projects and standalone actions together are every commitment in the app. An action either sits under a project or is standalone; there is no third place, and nothing is loose.

#### Stalled projects
An active project is stalled when it has no next action.

This is the single most common way things silently die: the project stays visible, looks alive, and nothing ever moves. Catching it is the highest value check in the app, and it costs nothing - it is derived, never stored.

- a project is exempt while it is snoozed, and once it is completed
- a project whose only next action is a waiting for action is **not** stalled
- the check applies to projects only. Standalone actions are not covered by it, and neither is the Tasks view - see "Tasks"

Stalled projects stay visible in the normal views, clearly marked as stalled (red, or similarly loud). They are not hidden away in a dedicated screen, and they are not something only the weekly review surfaces.

The app never prevents a project from being stalled. Forcing a next action to be invented at a moment when there is no time or energy for it produces a bad action, and a bad action is worse than a stalled project that is shouting about itself and will be dealt with at the weekly review or sooner.

### Time fields
Time related fields, and the items each one applies to:
- creation date: (required, all items) when the item was created, used to calculate its age
- due date: (optional, **actions only**) a real, externally imposed deadline, after which there are consequences outside your control. It is not a way to hide an item until a date and not a self-imposed target - invented deadlines are what makes the real ones stop working. Deferring something to a date is what `snoozeUntil` is for. It is what the "Calendar" view is built on, and it is shown wherever the item appears
- lastReviewedAt: (required, projects, actions, someday/maybe items and schedules) when the item was last reviewed. Stamped with the creation date when the item is created - creating an item is always a conscious act, so creation counts as its first review, and the field is never empty. It drives the weekly review: it shows what has already been walked through and what is still outstanding, which is what makes an interrupted review resumable
- becameNextActionAt: (optional, actions only) when the action became a next action. An empty field means the action is not a next action - it is parked, written down in advance during planning. Only an action inside a project can be parked; a standalone action always has this field set - see "Standalone actions". A **real** next action is one where `becameNextActionAt` is set and `completedAt` is still empty. The field doubles as the age of the next action, which is what shows an action that has been next for a long time without moving, and for actions with "assigned to" set it is also the delegation date. Because it is also the delegation date, changing "assigned to" restamps it: delegating an action starts a new clock - you stopped waiting on yourself and started waiting on them - and taking an action back restamps it again for the same reason in reverse. Without the restamp, an action that had been next for three weeks and was then delegated would look three weeks stale in the "Waiting for" view on day one.
- snoozeUntil: (optional, projects, actions and someday/maybe items) marks the item as not yet ready to be worked on, until that date passes
- completedAt: (optional, projects and actions) when the item was completed. Being set is what makes the item done - there is no separate status flag

Every one of these is a date and never a time of day, and they are all read against a single clock: the timezone the app is configured with. "Today" therefore means the same day everywhere it is asked - the `#today` clearing, lazy schedule firing, "due today" and overdue all share the one boundary, and a capture sent from a phone in another timezone lands on the app's day, not the phone's. Two clocks would mean two opinions about whether something is overdue, which is the kind of disagreement that makes a view stop being trusted.

`snoozeUntil` is a universal field and means the same thing everywhere it appears - on projects, actions and someday/maybe items: do not bother me about this until that date. It is how an already clarified commitment is shelved for a while without losing its DOD, its actions and the material collected in it.

`snoozeUntil` is also what covers deferral - "there is no point looking at this before Tuesday" - so there is no separate defer date.

A snoozed item is **not hidden**. It stays visible and is shown differently, to indicate that it is not yet ready to be worked on. Hiding it would be confusing: a project whose only action had become invisible would look stalled while the app insists it is not.

What a snooze actually does:
- it excludes the item from the weekly review requirement until the date passes
- a snoozed **project** is exempt from the stalled project check
- a snoozed **action** still counts as a next action of its project, so deferring a single action does not make the whole project look stalled. The stalled project check knows about snoozed actions. This is the same exemption a waiting for action gets, and for the same reason

The single exception is the inbox: an inbox item has no `snoozeUntil`. Snoozing an inbox item is the same thing as making it a someday/maybe item. Emptying the inbox is a non-negotiable rule and must not be avoidable by snoozing.

### Contexts
A context is a physical prerequisite for doing an action: something that has to be true before the action is possible at all. If the action could be done without it, it is not a context.

Contexts apply to **actions only**. A project is not something you do, so it has no context.

An action has **at most one** context. Notation is `@name`: `@home`, `@garage`, `@online` (an internet connection is needed, on any device), `@computer` (a real computer is needed, a phone will not do).

Context names come from a remembered list - never typed fresh, otherwise `@home` and `@Home` drift into two contexts. Writing `@home` on an action's meta line sets the context only if `home` is on that list; if it is not, the line is refused rather than saved with the name quietly ignored (see "Writing an action"). Adding a new name is therefore a deliberate act, and it should be offered where it is wanted rather than as a trip to another screen: when what was written matches nothing, the app offers to create it there, behind an explicit confirm - deliberate enough to stop drift, cheap enough not to fight capture. The list is editable, so that a context no longer used can be removed; one still carried by actions can not be, since removing it would be editing those actions behind their back.

#### Parameters
A context may carry a parameter: `@person(Andres)`, `@grocery(Selver)`. This keeps the context namespace small and scannable, which is the only reason contexts are useful at all - putting every person and every shop chain at the top level would destroy that.

- the parameterised form is **narrower** than the bare one. Standing in Selver satisfies `@grocery(Selver)` and bare `@grocery`, but not `@grocery(Prisma)`
- the bare form is not always meaningful. Bare `@grocery` is useful ("buy milk, any shop"), bare `@person` is not. Some context types will in practice always carry a parameter, and that is fine
- parameter values are picked from a remembered list per context type, never typed fresh, otherwise `@person(Andres)`, `@person(andres)` and `@person(Andres P.)` become three different contexts
- that set of values has to be editable, so that values that are no longer used can be removed

#### Filtering
The "Next actions" view filters by one or several contexts, combined with **OR**: at home, with a computer and an internet connection means `@home OR @computer OR @online`.

OR is the correct combination precisely because an action carries a single context - the question being asked is "is this action's context among the ones I currently satisfy". The cost of the single context is that an action needing two prerequisites at once has to name the scarcer one; this is accepted.

### Tags
A tag is a label used to filter and categorise. Unlike a context it is not a precondition - it says nothing about whether an item can be done, only about what it is about.

Notation is `#name`: `#car`, `#finance`, `#hobby`, `#programming`.

- tags apply to **both projects and actions**
- an item can have zero, one or several tags
- in practice these are not arbitrary keywords but the standing areas of responsibility that work belongs to. That makes them the thing that answers the review question "which part of my life am I starving?". The single exception is `#today`
- tags follow the same rule context names do: from a remembered list, never typed fresh, added deliberately, and removable from the list only while no item carries them - see "Contexts". Writing `#car` on a meta line makes it a tag only if `car` is on the list. Areas of responsibility are few and stable, so a list that is deliberate to grow costs nothing here
- `#today` is built in, and so are the tags that are not tags at all but fields wearing a tag's notation: `#short`, `#medium`, `#long`, `#focus` and `#parked`, along with the `@waitingFor` context. None of them can be removed, because removing one would not take away a label - it would take away a field
- the lists are shown with **how many items carry each name**, built-in ones included. For a built-in that count is not bookkeeping: it is the only place the app says how much of the work is short, how much needs focus, how much is parked - which is a review question, asked where the vocabulary is kept

#### #today
`#today` marks an action as picked for the day - see "Today". It is an ordinary tag in every respect but one: it expires.

- it is applied and removed by hand, like any other tag, from anywhere a tag can be edited
- it is cleared from every item on the first use of the app on a new day, by the **local** day. A read through the read API counts as use, whichever comes first: the API returns what the screen would show, so a caller that saw yesterday's picks would be seeing a view the app itself would never render - the same boundary "due today" uses. The clearing is deliberately not a scheduled sweep: a sweep only runs if something is up to run it, and a "Today" still showing yesterday's picks because a machine was asleep is the view being quietly wrong in the direction that matters most. Clearing on arrival cannot be observed stale, because nothing looks at the view before you do
- it is never audited, neither when applied nor when it expires. The audit log is what makes commitments recoverable, and a pick is not a commitment: there is nothing in it to recover, and a few picks every morning would bury the entries that are worth finding
- on a **project** it is inert. It is not special cased on assignment - a tag that behaves differently depending on what it is attached to is worse than one that does nothing - and a project is not something you do, so there is nothing for it to narrow
- it is the one tag that is not an area of responsibility, and the only one that says *when* rather than *what about*

### Completion
An item is resolved explicitly, and only in one of two ways: it is completed, or it is deleted. There are no shortcuts and nothing is resolved implicitly.

- a completed action leaves the "Next actions" view and stops counting as a next action for its project, which may leave the project stalled
- a project can not be completed **or deleted** while it still has open actions. Every one of them is resolved explicitly first: completed, deleted, or detached into a standalone action (see "Reshaping items")
- a project with no DOD can not be completed until it has one. Completing is the moment the DOD is confirmed to be met, and there is nothing to confirm - see "Error state"
- completing a project is therefore always a deliberate act, and the moment the DOD is confirmed to be met. A project is never completed automatically just because it ran out of actions
- a completed item leaves the active views and is found in the "Archive". That view and the audit log are not the same record: the Archive holds finished **commitments**, the audit log holds **events** - every creation, edit, completion and deletion, including the ones that never became items at all

### Error state
An item that is not in a state you would have accepted is in an error state: its fields contradict each other, or something it can not do without is missing. It stays highly visible until it is fixed, the same way a stalled project does, and is dealt with at the weekly review or whenever there is time.

The known cases:
- `snoozeUntil` set past the due date. The item would stay marked as not yet ready to be worked on until after the moment it was supposed to be finished, which is never what was meant.
- a project with no DOD. Reached by clearing the DOD of an existing project - the save is never blocked, for the same reason a project is never prevented from being stalled: being unable to say what done means is information worth showing, not a reason to refuse the edit. Such a project can not be completed until it has one again - see "Completion".
- "assigned to" set while `becameNextActionAt` is empty. A waiting for action that is not a next action would appear in no view and silently disappear. The normal flows cannot produce this - setting "assigned to" restamps `becameNextActionAt` - so this state means data got in past the normal flows, and it is flagged rather than reinterpreted.

Such a state is not silently resolved by letting one field win over the other - that would hide the mistake instead of the item. The app makes an effort to avoid the situation when the dates are entered, and if it still occurs, the item is marked as being in error rather than quietly reinterpreted.

### Audit entry
Every action performed in the app is audited. An audit entry contains at minimum:
- timestamp
- what happened (item created, edited, completed, trashed, reshaped, ...)
- the item it refers to

This keeps destructive operations (trashing an inbox item) and instant ones (completing an item under the two minute rule) reviewable and recoverable, without keeping those items among the active ones.

The one exception is `#today`, which is never audited - see "#today".

### Writing an action
An action is written in four fields and no more: its **title**, its **project**, a **meta** line and a **description**. Everything it carries beyond the first two is written on the meta line, in notation:

| written | means |
|---|---|
| `@name` | its context |
| `@name(parameter)` | a context that needs naming, e.g. `@person(Marju)` |
| `@waitingFor(who)` | "assigned to" - it is not yours to move, and it appears in "Waiting for" |
| `#short` `#medium` `#long` | how big it is |
| `#focus` | needs undivided attention |
| `#today` | picked for today |
| `#parked` | inside a project: written down in advance, not yet a next action |
| `#name` | a tag |
| `due:2026-09-20` | a real deadline |
| `snooze:2026-09-20` | out of sight until then |

The point is that the form asks for nothing that has to be decided. A row of controls asks every question of every action, and most of them have no answer worth giving - a line asks one question, and you write only what is true. It is also the same gesture capture already is, so the two ends of the process are typed the same way.

- **the meta line and the description are two fields because they are read for two different reasons.** The description is read to remember what an action is about; the meta line is read to see what the app thinks it is. They shared one box until it became clear that neither could be looked at without the other in the way: the notation had to be found again at the bottom of the prose on every edit, and the prose could not be rewritten without editing notation by accident.
- **a name is notation only if it is already known.** `@name` and `#name` are read as metadata when the name is on the remembered list (see "Contexts" and "Tags"). This is the same rule that already said names are never typed fresh - without it a written field is the widest possible door for `@home` and `@Home` to walk through separately.
- **the meta line refuses what it cannot read.** Whatever is left on it once the notation has been taken out is reported and nothing is saved, whether that is an unknown name, a typo or a sentence. While the two shared a box this question did not arise - anything unknown stayed prose, which is what kept `marju@gmail.com` from becoming a context and `invoice #12345` from becoming a tag. On a line that holds nothing but names there is no prose left for it to stay as, so the choice is between saying so and swallowing it silently, and being told that `#hobbies` is not `#hobby` is worth more than a tag that quietly did not apply.
- **the description is prose, and nothing is read out of it.** `@home` written there is a word like any other. Nothing an action carries can be changed by editing it, which is what makes it safe to write in freely - and it is why it, not the meta line, is where a sentence that happens to mention a context belongs.
- **the fields are still fields.** What is written on the meta line is read into them when the action is saved, and written back out of them when it is opened. Every view, filter and sort works on the fields exactly as before - nothing queries text. This is also why the app can still change them on its own: picking for today, a detach stamping a parked action, a delegation restamping the clock all move a field, and the box simply shows the new truth next time it is opened.
- **a contradiction is refused, never guessed at.** Two contexts, two sizes, `@waitingFor` with nobody named, `#parked` on a standalone action - each is reported and nothing is saved. Guessing which one was meant would be the app deciding something the person is in the middle of deciding.
- **the notation is written back in a fixed order.** Opening and saving an action twice cannot shuffle or lose anything, which is what makes the line safe to keep editing.

### Writing a project
A project is written in three fields: its **title**, its **definition of done**, and a **meta** line - plus the actions under it, which are written as actions and not as part of the project (see below). The meta line is the same notation read the same way, narrowed to what a project has:

| written | means |
|---|---|
| `#name` | a tag |
| `snooze:2026-09-20` | out of sight until then |

- **a project's line is short because a project carries little.** It has no context, no size, no deadline and nobody it is waiting on: those describe doing something, and a project is not something you do - it is the outcome that a list of actions is aimed at. Writing one of them here is **refused by name** rather than dropped, because a size written on a project is a mistake about where the thing belongs, and a silent drop would leave that mistake believed.
- **it is the same line in both places it is written.** The project screen of Inbox Zero and a project's own page use it identically, so a project's tags are not written one way while it is being made and another way afterwards.
- **the actions are written as actions.** Adding one opens the same form an action is written in anywhere else, with the project already answered, and each carries its own meta line - which is how a delegated action is delegated here, with `@waitingFor(who)`, exactly as it would be anywhere else.
- **every action written while the project is being made becomes a next action of it**, so `#parked` is refused there: parking means "written down in advance, not yet next", and a project whose actions were all parked would be created already stalled, which is a contradiction (see "Inbox Zero"). Once the project exists, parking is one keystroke away.


## Views
Every list the app shows is a view: a query over the items. No **item** is ever stored in a view, and a view can not be created, renamed or deleted - which is what makes the ones below permanent fixtures, and what makes each of them free.

Opening a single item to work on it is not a view, and not an exception to this either: an item shown in full is the item, holding exactly what it held in the list, and nothing is kept there - see "Editing items". The guided processes are the same, showing one item at a time - see "Processes".

A due date is shown in every view the item carrying it appears in, and an overdue one is marked loudly, the same way a stalled project is. A deadline is the one thing that can not wait for the right screen to be opened, which is why it is not left to the "Calendar" alone - that view is where deadlines are ordered and asked about, not where they are learned of.

Filters are the one piece of state a view remembers, and they are not items: they decide which items a query returns, and never what exists. Nothing is created, moved or lost by filtering, and turning every filter off gives the complete list back. A filtered view says so loudly - which filters are on and how many items they are hiding - with the reset next to it, because a view quietly showing part of itself is exactly how a view stops being trusted.

**Ages are hidden until they are asked for.** Every list can say how old the things on it are - how long an action has been next, how long an idea has sat, when an entry was written - and that answer decides something two or three times a week and is noise on every other read. So the app carries one flag for it, and one for the whole app rather than one per view: ages off, which is how it starts, or ages on. It is not a filter and must not be read as one. Filtering changes which items the view returns and is therefore something the view has to confess to; this leaves a field off rows that are all still there, and hides nothing that could be acted on. Where the flag stands is written in the corner of the key bar, because a screen that can be either way has to say which way it is, and it is remembered the way a filter set is - the answer to "show me the dates" should not have to be given again after every jump between views.

### Filtering by name
Every view that can grow long carries the same name filter, and it behaves identically in all of them: **Someday/Maybe**, **Projects**, **Tasks**, **Next actions**, **Waiting for**, the **Calendar** and the **Archive**.

Matching is case insensitive. Several words may be given and **all** of them have to be present, in any order and anywhere in the name - `call bank` finds "Call the bank about the mortgage". Each word matches as a substring and not as a whole word, so `mortg` still finds it. Substrings and not fuzzy matching, so that it is always obvious why something matched. Clearing the box is how it resets.

What counts as the name is whatever names the item on that screen: the title of an action or a project, and for a someday/maybe item its text, since that is all it has. For a **project**, the titles of the actions under it count as part of its name as well - a project is remembered by a step in it at least as often as by its outcome, and hiding a project whose action matched would be hiding the answer.

The **Inbox** deliberately has no name filter. It is worked through one item at a time, oldest first, until it is empty, and a filter there would only be a way to look away from something. Processing a single item ahead of the queue is a different thing and is allowed - it takes nothing out of sight - see "Inbox Zero". Neither does **Today**, for a related reason - see "Today".

### Filtering by tag
The **tag cloud** is the other shared filter: every tag in use, each one toggled in or out of the filter. It is carried by every view that holds a commitment - **Projects**, **Tasks**, **Next actions**, **Waiting for**, the **Calendar** and the **Archive** - and behaves identically in all of them. It is what answers the review question "which part of my life am I starving?", which is why it reaches all of them and not only the working view.

- selected tags combine with **OR**: `#car` and `#finance` selected means everything about either
- an item with **no** tags is excluded as soon as any tag is selected. The asymmetry with the context filter is deliberate: the context filter asks "can I do this here", which "nothing required" always answers yes to, while the tag filter asks "is this about #car", which "about nothing in particular" answers no to
- clearing the selection is how it resets, and means all tags again, never none
- it matches the item's **own** tags. In "Projects" this deliberately differs from the name filter: a project is matched by the title of an action under it, but never by that action's tags. The name filter is a recall aid - a project is remembered by a step in it - while a tag says what the commitment itself belongs to, and a project does not belong to an area because one action in it happens to

The **Inbox** and **Someday/Maybe** do not carry it, for the same reason they carry so little else: their items are raw, unclarified captures, with no tags to filter by. **Today** carries no filters at all - see "Today".

The views:

### Inbox
The inbox items that have not been decided about yet, oldest first.

This is the only view with a rule attached to being non-empty: it must be emptied, see "Inbox Zero".

### Someday/Maybe
The someday/maybe items - raw ideas worth revisiting some time, but not now.

- it is reviewed during the weekly review, skipping items that are still snoozed
- the age shown is the age of the idea, from its creation date
- it carries the name filter, matching the text of the item - see "Filtering by name"

### Projects
The active projects, with stalled ones loudly marked and snoozed ones shown differently to mark them as not yet ready. Actions inside a project are shown with their project. A project leaves this view the moment its `completedAt` is set, and is found in the "Archive" from then on.

It carries the name filter, which matches a project by its title or by the title of any action under it - see "Filtering by name" - and the tag cloud, which matches the project's own tags - see "Filtering by tag".

### Tasks
The standalone actions: `completedAt` is empty and no project is set. Together with "Projects" this covers every commitment in the app.

Tasks is deliberately unremarkable, and each of its properties falls out of it being a view rather than a container:

- **it has no DOD.** It is not an outcome. It is not one commitment, it is the pile of small ones, and there is nothing to define done for.
- **it never shouts.** The stalled project check does not apply here. An empty Tasks view means there is nothing outstanding outside the projects, which is a good state and not a problem to fix.
- **it can not be deleted, and it does not have to be created.** It is a query, so it is simply always there - the same way the Inbox is.

Snoozed standalone actions appear here, shown differently to mark them as not yet ready. Standalone waiting for actions appear here too: Tasks answers "where does this action live", not "is it mine to act on".

It carries the name filter, matching the action title - see "Filtering by name" - and the tag cloud - see "Filtering by tag".

Every standalone action is a next action (see "Standalone actions"), so all of them are already covered by the "Next actions" view and by step 4 of the weekly review. Tasks needs no review step of its own.

### Next actions
The main working view, and the one the app is used from day to day: the actions that are on you to act on. `becameNextActionAt` is set, `completedAt` is empty and "assigned to" is empty. Actions inside a project and standalone ones appear side by side - what matters here is that they are next, not where they live.

Note the distinction in naming. A waiting for action is still a next action of its project - that is what keeps a delegated project from counting as stalled - but it does not appear in this view, because this view is only the actions that are yours to act on.

Snoozed actions appear here as well, shown differently to mark them as not yet ready. They still count as a next action of their project for the stalled project check.

There is no separate "what can I do right now" screen. It was this same query with a few filters applied, and a second view that can quietly disagree with the first about what is next is exactly the kind of thing that stops being trusted. Asking "what can I do right now" is narrowing this view, not going somewhere else.

"Today" is not that second screen. It does not re-ask this view's question with the filters set differently: it shows what has run out of time, and what you decided this morning to aim at. That decision is recorded on the item and is derivable from nothing, so "Today" is a view over a field, the way every other view is. What was rejected here is a view over filter state.

#### Filters
The filters are what make one view enough. All of them are optional and combine with **AND** - each one narrows what the ones before it left. Every filter is reachable and resettable from the keyboard, since this is the screen the app is used from.

- **contexts** - the context cloud: every context in use, each one toggled in or out of the filter. Selected contexts combine with **OR** (see "Contexts"). An action with **no** context is always shown, whatever is selected: it has no prerequisite, so there is no moment at which it is not doable, and a filter about prerequisites has nothing to exclude it by.
- **tags** - the shared tag cloud, selected tags combining with **OR** - see "Filtering by tag"
- **name** - the shared name filter, matching the action title - see "Filtering by name"
- **duration** - one or several buckets, combined with OR: what fits in the time available.
- **needs focus** - three states: **all**, **exclude** (drop the actions that can not be done while tired) and **only** (keep nothing else). Default is all. Exclude is the tired question, and only is its opposite - an hour of real attention is worth spending on the actions that need one, and nothing is more wasteful than spending it on things that could have been done half asleep.

Resetting is a first class operation, because a filter that is awkward to remove is a filter that quietly stays on:
- each filter resets on its own - clearing the context selection means all contexts again, never none
- one control resets every filter at once, back to the complete list

The filter set persists: it is remembered when you leave the view and is still applied when you come back, which is what makes working in one context for a whole afternoon cheap. That is also why the view has to be loud about being filtered (see "Views"). The filter set is momentary state: it lives on no item, and it is never named or saved (see "Deliberate omissions").

#### Order
The results are sorted by one of:

- **title** - alphabetical, ascending or descending
- **age** - `becameNextActionAt`, how long the action has been next. Not the creation date: what is worth seeing is how long something has been available to be done and has not been done. Reversible as well

Default is age, oldest first. An action that has been next for weeks without moving is the thing this view should push under your nose, and it is the same signal step 4 of the weekly review goes looking for.

### Today
The narrowing used to get through a day: everything that has run out of time, and the actions picked out this morning. It holds two groups, shown separately.

- **out of time** - every action due today or already overdue, ordered by due date with the overdue first. This is the "Calendar" today-and-earlier slice unchanged, down to the parked actions and the waiting for ones. Anything narrower would let something be out of time in one view and not in the other
- **picked** - the actions carrying `#today` (see "#today"), ordered by age the way "Next actions" is

"Out of time" offers no pick mark on its rows. An action is in that group because its due date arrived, not because it was chosen, and taking the mark off it there would change nothing you can see: it would still be out of time, still on the list, still first. A control whose only visible effect is that nothing happens does not teach that the group is not about picking - it teaches that the mark is broken. The mark is still there to put on and take off in "Picked" and in "Next actions", where it means what it says, and an action that is both due and picked simply appears in both groups.

A pick is **not a promise.** It is a hint that narrows the field of view for a few hours, made once in the morning by looking at "Next actions" - the whole system - and deciding what to aim at. Nothing is recorded when a picked action is not done, nothing is late, and nothing shouts. The commitment never lived in the mark: the action is still in "Next actions", still under its project, still reviewed, still stamped with how long it has been waiting. That is what makes it safe for the mark to expire silently, which nothing else in this app does.

The view carries no filters, for the reason the "Inbox" carries none. It is short by construction, and everything in it is either out of time or something you chose this morning; narrowing a narrowing would only be a way to look away from a deadline. When the picks turn out to be the wrong ones, the answer is not a filter here, it is "Next actions", one keystroke away.

Today has no review step. Everything in it is walked already, as part of a project, as a next action or as a waiting for item.

### Waiting for
Every next action with a non-empty "assigned to" field: commitments that are still tracked, but where the ball is not in your court.
This covers people (delegated to somebody) as well as things (an order placed, a form submitted, a PR awaiting CI).

Rules:
- a waiting for action is still a next action, so a project whose only next action is a waiting for one is **not** stalled
- it is excluded from the "Next actions" view, since it cannot be acted upon
- its age comes from `becameNextActionAt`, which for these items is the delegation date
- there is no automatic chasing. If a waiting for item has to be chased at a specific moment, the existing due date / `snoozeUntil` are used
- it is reviewed during the weekly review
- it carries the name filter, matching the action title - see "Filtering by name". The filter is on the title and not on "assigned to": the field is free text, so filtering by it would be filtering by however the name happened to be typed that day
- it carries the tag cloud as well - see "Filtering by tag"

### Calendar
Everything with a real deadline, soonest first: the actions whose due date is set and whose `completedAt` is empty. It answers "what is coming at me", which is a question no other view asks - "Next actions" is ordered by how long something has been available, not by when it runs out of time.

Every kind of action stands side by side here - standalone, inside a project, and parked - because what matters is the date, not where the action lives. Waiting for actions appear as well: chasing a delegation at a specific moment is exactly what the due date is for (see "Waiting for"), and the date is no less real for the ball being in somebody else's court.

Overdue items are loudly marked, the same way stalled projects are. A due date is by definition a date with consequences outside your control, so one that has passed is the loudest thing the app has to say.

Snoozed items appear here too, shown differently to mark them as not yet ready. A `snoozeUntil` reaching past the due date is an error state and is marked as one - see "Error state".

Projects are never here, and neither are inbox or someday/maybe items, because none of them carries a due date - see "Deliberate omissions".

#### Filters
The filters combine with **AND**, and reset the way they do everywhere else: each one on its own, plus a single control that clears them all.

- **name** - the shared name filter, matching the action title - see "Filtering by name"
- **due** - when it falls due, picked from a fixed list: **anytime** (the default), **today**, **tomorrow**, **this week**, **next week**. As in the "Archive" these are calendar periods and not rolling windows - "this week" is the week you are in, Monday to Sunday, and "next week" the one after it, not the next seven days. Anytime is how this filter resets.
- **tags** - the shared tag cloud - see "Filtering by tag"

The Calendar carries neither context, nor duration, nor needs focus. Those three ask whether something can be done right now, which is not the question this view asks.

**Overdue items are shown whatever the due filter says.** They are not what the filter is about: it asks what is coming, and something already late is not coming, it has arrived. Letting "today" hide an item that was due yesterday would be the app helping you look away from the one thing it exists to shout about - the same reason an action with no context survives the context filter in "Next actions".

The Calendar has no review step. Everything in it is walked through already, as part of a project, as a next action or as a waiting for item; having a deadline does not make it a second open loop.

### Archive
The completed commitments: projects and standalone actions whose `completedAt` is set, newest first. It is the finished mirror of "Projects" and "Tasks" - the same two halves that between them cover every commitment in the app, seen after the fact.

- a completed project is shown with the actions it was completed with, so what is kept is the whole thing and not a bare title
- a completed action that belonged to a project is **not** listed on its own. It is not a finished commitment, it is a finished step of one, and it is found with its project - here if the project is done, in the project itself while it is still running
- deleted items are not here. Deletion is not completion, and the audit log is where a trashed item is found and recovered from
- neither are the things done under the two minute rule. They never became items, so the audit log is their only record

#### Filters
The archive exists to answer "what did I do about X", and unfiltered it is only a pile that grows forever. The filters combine with **AND**, and reset the way they do everywhere else: each one on its own, plus a single control that clears them all.

- **name** - the shared name filter, matching a standalone action by its title and a project by its title or by the title of any action it was completed with - see "Filtering by name"
- **completed** - when it was finished, picked from a fixed list: **anytime** (the default), **today**, **yesterday**, **this week**, **last week**. These are calendar periods and not rolling windows - "this week" is the week you are in, Monday to Sunday, and "last week" the one before it, neither of them the last seven days. Anytime is how this filter resets. The list is short on purpose and there is no custom range: the archive is searched by what a thing was called far more often than by when it happened, and the near buckets are there mostly to answer "what did I actually get done today".
- **tags** - the shared tag cloud - see "Filtering by tag"

The archive carries neither context, nor duration, nor needs focus: those three ask whether something can be done right now, which is not a question the finished have.

It is a view like any other, so nothing is moved into it - an item is in it for exactly as long as `completedAt` is set. Clearing that field is therefore how something completed by mistake comes back to the active views, and like every other change it is audited.

The archive has no review step. Nothing in it is an open loop, so there is nothing in it that can silently die.

### Scheduler
The schedules, ordered by when they next fire.

It is the only view holding something you have not committed to, and the only one showing what is going to arrive rather than what already has. A one-shot leaves it the moment it fires; a cron schedule stays.

- it shows the text, the rule in readable form, when it next fires and when it last did
- it carries the name filter, matching the text of the schedule - see "Filtering by name"
- it carries no tag cloud. A schedule has no tags: it is not a commitment and belongs to no area of responsibility. What it produces does, once accepted
- it is reviewed during the weekly review, at step 6

### The read API
The views are readable from outside the app, so that an AI can analyse what is going on without anything being copied out by hand. It is the counterpart of the capture API (see "External capture"), which stays the only way in.

- what it returns is a **view**. The caller states its own filters as request parameters - the same filters the view itself offers, with the same semantics, and nothing beyond them - and no parameters means the complete, unfiltered view
- the caller's filters are its own: the screen's filter state is the screen's, and a read neither sees it nor touches it. An AI reading "Next actions" is asking its own question, not looking over your shoulder, and its answer must not depend on what you left toggled on last night
- there is no query language, and nothing can be asked for that a view does not already offer. Every possible response is a state the corresponding screen could be put in by setting its filters, so the API can never show a list the app itself could not - which is the property that matters. A caller composing arbitrary queries would be looking at a screen that cannot exist in the app, and that is the same reason saved filters are out of scope (see "Deliberate omissions")
- it is read only. Nothing is created, edited or completed through it. Whatever an outside tool wants to put into the app arrives in the inbox as a capture, and is decided about by hand in Inbox Zero
- reads are not audited. The audit log records what happened to an item, and a read makes no change worth recording. The one thing it can trigger is the daily clearing of `#today`, since an API read counts as first use of a new day, and that is never audited either - see "#today"

## Processes

### Inbox Zero
A dedicated mode that processes captured items one at a time, oldest first. It is used to empty the inbox, and also to process a someday/maybe item once you decide to move on it.
While the process runs everything else is hidden from view - only the current item is shown.
For each item the only question asked is: what is it? The answer is one of:

- **Trash**: the item is deleted. Recorded in the audit log.
- **Send to reference materials**: the item is not actionable, but is worth keeping - a manual, an account number, an article to come back to. It is sent out of the app, to wherever reference material is kept. This is an external action: the app itself stores no reference material. The branch exists so that such captures have a correct answer, instead of being trashed or parked in someday/maybe forever.
- **Action**: it is done in a single step. The item is converted into an action and must be created in valid form - the title starts with a verb and is self-descriptive; context and other optional fields may be filled in. Two further things are settled here, and neither is a branch of its own:
  - **who does it.** By default it is yours. Saying that someone else does it sets "assigned to", which is what puts the action in the "Waiting for" view; `becameNextActionAt` is stamped either way, and for a delegated action it is the delegation date. This is a property of the action, not a different answer to "what is it?" - the object created is the same one, holding the same title, the same context, the same home. An answer that differs from another by one field is that field.
  - **where it lives.** By default it is created standalone, so `becameNextActionAt` is stamped immediately - deciding it is worth doing is exactly what makes it a next action - and it appears in Tasks. An active project can be chosen instead - from the active projects, narrowed by the same name matching the name filter uses (see "Filtering by name") - in which case the action is created under that project, following the same default as adding an action from the project itself: it becomes the project's next action, parking it is one keystroke away (see "Editing items"). A stalled or snoozed project is a valid target: filing a next action into a stalled project is exactly what resolves the stall, and a project's snooze is about not being bugged, not about being closed to new next actions. A completed project is not: it is finished, and reopening one is not a decision to be made in passing while emptying the inbox.
  - a project is **chosen, never typed**. Text that matches nothing is a typo far more often than it is an intention, and a typo that silently becomes a second project is the one mistake here that is expensive to undo - the action is filed somewhere real, so nothing looks wrong until the outcome is being tracked in two places.
  - **a project that does not exist yet can be created here**, as an explicit choice rather than as what an unmatched name means. It is created with this action as its first and therefore its next action, so a definition of done is required exactly as it is in the Project branch. Without this, a step whose outcome you have not started tracking has no correct answer here: it would go in standalone and the outcome it belongs to would go unrecorded, which is the thing projects exist to prevent. Neither the project nor the action exists until the action is saved - both are written at once, so an abandoned form leaves nothing behind.
- **Two minute rule**: if it can be completed in under two minutes, it is done right now and marked as completed in the audit log, without being turned into a "proper" action first.
- **Project**: more than one action is needed. Requires:
  - a title that is a reference to the outcome, not a description of what to do (validated)
  - a DOD
  - at least one action. Actions are added one at a time, each written in the same form an action is written in anywhere else (see "Writing a project"), and the list can be reordered and pruned before the project is made - what is being decided here is the shape of the plan, and a plan is not written in the order it occurs to you. Every one of them becomes a next action, so none can be parked: a project created already stalled is a contradiction. Delegation is carried by each action's own meta line, because a delegated action belongs to a project exactly as validly as one you will do yourself
- **Someday/Maybe**: worth looking at some time, but not now. The item becomes a someday/maybe item, staying raw. The text may be edited to formulate the idea more clearly, and a `snoozeUntil` date may be set to exclude it from the weekly review requirement until that date - both optional, and both done on the item itself once it has landed rather than as a condition of filing it. Answering "what is it?" is the decision being asked for here; wording an idea better is a separate act, and one that reads differently once the idea is sitting among the others it will be reviewed with.
- **Keep incubating** (only when processing a someday/maybe item): still interesting, still not now. The item stays as it is, with a new `snoozeUntil`.

The process ends when the inbox is empty. The inbox should be emptied regularly, and always as part of the weekly review.

Oldest first is the default way through, and the reason is that it removes a decision: the mode exists to make deciding cheap, and choosing what to decide about next is a decision like any other. It is not a lock. A single item may be picked out of the inbox and processed on its own, ahead of the queue.

That escape hatch is deliberate, and it is not the name filter the "Inbox" view rejects (see "Filtering by name"). A filter hides items, and what it hides is what you did not want to look at; picking one item hides nothing - the rest of the inbox is still in front of you, still oldest first, and still has to be emptied. What it exists for is the case where holding the queue would do harm: something has arrived that has to be decided now, and working down to it means making every decision before it in a hurry. A rushed decision about the wrong thing is worse than one item taken out of order, and the whole point of the mode is that each decision gets made properly, once.

Both are reachable directly and neither is hidden behind the other, but the app says which is which - the interface offers the run first and processing one picked item second, so the default reads as the default (see implementation.md, "Processing from the Inbox").

The screen can also be left at any point, on any item, without answering the question. Nothing is written when it is and the item stays exactly as it was, because leaving is not an answer - it is declining to give one yet. This is the same reasoning as processing out of order, and it is worth stating in its own right: the mode exists to get each decision made **properly**, and an answer forced out of someone who does not have one yet is a wrong answer, not a decided item. An item sat with and left alone is in a better state than one filed hastily into the wrong branch, because the hasty one is now out of the inbox and out of sight, and nothing will bring it back for a second look.

Neither escape hatch weakens the rule that the inbox must be emptied. That rule is kept by the person and not by the software - see "The protocol is followed, not enforced" - and what the app owes it is a state that cannot be misread and a cheap way to act, which is the loud inbox, the count on the nav, and the review step that asks.

### Doing one action
Every list in this app is a list of things not yet done, and reading one is deciding. Doing is the opposite of deciding, and the screen that is right for choosing is wrong for working: with an action selected in any view that shows actions, one key puts that action alone on the screen, large, and takes everything else off it - the other items, the counts, the filters, the badges on its own row.

- **it is a mode, not a view.** Nothing is queried, nothing is stored, and nothing about the action changes by looking at it this way. Leaving puts you back in the view you came from with the same row still selected, because you never went anywhere
- **two keys work in it and the rest do nothing**: **done**, which completes the action exactly as completing it from the list does - the project check included, see "Completing a next action" - and **back**, which leaves the mode. Completing also leaves it, since the thing being done is finished. A third key would be a decision, and the point of the mode is that there are none in it left to make
- **what it shows is the title and nothing else.** Not the project, not the context, not the due date: those are what you needed in order to pick this action, and this screen is for after the picking. An action whose title does not say what to do is an action that was written badly (see "Writing an action"), and hiding that is not a kindness
- **it is offered on an action and only while the action is open.** Not on an inbox item, which has not been decided about yet; not on a someday/maybe item, which is not committed to; not on something already completed, which has nothing left to do
- **how much of the app's own furniture goes with it** - the navigation rail, the key bar along the bottom - is a setting and not a rule. How bare a screen has to be before it stops pulling at you is a fact about a person, not about the app, so the app takes an answer rather than having an opinion (implementation.md, "Doing mode")
- **a timer is offered, off by default, and it is never written down.** Switched on, it counts the minutes since this action went on the screen - `07`, and `1:04` once an hour has gone. It is for one thing only: a feel for how long work actually takes, built up by noticing rather than by measuring. Nothing reads it, nothing stores it, no item carries it and no view reports it, so leaving the mode loses it and coming back starts it at `00` again. That is deliberate. A number that was kept would become a record of how long you took, and an action would arrive with an expectation attached - an estimate to beat, then a target, then a reason to feel late about work that was never promised for a time. This app does not do that to next actions (see "#today"), and a stopwatch is the shortest route back to it

### Completing a next action
Completing a next action is the moment with the most context about what comes next, so the project is checked right there:

- there are still open actions, and at least one of them is marked as a next action - nothing is asked, the completion is accepted silently
- there are still open actions, but none of them is marked as a next action - ask to mark one of them as the next action
- there are no open actions left - ask whether to complete the project, showing the DOD for reference, or to create a next action
- in that last case, doing nothing is always allowed. If there is no time or energy to decide right now, nothing is forced and the project immediately becomes stalled

Completing a standalone action asks nothing. There is no project to check and nothing to leave stalled - it is simply done.

### Weekly review
The ritual that keeps the views trustworthy. Without it they silently go out of date, and a view that is not trusted to be complete is a view that stops being used. Everything else in this document is bookkeeping in service of this process.

The review is guided, and runs in a fixed order:

0. **Gather** - collect everything from the other places captures land in (calendar - past days as well as the weeks ahead - messengers, mail, ...) into the inbox, so that the inbox really does hold all open loops. Looking ahead in the calendar is what triggers preparation actions, and is also the moment to check that due dates in the app and the external calendar still agree, since that sync is manual.
1. **Get clear** - run Inbox Zero until the inbox is empty. Non-negotiable.
2. **Waiting for** - walk the "Waiting for" view. Anything stale is chased, or gets a due date / `snoozeUntil`.
3. **Projects** - for each active project: is the DOD still what you want, and does it have a next action? This is where stalled projects, and projects left without a DOD, are fixed. Snoozed projects are skipped.
4. **Next actions** - still valid, still a real physical next action? An action that has been next for weeks without moving usually means the action is phrased wrong, not that you are lazy. Standalone actions are covered here, since every one of them is a next action.
5. **Someday/Maybe** - promote, re-snooze or trash. Snoozed items are skipped.
6. **Scheduler** - walk the schedules: is this still wanted, and is the rule still right? A schedule set eight months ago goes on firing whether or not the reason for it still exists, and this is the only place that can be noticed before it lands in the inbox again.

The review is resumable. It can be interrupted at any point and continued later, and does not have to be finished in one sitting.

Progress is tracked by the per-item `lastReviewedAt`, stamped as each item is walked through and prefilled with the creation date when the item is created. There is no global "last weekly review" record: an item that is not snoozed and whose `lastReviewedAt` is older than a week is simply outstanding, and that is also how the app shows that a review is due. A freshly created item is by construction not outstanding - it was consciously looked at when it was made.

### Editing items
Every item stays editable after it is created, and every edit is recorded in the audit log (see "Audit entry"). Nothing in the app is written once.

Editing happens in two places:

**Inline, in the views.** The cheap changes are made where the item is already shown: renaming an action, toggling a tag, setting a `snoozeUntil` or a due date, marking an action as next or parking it. These are the changes noticed while scanning a list, and making them cost a screen transition is the friction that ends with them not being made at all.

**In the item itself.** Opening a project or an action shows every field it has, editable, and for a project the full list of actions under it: add one, delete one, rename one, detach one (see "Reshaping items"). This is where a project is actually worked on. The DOD is prose and it is the field step 3 of the weekly review asks about, so it needs the room a list does not have.

A project is reachable this way from everywhere it appears - "Projects", the "Calendar", the "Archive" - and from any of its actions, wherever that action is seen.

Rules:

- **an action added to a project is a next action**, with parking one keystroke away. The default is deliberate, because the costs are asymmetric: a wrongly parked action is invisible to "Next actions", to the stalled project check and to the weekly review - it silently dies, which is the failure mode this whole document is built against - while a wrongly next action merely turns up in the working view, where it is seen and parked. Parking is the deliberate act, so it is the one that has to be performed
- **removing an obsolete action is deleting it**, which is how any action that is not completed gets resolved (see "Completion"). It is audited and recoverable. Deleting the last open action of a project is allowed, and leaves the project stalled and shouting about it
- **editing does not stamp `lastReviewedAt`.** A review is the deliberate act of walking an item and asking whether it is still what you want, and fixing a typo is not that - neither is editing the DOD. If an edit counted as a review, the outstanding list, which is how the app knows a review is due, could be silenced by cosmetic changes. It is the one thing that has to stay trustworthy
- **the validations are the ones from the Project branch of Inbox Zero** (see "Inbox Zero"), with one asymmetry: the title is still required and still has to be a reference to the outcome, but an existing project may be left without a DOD and without any action. Neither is prevented, both are marked loudly instead - no DOD is an error state, no next action is stalled. Requiring them at creation is not the same demand: Inbox Zero is the deliberate act of deciding what a thing is, and that is the moment those answers are cheapest and most honest

### Reshaping items
Nothing is ever retyped. When an item turns out to be the wrong shape it is converted, carrying over everything it already has.

**Detach** - an action leaves its project and becomes standalone, so it appears in Tasks from then on. Used when the action turns out not to belong to the scope of the project after all, and when closing a project that still has open actions. It keeps its title, context, duration, tags, description and dates - with one exception: a standalone action is always a next action, so a parked action gets `becameNextActionAt` stamped with the detach time. Nothing leaves a project into limbo.

**Attach** - the mirror of Detach: a standalone action joins an existing active project, picked by name the same way the Action branch of Inbox Zero picks one (see "Inbox Zero"). It keeps its title, context, duration, tags, description and dates unchanged, including `becameNextActionAt` - a standalone action is always a next action, and it stays exactly as next as it already was, simply under a project now. Used when an action was filed standalone and later turns out to belong to a project - either because it was captured before the project existed, or because it should have been filed under it from the start.

**Promote** - a standalone action becomes a project, because it turns out to need more than one step. Promotion runs the same Project branch as Inbox Zero, and is therefore subject to the same validations, with the fields prefilled from the action:
- the project title is prefilled from the action title, and has to be edited into a reference to the outcome rather than a description of what to do
- tags carry over to the project, on its meta line; everything else the action carried is left behind with it, since a context or a size describes doing something and a project is not something you do (see "Writing a project"). The description carries over to the first action, since a project has no description of its own (see "Deliberate omissions")
- a DOD is required
- at least one action is required, which becomes the next action

An action that belongs to a project and should become a project of its own is first detached, then promoted.

## Out of scope

### Deliberate omissions
Things consciously left out, recorded here so that they do not come back later as fresh ideas.

- **Priority.** No priority field, no P1 / P2 / P3. It is subjective and unstable - what matters on Monday does not on Thursday - and re-ranking things feels productive while producing nothing. Real urgency is already carried by the due date, and importance comes out of the weekly review and the areas of responsibility carried by tags.
- **A "Tasks" project.** Standalone actions get a home as the Tasks **view**, never as a special project record. Such a project would need no DOD, no stalled check, no completion and no deletion - every defining property of a project removed, leaving only the name. Worse, a project exempt from the stalled check is a project nothing watches, which would make it a second inbox for things that were decided to be worth doing and then never shouted about again.
- **Horizons 3 to 5.** No goals, no vision, no purpose level. Areas of responsibility (horizon 2) are carried by tags, and that is where it stops. The levels above are journal territory, not something this app models.
- **Saved filters.** The Next actions filters are momentary state - never named, never saved as presets. A saved filter is a view under another name, and views can not be created: the moment there are five saved filters there are five screens that each show a part of the truth, and no way to tell which one is the complete list.
- **A due date on a project.** Deadlines belong to actions. A project can not be acted on, so a deadline on one is an alarm with no lever: when it fires you go and look at its actions anyway. It also fires at the wrong moment - learning on 30 March that the return is due on 31 March is worth nothing, because the value of that deadline was in the six weeks before it. And it is the field most exposed to the invented deadline the due date exists to keep out: a project is an outcome, and outcomes invite targets, while "call the plumber by June" is obviously silly. An outcome deadline is carried by a real action instead - "File the tax return", due 31 March, parked until it can be started. Being made to write that action is the point: it names what done looks like operationally, the same discipline the DOD imposes.
- **Structured scheduled submissions.** A schedule produces raw text and never a ready made action or project with its fields filled in. Anything able to inject a formed project would have to enforce the title, DOD and at-least-one-action rules at that boundary too, or it becomes a hole that lets malformed projects past the discipline Inbox Zero exists to impose. It would also usually be wrong: a recurring outcome differs every time it comes round - this year the tyres may be worn and need replacing first - so re-instantiating last time's action list would be re-instantiating the wrong plan.
- **Interval based recurrence.** No "every 3 days after I last did it" in a schedule. Cron describes the calendar and knows nothing about your last completion, and adding a second kind of rule to schedules would double the concept to cover a case that is already covered: complete the action, and put a `snoozeUntil` on the next one. Watering the plants late simply shifts the next watering, which is exactly what a snooze does.
- **Reference material storage.** The app keeps no reference material of its own. Material that belongs to a specific commitment lives in the description of the **action** it belongs to; everything else leaves through the "send to reference materials" branch of Inbox Zero and is kept outside the app. A project has no description of its own: it has a definition of done, which is the one thing about a project worth writing down, and a free-text field beside it was somewhere for the same sentence to be written a second time in weaker words. Material a project needs belongs to whichever of its actions needs it - and material that belongs to none of them is not material this project needs, it is reference material, which leaves.
