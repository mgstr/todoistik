# todoistik
Describes todo application, intended for my purposes only. This app is not intended as a generic todo app.

## Design principals
- add only functionality that I will use, don't add anything for future development
- the design of the app should allow to follow principles describe in David Allan book "GTD - Getting Things Done"

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
Actions is a single (non-breakable into smaller parts) task, that should be done in order to move to the desired goal.
The action should have visible effect. So "thinking about design" is not an action. Use "Write draft a MD with design" instead.
Action has following fields:
- Title: ideally is should start with verb and be fully self-descriptive, avoiding letting something to be in context. So when looking at the action title you don't have to think before you start doing it.
- Context: (optional) defines what physical environment is needed to proceed with action. For example: online (action requires internet connection), home (I need to be home in order to clean my room) etc
- Duration: (optional) the expected duraion of the action. It is not expected to have estimation for all actions, it rather a way to mark actions, that are known to have a long duration, for example - if I need to read long article, I don't want to break this activity, and need to reserve time enought to finish reading at one sitting.
- Description: (optional) any extra meterials needed to be referenced (like URL, link to email, reference to PDF etc) that could be usefull during action.
- Assigned to: (optional) free text. If not set, it is assumed that you are the one who should do it. If set, the action is waiting on somebody or something else, and appears on the "Waiting for" list.

### Project
Project is a desired result, that requires more than one step to complete.
Project has following fields:
- Title: name that helps to reference the result
- DOD (definition of done): required, since it helps to define what is the expected outcome of the project, and used during review and decision what is the next action
- Actions: a list of actions required to complete a project. In most cases it is enought to have only one next action, to move project forward. But in some cases the listing more steps in advance during planning phase will be helpfull.

A next action is not a property of the project. It is a property of the action - see `becameNextActionAt`. A project can therefore have several next actions at the same time, which is what a parallel project looks like (booking the flight, renewing the passport and asking for time off are all available at once), while a sequential project simply happens to have one.

### Time fields
Both project and action could have following time related fields:
- cration date: (required) when item was created, will be used for calculation age of the item
- due date: (optional) when item should be completed, will be used for indicating that item complition is time sensitive
- lastReviewedAt: (optional) when the item was last reviewed. It drives the weekly review: it shows what has already been walked through and what is still outstanding, which is what makes an interrupted review resumable
- becameNextActionAt: (optional) when the action became a next action. An empty field means the action is not a next action - it is parked, written down in advance during planning. A **real** next action is one where `becameNextActionAt` is set and `completedAt` is still empty. The field doubles as the age of the next action, which is what shows an action that has been next for a long time without moving, and for actions with "assigned to" set it is also the delegation date.
- snoozeUntil: (optional) hides the item from the active views, from the weekly review and from the stalled project check, until that date passes
- completedAt: (optional) when the item was completed. Being set is what makes the item done - there is no separate status flag

`snoozeUntil` is a universal field and means the same thing everywhere it appears - on projects, actions and someday/maybe items: do not bother me about this until that date. It is how an already clarified commitment is shelved for a while without losing its DOD, its actions and the material collected in it.

The single exception is the inbox: an inbox item has no `snoozeUntil`. Snoozing an inbox item is the same thing as moving it to the someday/maybe list. Emptying the inbox is a non-negotiable rule and must not be avoidable by snoozing.

### Completion
An item is resolved explicitly, and only in one of two ways: it is completed, or it is deleted. There are no shortcuts and nothing is resolved implicitly.

- a completed action leaves the next actions list and stops counting as a next action for its project, which may leave the project stalled
- a project can not be completed while it still has actions that are neither completed nor deleted. Every one of them has to be walked through the action lifetime explicitly
- completing a project is therefore always a deliberate act, and the moment the DOD is confirmed to be met. A project is never completed automatically just because it ran out of actions
- there is no separate "done" list. The audit log is the record of what was finished

### Tags
Both project and action could have tags, that should be used for items categorization.
Each item could have zero, one or several tags.

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

## Lists

### Waiting for
A first class list, sitting alongside next actions. It holds every next action with a non-empty "assigned to" field: commitments that are still tracked, but where the ball is not in your court.
This covers people (delegated to somebody) as well as things (an order placed, a form submitted, a PR awaiting CI).

Rules:
- a waiting for action is still a next action, so a project whose only next action is a waiting for one is **not** stalled
- it is excluded from the "what can I do right now" view, since it cannot be acted upon
- its age comes from `becameNextActionAt`, which for these items is the delegation date
- there is no automatic chasing. If a waiting for item has to be chased at a specific moment, the existing due date / snooze date are used
- the list is reviewed during the weekly review

### Someday/Maybe
A first class list, holding raw ideas that are worth looking at some time, but that you are not ready to work on now.

A someday/maybe item is not a project and not an action - it is the same raw, unclarified capture as an inbox item. Clarifying it would mean defining an outcome and a next action for something you have deliberately decided not to commit to, which is wasted work and is exactly the friction that makes a someday list go unused. It therefore stays raw until you decide to move on it.

Rules:
- items arrive here from the inbox, as one of the outcomes of Inbox Zero
- the creation date shows the age of the idea
- `snoozeUntil` (optional) hides the item from the weekly review requirement until that date, so that a long someday list stays reviewable
- when you decide to move on an item, it is processed exactly the same way as an inbox item (see Inbox Zero)
- the list is reviewed during the weekly review, skipping items that are still snoozed

## Processes

### Inbox Zero
A dedicated mode that processes captured items one at a time, oldest first. It is used to empty the inbox, and also to process a someday/maybe item once you decide to move on it.
While the process runs everything else is hidden from view - only the current item is shown.
For each item the only question asked is: what is it? The answer is one of:

- **Trash**: the item is deleted. Recorded in the audit log.
- **Action**: it is done in a single step and needs no project. The item is converted into an action and must be created in valid form - the title starts with a verb and is self-descriptive; context and other optional fields may be filled in.
- **Two minute rule**: if it can be completed in under two minutes, it is done right now and marked as completed in the audit log, without being turned into a "proper" action first.
- **Someone else does it**: the item is not yours to act on. It becomes an action with "assigned to" set, and lands on the "Waiting for" list.
- **Project**: more than one action is needed. Requires:
  - a title that is a reference to the outcome, not a description of what to do (validated)
  - a DOD
  - at least one action, which becomes the next action
- **Someday/Maybe**: worth looking at some time, but not now. The item moves to the someday/maybe list, staying raw. The text may be edited to formulate the idea more clearly. Optionally a `snoozeUntil` date can be set, to hide it from the weekly review requirement until that date.
- **Keep incubating** (only when processing a someday/maybe item): still interesting, still not now. The item stays where it is, with a new `snoozeUntil`.

The process ends when the inbox is empty. The inbox should be emptied regularly, and always as part of the weekly review.

### Weekly review
The ritual that keeps the lists trustworthy. Without it the lists silently go out of date, and a list that is not trusted to be complete is a list that stops being used. Everything else in this document is bookkeeping in service of this process.

The review is guided, and runs in a fixed order:

0. **Gather** - collect everything from the other places captures land in (calendar, messengers, mail, ...) into the inbox, so that the inbox really does hold all open loops.
1. **Get clear** - run Inbox Zero until the inbox is empty. Non-negotiable.
2. **Waiting for** - walk the waiting for list. Anything stale is chased, or gets a due date / `snoozeUntil`.
3. **Projects** - for each active project: is the DOD still what you want, and does it have a next action? This is where stalled projects are fixed. Snoozed projects are skipped.
4. **Next actions** - still valid, still a real physical next action? An action that has been next for weeks without moving usually means the action is phrased wrong, not that you are lazy.
5. **Someday/Maybe** - promote, re-snooze or trash. Snoozed items are skipped.

The review is resumable. It can be interrupted at any point and continued later, and does not have to be finished in one sitting.

Progress is tracked by the per-item `lastReviewedAt`, stamped as each item is walked through. There is no global "last weekly review" record: an item that is not snoozed and whose `lastReviewedAt` is older than a week is simply outstanding, and that is also how the app shows that a review is due.

## Audit log
Every action performed in the app is audited. An audit entry contains at minimum:
- timestamp
- what happened (item created, edited, completed, trashed, moved between lists, ...)
- the item it refers to

This keeps destructive operations (trashing an inbox item) and instant ones (completing an item under the two minute rule) reviewable and recoverable, without keeping those items in the active lists.
