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

### Design principles
- add only functionality that I will use, don't add anything for future development
- app should be fast, it usage should not be obstacle
- the keyboard only support should be provided
- the app should be AI friendly, so AI could get info from it for analysis and control the info send to it (using inbox) - see "The read API" and "External capture"
- the design of the app should allow to follow principles described in David Allen's book "GTD - Getting Things Done"

## Items
The objects the app works with:
- **inbox item**: a raw, unprocessed capture, that has not been decided about yet
- **someday/maybe item**: a raw capture that is worth revisiting some time, but not now
- **action**: a single non-breakable task, that can be done and have visible output effect
- **project**: when end result can't be achieved in result of single action it is called a project, it contains a list of actions, and has a "definition of done"
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
- Duration: (optional) how much time the action needs, as a coarse bucket: `<5min`, `<15min`, `<1h`, `>1h`. Buckets and not minutes on purpose - free form estimates demand a precision that is not there, and force estimating things that are not worth estimating. The `>1h` bucket also carries the original meaning: do not start this unless there is enough time to finish it in one sitting, like reading a long article.
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
- Description: (optional) any extra materials worth keeping with the project (URL, link to an email, reference to a PDF etc)
- Tags: (optional) zero, one or several labels - see "Tags"
- Actions: a list of actions required to complete a project. In most cases it is enough to have only one next action to move the project forward. But in some cases listing more steps in advance during the planning phase is helpful.

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
- due date: (optional, projects and actions) a real, externally imposed deadline, after which there are consequences outside your control. It is not a way to hide an item until a date and not a self-imposed target - invented deadlines are what makes the real ones stop working. Deferring something to a date is what `snoozeUntil` is for. It is what the "Calendar" view is built on, and it is shown wherever the item appears
- lastReviewedAt: (required, projects, actions and someday/maybe items) when the item was last reviewed. Stamped with the creation date when the item is created - creating an item is always a conscious act, so creation counts as its first review, and the field is never empty. It drives the weekly review: it shows what has already been walked through and what is still outstanding, which is what makes an interrupted review resumable
- becameNextActionAt: (optional, actions only) when the action became a next action. An empty field means the action is not a next action - it is parked, written down in advance during planning. Only an action inside a project can be parked; a standalone action always has this field set - see "Standalone actions". A **real** next action is one where `becameNextActionAt` is set and `completedAt` is still empty. The field doubles as the age of the next action, which is what shows an action that has been next for a long time without moving, and for actions with "assigned to" set it is also the delegation date. Because it is also the delegation date, changing "assigned to" restamps it: delegating an action starts a new clock - you stopped waiting on yourself and started waiting on them - and taking an action back restamps it again for the same reason in reverse. Without the restamp, an action that had been next for three weeks and was then delegated would look three weeks stale in the "Waiting for" view on day one.
- snoozeUntil: (optional, projects, actions and someday/maybe items) marks the item as not yet ready to be worked on, until that date passes
- completedAt: (optional, projects and actions) when the item was completed. Being set is what makes the item done - there is no separate status flag

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

#### #today
`#today` marks an action as picked for the day - see "Today". It is an ordinary tag in every respect but one: it expires.

- it is applied and removed by hand, like any other tag, from anywhere a tag can be edited
- it is cleared from every item on the first use of the app on a new day, by the **local** day - the same boundary "due today" uses. The clearing is deliberately not a scheduled sweep: a sweep only runs if something is up to run it, and a "Today" still showing yesterday's picks because a machine was asleep is the view being quietly wrong in the direction that matters most. Clearing on arrival cannot be observed stale, because nothing looks at the view before you do
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

## Views
Every list the app shows is a view: a query over the items. No **item** is ever stored in a view, and a view can not be created, renamed or deleted - which is what makes the ones below permanent fixtures, and what makes each of them free.

Opening a single item to work on it is not a view, and not an exception to this either: an item shown in full is the item, holding exactly what it held in the list, and nothing is kept there - see "Editing items". The guided processes are the same, showing one item at a time - see "Processes".

A due date is shown in every view the item carrying it appears in, and an overdue one is marked loudly, the same way a stalled project is. A deadline is the one thing that can not wait for the right screen to be opened, which is why it is not left to the "Calendar" alone - that view is where deadlines are ordered and asked about, not where they are learned of.

Filters are the one piece of state a view remembers, and they are not items: they decide which items a query returns, and never what exists. Nothing is created, moved or lost by filtering, and turning every filter off gives the complete list back. A filtered view says so loudly - which filters are on and how many items they are hiding - with the reset next to it, because a view quietly showing part of itself is exactly how a view stops being trusted.

### Filtering by name
Every view that can grow long carries the same name filter, and it behaves identically in all of them: **Someday/Maybe**, **Projects**, **Tasks**, **Next actions**, **Waiting for**, the **Calendar** and the **Archive**.

Matching is case insensitive. Several words may be given and **all** of them have to be present, in any order and anywhere in the name - `call bank` finds "Call the bank about the mortgage". Each word matches as a substring and not as a whole word, so `mortg` still finds it. Substrings and not fuzzy matching, so that it is always obvious why something matched. Clearing the box is how it resets.

What counts as the name is whatever names the item on that screen: the title of an action or a project, and for a someday/maybe item its text, since that is all it has. For a **project**, the titles of the actions under it count as part of its name as well - a project is remembered by a step in it at least as often as by its outcome, and hiding a project whose action matched would be hiding the answer.

The **Inbox** deliberately has no name filter. It is worked through one item at a time, oldest first, until it is empty, and a filter there would only be a way to look away from something. Neither does **Today**, for a related reason - see "Today".

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

- **out of time** - every project and action due today or already overdue, ordered by due date with the overdue first. This is the "Calendar" today-and-earlier slice unchanged, down to the projects, the parked actions and the waiting for ones. Anything narrower would let something be out of time in one view and not in the other
- **picked** - the actions carrying `#today` (see "#today"), ordered by age the way "Next actions" is

A pick is **not a promise.** It is a hint that narrows the field of view for a few hours, made once in the morning by looking at "Next actions" - the whole system - and deciding what to aim at. Nothing is recorded when a picked action is not done, nothing is late, and nothing shouts. The commitment never lived in the mark: the action is still in "Next actions", still under its project, still reviewed, still stamped with how long it has been waiting. That is what makes it safe for the mark to expire silently, which nothing else in this app does.

The view carries no filters, for the reason the "Inbox" carries none. It is short by construction, and everything in it is either out of time or something you chose this morning; narrowing a narrowing would only be a way to look away from a deadline. When the picks turn out to be the wrong ones, the answer is not a filter here, it is "Next actions", one keystroke away.

Today has no review step. Everything in it is walked already, as a project, a next action or a waiting for item.

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
Everything with a real deadline, soonest first: the projects and actions whose due date is set and whose `completedAt` is empty. It answers "what is coming at me", which is a question no other view asks - "Next actions" is ordered by how long something has been available, not by when it runs out of time.

Projects and actions stand side by side here, the same way standalone and project actions do in "Next actions": what matters is the date, not which kind of commitment carries it. Waiting for actions appear as well - chasing a delegation at a specific moment is exactly what the due date is for (see "Waiting for"), and the date is no less real for the ball being in somebody else's court.

Overdue items are loudly marked, the same way stalled projects are. A due date is by definition a date with consequences outside your control, so one that has passed is the loudest thing the app has to say.

Snoozed items appear here too, shown differently to mark them as not yet ready. A `snoozeUntil` reaching past the due date is an error state and is marked as one - see "Error state".

Inbox and someday/maybe items are never here, because neither carries a due date. Nothing has been committed to yet, so there is nothing that can be late.

#### Filters
The filters combine with **AND**, and reset the way they do everywhere else: each one on its own, plus a single control that clears them all.

- **name** - the shared name filter, matching an action by its title and a project by its title or by the title of any action under it - see "Filtering by name"
- **due** - when it falls due, picked from a fixed list: **anytime** (the default), **today**, **tomorrow**, **this week**, **next week**. As in the "Archive" these are calendar periods and not rolling windows - "this week" is the week you are in, Monday to Sunday, and "next week" the one after it, not the next seven days. Anytime is how this filter resets.

- **tags** - the shared tag cloud - see "Filtering by tag"

The Calendar carries neither context, nor duration, nor needs focus. Those three ask whether something can be done right now, which is not what this view asks, and none of them is a field a project has at all.

**Overdue items are shown whatever the due filter says.** They are not what the filter is about: it asks what is coming, and something already late is not coming, it has arrived. Letting "today" hide an item that was due yesterday would be the app helping you look away from the one thing it exists to shout about - the same reason an action with no context survives the context filter in "Next actions".

The Calendar has no review step. Everything in it is walked through already, as a project, a next action or a waiting for item; having a deadline does not make it a second open loop.

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

### The read API
The views are readable from outside the app, so that an AI can analyse what is going on without anything being copied out by hand. It is the counterpart of the capture API (see "External capture"), which stays the only way in.

- what it returns is a **view**, with its filters applied: exactly what the corresponding screen would show, item for item
- there is no query language, and nothing can be asked for that a view does not already offer. A caller composing arbitrary queries would be looking at a screen that does not exist in the app - it could disagree with every view, and there would be no way to tell which one was the complete list. That is the same reason saved filters are out of scope (see "Deliberate omissions")
- it is read only. Nothing is created, edited or completed through it. Whatever an outside tool wants to put into the app arrives in the inbox as a capture, and is decided about by hand in Inbox Zero
- reads are not audited. The audit log records what happened to an item, and reading one changes nothing

## Processes

### Inbox Zero
A dedicated mode that processes captured items one at a time, oldest first. It is used to empty the inbox, and also to process a someday/maybe item once you decide to move on it.
While the process runs everything else is hidden from view - only the current item is shown.
For each item the only question asked is: what is it? The answer is one of:

- **Trash**: the item is deleted. Recorded in the audit log.
- **Send to reference materials**: the item is not actionable, but is worth keeping - a manual, an account number, an article to come back to. It is sent out of the app, to wherever reference material is kept. This is an external action: the app itself stores no reference material. The branch exists so that such captures have a correct answer, instead of being trashed or parked in someday/maybe forever.
- **Action**: it is done in a single step and needs no project. The item is converted into an action and must be created in valid form - the title starts with a verb and is self-descriptive; context and other optional fields may be filled in. It is created standalone, so `becameNextActionAt` is stamped immediately - deciding it is worth doing is exactly what makes it a next action - and it appears in Tasks.
- **Two minute rule**: if it can be completed in under two minutes, it is done right now and marked as completed in the audit log, without being turned into a "proper" action first.
- **Someone else does it**: the item is not yours to act on. It becomes an action with "assigned to" set, and appears in the "Waiting for" view. `becameNextActionAt` is stamped as usual, and here it is the delegation date.
- **Project**: more than one action is needed. Requires:
  - a title that is a reference to the outcome, not a description of what to do (validated)
  - a DOD
  - at least one action, which becomes the next action
- **Someday/Maybe**: worth looking at some time, but not now. The item becomes a someday/maybe item, staying raw. The text may be edited to formulate the idea more clearly. Optionally a `snoozeUntil` date can be set, to exclude it from the weekly review requirement until that date.
- **Keep incubating** (only when processing a someday/maybe item): still interesting, still not now. The item stays as it is, with a new `snoozeUntil`.

The process ends when the inbox is empty. The inbox should be emptied regularly, and always as part of the weekly review.

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

**Promote** - a standalone action becomes a project, because it turns out to need more than one step. Promotion runs the same Project branch as Inbox Zero, and is therefore subject to the same validations, with the fields prefilled from the action:
- the project title is prefilled from the action title, and has to be edited into a reference to the outcome rather than a description of what to do
- tags and description carry over
- a DOD is required
- at least one action is required, which becomes the next action

An action that belongs to a project and should become a project of its own is first detached, then promoted.

## Out of scope

### Recurring items
Recurring actions and projects are **out of scope for this document** and need a separate design pass. The problem is acknowledged rather than solved: nothing in the system currently repeats, including the weekly review itself.

The discussion, the candidate direction and the questions blocking it live in [recurring.md](recurring.md).

### Deliberate omissions
Things consciously left out, recorded here so that they do not come back later as fresh ideas.

- **Priority.** No priority field, no P1 / P2 / P3. It is subjective and unstable - what matters on Monday does not on Thursday - and re-ranking things feels productive while producing nothing. Real urgency is already carried by the due date, and importance comes out of the weekly review and the areas of responsibility carried by tags.
- **A "Tasks" project.** Standalone actions get a home as the Tasks **view**, never as a special project record. Such a project would need no DOD, no stalled check, no completion and no deletion - every defining property of a project removed, leaving only the name. Worse, a project exempt from the stalled check is a project nothing watches, which would make it a second inbox for things that were decided to be worth doing and then never shouted about again.
- **Horizons 3 to 5.** No goals, no vision, no purpose level. Areas of responsibility (horizon 2) are carried by tags, and that is where it stops. The levels above are journal territory, not something this app models.
- **Saved filters.** The Next actions filters are momentary state - never named, never saved as presets. A saved filter is a view under another name, and views can not be created: the moment there are five saved filters there are five screens that each show a part of the truth, and no way to tell which one is the complete list.
- **Reference material storage.** The app keeps no reference material of its own. Material that belongs to a specific commitment lives in the description of that action or project; everything else leaves through the "send to reference materials" branch of Inbox Zero and is kept outside the app.
