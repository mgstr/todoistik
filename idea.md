# todoistik
Describes todo application, intended for my purposes only. This app is not intended as a generic todo app.

## Design principles
- add only functionality that I will use, don't add anything for future development
- the design of the app should allow to follow principles described in David Allen's book "GTD - Getting Things Done"

## Overview
App should allow manipulation with following entities:
- inbox item: a raw, unprocessed capture, that has not been decided about yet
- someday/maybe item: a raw capture that is worth revisiting some time, but not now
- action: a single non-breakable task, that can be done and have visible output effect
- project: when end result can't be achieved in result of single action it is called a project, it contains a list of actions, and has a "definition of done".

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

### Actions
An action is a single (non-breakable into smaller parts) task, that should be done in order to move to the desired goal.
The action should have visible effect. So "thinking about design" is not an action. Use "Write draft a MD with design" instead.
Action has following fields:
- Title: ideally it should start with a verb and be fully self-descriptive, avoiding letting something to be in context. So when looking at the action title you don't have to think before you start doing it.
- Context: (optional) the physical prerequisite for doing the action, at most one - see "Contexts"
- Duration: (optional) how much time the action needs, as a coarse bucket: `<5min`, `<15min`, `<1h`, `>1h`. Buckets and not minutes on purpose - free form estimates demand a precision that is not there, and force estimating things that are not worth estimating. The `>1h` bucket also carries the original meaning: do not start this unless there is enough time to finish it in one sitting, like reading a long article.
- Needs focus: (optional) marks an action that can not be done while tired. Deliberately a single flag rather than a low / normal / high scale - having to grade the energy of every action puts pressure on capture, which is exactly the friction worth avoiding.
- Description: (optional) any extra materials needed to be referenced (like URL, link to email, reference to PDF etc) that could be useful during the action.
- Tags: (optional) zero, one or several labels - see "Tags"
- Assigned to: (optional) free text. If not set, it is assumed that you are the one who should do it. If set, the action is waiting on somebody or something else, and appears in the "Waiting for view".

An action does not have to belong to a project, and most do not. A single action that fully achieves its outcome stands on its own and is never wrapped in a project just to give it a parent - that bureaucracy is what makes a system get abandoned. A standalone action is a next action by exactly the same rule as any other action, and the stalled project check simply does not apply to it.

### Project
Project is a desired result, that requires more than one step to complete.
Project has following fields:
- Title: name that helps to reference the result
- DOD (definition of done): required, since it helps to define what is the expected outcome of the project, and used during review and decision what is the next action
- Description: (optional) any extra materials worth keeping with the project (URL, link to an email, reference to a PDF etc)
- Tags: (optional) zero, one or several labels - see "Tags"
- Actions: a list of actions required to complete a project. In most cases it is enough to have only one next action to move the project forward. But in some cases listing more steps in advance during the planning phase is helpful.

A next action is not a property of the project. It is a property of the action - see `becameNextActionAt`. A project can therefore have several next actions at the same time, which is what a parallel project looks like (booking the flight, renewing the passport and asking for time off are all available at once), while a sequential project simply happens to have one.

### Time fields
Time related fields, and the items each one applies to:
- creation date: (required, all items) when the item was created, used to calculate its age
- due date: (optional, projects and actions) a real, externally imposed deadline, after which there are consequences outside your control. It is not a way to hide an item until a date and not a self-imposed target - invented deadlines are what makes the real ones stop working. Deferring something to a date is what `snoozeUntil` is for
- lastReviewedAt: (optional, projects and actions) when the item was last reviewed. It drives the weekly review: it shows what has already been walked through and what is still outstanding, which is what makes an interrupted review resumable
- becameNextActionAt: (optional, actions only) when the action became a next action. An empty field means the action is not a next action - it is parked, written down in advance during planning. A **real** next action is one where `becameNextActionAt` is set and `completedAt` is still empty. The field doubles as the age of the next action, which is what shows an action that has been next for a long time without moving, and for actions with "assigned to" set it is also the delegation date.
- snoozeUntil: (optional, projects, actions and someday/maybe items) marks the item as not yet ready to be worked on, until that date passes
- completedAt: (optional, projects and actions) when the item was completed. Being set is what makes the item done - there is no separate status flag

`snoozeUntil` is a universal field and means the same thing everywhere it appears - on projects, actions and someday/maybe items: do not bother me about this until that date. It is how an already clarified commitment is shelved for a while without losing its DOD, its actions and the material collected in it.

`snoozeUntil` is also what covers deferral - "there is no point looking at this before Tuesday" - so there is no separate defer date.

A snoozed item is **not hidden**. It stays visible and is shown differently, to indicate that it is not yet ready to be worked on. Hiding it would be confusing: a project whose only action had become invisible would look stalled while the app insists it is not.

What a snooze actually does:
- it excludes the item from the weekly review requirement until the date passes
- a snoozed **project** is exempt from the stalled project check
- a snoozed **action** still counts as a next action of its project, so deferring a single action does not make the whole project look stalled. The stalled project check knows about snoozed actions. This is the same exemption a waiting for action gets, and for the same reason

The single exception is the inbox: an inbox item has no `snoozeUntil`. Snoozing an inbox item is the same thing as moving it to the someday/maybe list. Emptying the inbox is a non-negotiable rule and must not be avoidable by snoozing.

### Completion
An item is resolved explicitly, and only in one of two ways: it is completed, or it is deleted. There are no shortcuts and nothing is resolved implicitly.

- a completed action leaves the next actions list and stops counting as a next action for its project, which may leave the project stalled
- a project can not be completed **or deleted** while it still has open actions. Every one of them is resolved explicitly first: completed, deleted, or detached into a standalone action (see "Reshaping items")
- completing a project is therefore always a deliberate act, and the moment the DOD is confirmed to be met. A project is never completed automatically just because it ran out of actions
- there is no separate "done" list. The audit log is the record of what was finished

### Contexts
A context is a physical prerequisite for doing an action: something that has to be true before the action is possible at all. If the action could be done without it, it is not a context.

Contexts apply to **actions only**. A project is not something you do, so it has no context.

An action has **at most one** context. Notation is `@name`: `@home`, `@garage`, `@online` (an internet connection is needed, on any device), `@computer` (a real computer is needed, a phone will not do).

#### Parameters
A context may carry a parameter: `@person(Andres)`, `@grocery(Selver)`. This keeps the context namespace small and scannable, which is the only reason contexts are useful at all - putting every person and every shop chain at the top level would destroy that.

- the parameterised form is **narrower** than the bare one. Standing in Selver satisfies `@grocery(Selver)` and bare `@grocery`, but not `@grocery(Prisma)`
- the bare form is not always meaningful. Bare `@grocery` is useful ("buy milk, any shop"), bare `@person` is not. Some context types will in practice always carry a parameter, and that is fine
- parameter values are picked from a remembered list per context type, never typed fresh, otherwise `@person(Andres)`, `@person(andres)` and `@person(Andres P.)` become three different contexts
- that list has to be editable, so that values that are no longer used can be removed

#### Filtering
The "what can I do right now" view filters by one or several contexts, combined with **OR**: at home, with a computer and an internet connection means `@home OR @computer OR @online`.

OR is the correct combination precisely because an action carries a single context - the question being asked is "is this action's context among the ones I currently satisfy". The cost of the single context is that an action needing two prerequisites at once has to name the scarcer one; this is accepted.

### Tags
A tag is a label used to filter and categorise. Unlike a context it is not a precondition - it says nothing about whether an item can be done, only about what it is about.

Notation is `#name`: `#car`, `#finance`, `#hobby`, `#programming`.

- tags apply to **both projects and actions**
- an item can have zero, one or several tags
- in practice these are not arbitrary keywords but the standing areas of responsibility that work belongs to. That makes them the thing that answers the review question "which part of my life am I starving?"

## Stalled projects
An active project is stalled when it has no next action.

This is the single most common way things silently die: the project stays on the list, looks alive, and nothing ever moves. Catching it is the highest value check in the app, and it costs nothing - it is derived, never stored.

- a project is exempt while it is snoozed, and once it is completed
- a project whose only next action is a waiting for action is **not** stalled

### On completing a next action
Completing a next action is the moment with the most context about what comes next, so the project is checked right there:

- there are still open actions, and at least one of them is marked as a next action - nothing is asked, the completion is accepted silently
- there are still open actions, but none of them is marked as a next action - ask to mark one of them as the next action
- there are no open actions left - ask whether to complete the project, showing the DOD for reference, or to create a next action
- in that last case, doing nothing is always allowed. If there is no time or energy to decide right now, nothing is forced and the project immediately becomes stalled

### Visibility
Stalled projects stay visible in the normal lists, clearly marked as stalled (red, or similarly loud). They are not hidden away in a dedicated screen, and they are not something only the weekly review surfaces.

The app never prevents a project from being stalled. Forcing a next action to be invented at a moment when there is no time or energy for it produces a bad action, and a bad action is worse than a stalled project that is shouting about itself and will be dealt with at the weekly review or sooner.

## Error state
An item whose fields contradict each other is in an error state. It stays highly visible until it is fixed, the same way a stalled project does, and is dealt with at the weekly review or whenever there is time.

The known case: `snoozeUntil` set past the due date. The item would stay marked as not yet ready to be worked on until after the moment it was supposed to be finished, which is never what was meant.

Such a combination is not silently resolved by letting one field win over the other - that would hide the mistake instead of the item. The app makes an effort to avoid the situation when the dates are entered, and if it still occurs, the item is marked as being in error rather than quietly reinterpreted.

## Lists
There are exactly three lists. Everything else the app shows is a **view** derived from them - a query, not a place where anything is stored.

1. **Inbox** - captured items that have not been decided about yet. Must be emptied, see "Inbox Zero".
2. **Someday/Maybe** - raw ideas worth revisiting some time, but not now.
3. **Projects + standalone actions** - everything that is an actual commitment: projects with their actions, and the standalone actions that belong to no project.

### Someday/Maybe
A first class list, holding raw ideas that are worth looking at some time, but that you are not ready to work on now.

A someday/maybe item is not a project and not an action - it is the same raw, unclarified capture as an inbox item. Clarifying it would mean defining an outcome and a next action for something you have deliberately decided not to commit to, which is wasted work and is exactly the friction that makes a someday list go unused. It therefore stays raw until you decide to move on it.

Rules:
- items arrive here from the inbox, as one of the outcomes of Inbox Zero
- the creation date shows the age of the idea
- `snoozeUntil` (optional) excludes the item from the weekly review requirement until that date, so that a long someday list stays reviewable
- when you decide to move on an item, it is processed exactly the same way as an inbox item (see Inbox Zero)
- it is reviewed during the weekly review, skipping items that are still snoozed

## Views
Views are derived from the projects + standalone actions list. Nothing lives in a view.

### Next actions view
The actions that are on you to act on: `becameNextActionAt` is set, `completedAt` is empty and "assigned to" is empty.

Note the distinction in naming. A waiting for action is still a next action of its project - that is what keeps a delegated project off the stalled list - but it does not appear in this view, because this view is only the actions that are yours to act on.

Snoozed actions appear here as well, shown differently to mark them as not yet ready. They still count as a next action of their project for the stalled project check.

### What can I do right now
The main working view: next actions filtered by the three things that decide whether something is doable at this moment.

- **context** - one or several of the contexts currently satisfied, combined with OR (see "Contexts")
- **duration** - what fits in the time available
- **needs focus** - what can be faced with the energy available

### Waiting for view
Every next action with a non-empty "assigned to" field: commitments that are still tracked, but where the ball is not in your court.
This covers people (delegated to somebody) as well as things (an order placed, a form submitted, a PR awaiting CI).

Rules:
- a waiting for action is still a next action, so a project whose only next action is a waiting for one is **not** stalled
- it is excluded from the "what can I do right now" view, since it cannot be acted upon
- its age comes from `becameNextActionAt`, which for these items is the delegation date
- there is no automatic chasing. If a waiting for item has to be chased at a specific moment, the existing due date / `snoozeUntil` are used
- it is reviewed during the weekly review

### Projects
The active projects, with stalled ones loudly marked and snoozed ones shown differently to mark them as not yet ready.

## Processes

### Inbox Zero
A dedicated mode that processes captured items one at a time, oldest first. It is used to empty the inbox, and also to process a someday/maybe item once you decide to move on it.
While the process runs everything else is hidden from view - only the current item is shown.
For each item the only question asked is: what is it? The answer is one of:

- **Trash**: the item is deleted. Recorded in the audit log.
- **Send to reference materials**: the item is not actionable, but is worth keeping - a manual, an account number, an article to come back to. It is sent out of the app, to wherever reference material is kept. This is an external action: the app itself stores no reference material. The branch exists so that such captures have a correct answer, instead of being trashed or parked in someday/maybe forever.
- **Action**: it is done in a single step and needs no project. The item is converted into an action and must be created in valid form - the title starts with a verb and is self-descriptive; context and other optional fields may be filled in.
- **Two minute rule**: if it can be completed in under two minutes, it is done right now and marked as completed in the audit log, without being turned into a "proper" action first.
- **Someone else does it**: the item is not yours to act on. It becomes an action with "assigned to" set, and lands in the "Waiting for view".
- **Project**: more than one action is needed. Requires:
  - a title that is a reference to the outcome, not a description of what to do (validated)
  - a DOD
  - at least one action, which becomes the next action
- **Someday/Maybe**: worth looking at some time, but not now. The item moves to the someday/maybe list, staying raw. The text may be edited to formulate the idea more clearly. Optionally a `snoozeUntil` date can be set, to exclude it from the weekly review requirement until that date.
- **Keep incubating** (only when processing a someday/maybe item): still interesting, still not now. The item stays where it is, with a new `snoozeUntil`.

The process ends when the inbox is empty. The inbox should be emptied regularly, and always as part of the weekly review.

### Weekly review
The ritual that keeps the lists trustworthy. Without it the lists silently go out of date, and a list that is not trusted to be complete is a list that stops being used. Everything else in this document is bookkeeping in service of this process.

The review is guided, and runs in a fixed order:

0. **Gather** - collect everything from the other places captures land in (calendar, messengers, mail, ...) into the inbox, so that the inbox really does hold all open loops.
1. **Get clear** - run Inbox Zero until the inbox is empty. Non-negotiable.
2. **Waiting for** - walk the waiting for view. Anything stale is chased, or gets a due date / `snoozeUntil`.
3. **Projects** - for each active project: is the DOD still what you want, and does it have a next action? This is where stalled projects are fixed. Snoozed projects are skipped.
4. **Next actions** - still valid, still a real physical next action? An action that has been next for weeks without moving usually means the action is phrased wrong, not that you are lazy.
5. **Someday/Maybe** - promote, re-snooze or trash. Snoozed items are skipped.

The review is resumable. It can be interrupted at any point and continued later, and does not have to be finished in one sitting.

Progress is tracked by the per-item `lastReviewedAt`, stamped as each item is walked through. There is no global "last weekly review" record: an item that is not snoozed and whose `lastReviewedAt` is older than a week is simply outstanding, and that is also how the app shows that a review is due.

### Reshaping items
Nothing is ever retyped. When an item turns out to be the wrong shape it is converted, carrying over everything it already has.

**Detach** - an action leaves its project and becomes a standalone action. Used when the action turns out not to belong to the scope of the project after all, and when closing a project that still has open actions. It keeps its title, context, duration, tags, description and dates.

**Promote** - a standalone action becomes a project, because it turns out to need more than one step. Promotion runs the same Project branch as Inbox Zero, and is therefore subject to the same validations, with the fields prefilled from the action:
- the project title is prefilled from the action title, and has to be edited into a reference to the outcome rather than a description of what to do
- tags and description carry over
- a DOD is required
- at least one action is required, which becomes the next action

An action that belongs to a project and should become a project of its own is first detached, then promoted.

## Audit log
Every action performed in the app is audited. An audit entry contains at minimum:
- timestamp
- what happened (item created, edited, completed, trashed, moved between lists, ...)
- the item it refers to

This keeps destructive operations (trashing an inbox item) and instant ones (completing an item under the two minute rule) reviewable and recoverable, without keeping those items in the active lists.

## Recurring items
Recurring actions and projects are **out of scope for this document** and need a separate design pass. The problem is acknowledged rather than solved: nothing in the system currently repeats, including the weekly review itself.

The discussion, the candidate direction and the questions blocking it live in [recurring.md](recurring.md).

## Deliberate omissions
Things consciously left out, recorded here so that they do not come back later as fresh ideas.

- **Priority.** No priority field, no P1 / P2 / P3. It is subjective and unstable - what matters on Monday does not on Thursday - and re-ranking a list feels productive while producing nothing. Real urgency is already carried by the due date, and importance comes out of the weekly review and the areas of responsibility carried by tags.
- **Horizons 3 to 5.** No goals, no vision, no purpose level. Areas of responsibility (horizon 2) are carried by tags, and that is where it stops. The levels above are journal territory, not something this app models.
- **Reference material storage.** The app keeps no reference material of its own. Material that belongs to a specific commitment lives in the description of that action or project; everything else leaves through the "send to reference materials" branch of Inbox Zero and is kept outside the app.
