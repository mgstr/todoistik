# todoistik
Describes todo application, intended for my purposes only. This app is not intended as a generic todo app.

## Design principals
- add only functionality that I will use, don't add anything for future development
- the design of the app should allow to follow principles describe in David Allan book "GTD - Getting Things Done"

## Overview
App should allow manipulation with following entities:
- inbox item: a raw, unprocessed capture, that has not been decided about yet
- action: a single non-breakable task, that can be done and have visible output effect
- project: when end result can't be achieved in result of single action it is called a project, it contains a list of actions, and has a "definition of done".

### Inbox item
Raw, unprocessed capture. Deliberately near-schemaless - the point is zero friction at capture time.
Capture must never require a decision: deciding what an item actually means is a separate deliberate act (see "Inbox Zero"), performed later.
An inbox item is never something to be done, it is something to be decided about.
Inbox item has following fields:
- Text: (required) free-form, whatever was captured
- Creation date: (required)

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
- Next action: one action from actions list that will move project forward.

### Time fields
Both project and action could have following time related fields:
- cration date: (required) when item was created, will be used for calculation age of the item
- due date: (optional) when item should be completed, will be used for indicating that item complition is time sensitive
- review date: (optional) time of the last review, allows tracking of items that require attention during weekly review
- becameANextActionDate: (optional) when the action became a next action. Used to spot actions that have been next for a long time without moving. For actions with "assigned to" set it doubles as the delegation date, so the age of a waiting for item is visible directly.

### Tags
Both project and action could have tags, that should be used for items categorization.
Each item could have zero, one or several tags.

## Lists

### Waiting for
A first class list, sitting alongside next actions. It holds every next action with a non-empty "assigned to" field: commitments that are still tracked, but where the ball is not in your court.
This covers people (delegated to somebody) as well as things (an order placed, a form submitted, a PR awaiting CI).

Rules:
- a waiting for action is still a next action, so a project whose only next action is a waiting for one is **not** stalled
- it is excluded from the "what can I do right now" view, since it cannot be acted upon
- its age comes from `becameANextActionDate`, which for these items is the delegation date
- there is no automatic chasing. If a waiting for item has to be chased at a specific moment, the existing due date / snooze date are used
- the list is reviewed during the weekly review

## Processes

### Inbox Zero
A dedicated mode that processes inbox items one at a time, oldest first.
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
- **Someday/Maybe**: worth looking at some time, but not now. The title may be edited to formulate it more clearly. Optionally a `snoozeUntil` date can be set, to hide the item from the weekly review requirement until that date.

The process ends when the inbox is empty. The inbox should be emptied regularly, and always as part of the weekly review.

Open: the exact behaviour of Someday/Maybe and `snoozeUntil` is to be defined later.

## Audit log
Every action performed in the app is audited. An audit entry contains at minimum:
- timestamp
- what happened (item created, edited, completed, trashed, moved between lists, ...)
- the item it refers to

This keeps destructive operations (trashing an inbox item) and instant ones (completing an item under the two minute rule) reviewable and recoverable, without keeping those items in the active lists.
