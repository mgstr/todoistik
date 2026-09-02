# todoistik
Describes todo application, intended for my purposes only. This app is not intended as a generic todo app.

## Design principals
- add only functionality that I will use, don't add anything for future development
- the design of the app should allow to follow principles describe in David Allan book "GTD - Getting Things Done"

## Overview
App should allow manipulation with following entities:
- action: a single non-breakable task, that can be done and have visible output effect
- project: when end result can't be achieved in result of single action it is called a project, it contains a list of actions, and has a "definition of done".

### Actions
Actions is a single (non-breakable into smaller parts) task, that should be done in order to move to the desired goal.
The action should have visible effect. So "thinking about design" is not an action. Use "Write draft a MD with design" instead.
Action has following fields:
- Title: ideally is should start with verb and be fully self-descriptive, avoiding letting something to be in context. So when looking at the action title you don't have to think before you start doing it.
- Context: (optional) defines what physical environment is needed to proceed with action. For example: online (action requires internet connection), home (I need to be home in order to clean my room) etc
- Duration: (optional) the expected duraion of the action. It is not expected to have estimation for all actions, it rather a way to mark actions, that are known to have a long duration, for example - if I need to read long article, I don't want to break this activity, and need to reserve time enought to finish reading at one sitting.
- Description: (optional) any extra meterials needed to be referenced (like URL, link to email, reference to PDF etc) that could be usefull during action.

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

### Tags
Both project and action could have tags, that should be used for items categorization.
Each item could have zero, one or several tags.
