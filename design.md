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
- Text: (required) free-form, whatever was captured - one line or several, of which the first is the item as the inbox shows it
- Creation date: (required)

An inbox item can not be snoozed - see "Time fields".

**The text may run to more than one line, and the first line is the item.** Every list of inbox items shows that line and nothing else, because the inbox is read as a list and a list is read by its lines. The rest is carried with the item, unshown, until the item is processed - which is the moment it is being looked at deliberately, and the only moment the rest is worth seeing.

What the rest is for is a capture that arrives with a body already attached - a reminder's note, a link to the mail a loop came in on, anything the program capturing it had in hand and would otherwise have to mash into one line (see implementation.md, "move: a list into the inbox"). It is not a second field with a name and a meaning: nothing says what it holds, it is whatever was captured exactly as the first line is, and it is decided about along with the rest of the item.

**Nothing marks a listed item as carrying more.** Knowing before you look buys nothing, for the same reason a completion request is not marked either: the inbox is emptied whole and every item is opened, so a mark could only say that you are about to see something (see "Completion requests").

**The text may be written with notation in it** - `Book the tyre change @garage #car #short` - and it stays text: the item has no context, no tags and no size, and nothing reads the line until it is processed. This is not a second way to create things, it is the shortest way to write down what you already knew at the moment of capture. Stopping to open a form is exactly what capture must never require, and a thought that arrives with its area and its size attached loses both if the only place to put them is a screen you are not on. What the notation is worth is claimed in Inbox Zero, where the branches that create something read its first line and start their form from it (see "Inbox Zero").

#### External capture
Adding items to the inbox must be possible from outside the app.
The app exposes a simple consuming API for this - a single endpoint accepting a text payload - so that captures can arrive from scripts, CLI, a mobile share sheet, email or any other tool without opening the app.

#### Duplicate captures
A capture whose text is exactly identical to an item **already sitting in the inbox** is dropped. This holds for every way in: typed by hand, the capture API, a schedule.

- the comparison is against the open inbox only, never against history. Checking everything ever captured would mean a repeating chore arrived once and never again, because the first one was processed weeks ago
- matching is exact and not fuzzy, for the reason the name filter is exact: it should always be obvious why something matched
- **the whole text is compared, every line of it, not the first.** Two captures that lead with the same line and carry different bodies are two different things - two mails sharing a subject, two reminders whose notes differ - and collapsing them would throw one away on the strength of a line that was never meant to identify anything. The first line is what the inbox shows, not what the item is (see "Inbox item")
- nothing is lost when a duplicate is dropped, since the loop it describes is already in the inbox waiting to be decided about. It is not audited, for the same reason
- the capture API says so in its response rather than simply succeeding, so that a script cannot mistake "silently vanished" for "accepted" - see "External capture"

This is what makes capture **idempotent**, which is worth having on its own: a script retrying after a timeout, a share sheet tapped twice, and a schedule replaying the occurrences missed during three weeks away all stop being able to flood the inbox. Emptying the inbox is the one rule with no exceptions, so anything able to pile up in it without limit is a threat to that rule.

Collapsing is lossy wherever instances genuinely count - two months away would otherwise mean "Pay the rent" firing twice and being seen once. That is what a schedule's suffix is for: it makes each occurrence produce a different string, so nothing collapses that should not. The default is to collapse, and saying otherwise is one field - see "Schedule".

#### Completion requests
A capture may be a report from another program instead of a thought of your own. Something that mirrors items out of the app - a Reminders list carried onto a phone, say (see implementation.md, "Reminders, both ways") - can find that an item looks done over there, and it says so by capturing a line in one specific shape: which item, when it was finished, and what it was called.

- **it is a capture, not a completion.** The program reporting it completes nothing: it puts a line in the inbox and a person answers it. That is what keeps capture the only way in (see "External capture") and keeps one answer to what is done - a program allowed to complete things would be recording commitments met that nobody confirmed, and the audit log would hold them as if someone had
- **it does not make the phone a place where work is managed.** What comes back is a capture, which is the one thing the phone was ever for (see "Design principles"). What a phone away from the desk can honestly say is "this looks done", and it says so through the one way in, exactly as a share sheet does
- **it carries the item's name as well as its identity.** The identity is what the answer acts on; the name is what makes the request answerable at all, because the item may be gone by the time the line is read, and a request that can then only say that something you no longer have was finished has nothing to say to you
- **it is text until it is processed**, like every other capture. Nothing reads the line to dress it up in the inbox: it is shown exactly as it arrived, the same as a thought captured in notation (see "Inbox item"). Knowing which lines are requests before looking at them buys nothing - the inbox is emptied whole either way, and a second kind of row in it would be a second thing to learn to read
- **the same request arriving twice while the first is unanswered is dropped**, by the ordinary duplicate rule (see "Duplicate captures"). That is what makes reporting one safe to repeat, which matters because the program reporting it cannot know whether the last one got through
- what happens when one is read is in "Inbox Zero"

### Schedule
A piece of text and a rule for when to put it in the inbox. It exists so that the things which have to come back - a chore that repeats, or an obligation that has to be looked at weeks before it falls due - are not held in your head in the meantime.

A schedule is not a commitment and never becomes one by itself. What it produces is a **capture**: raw text arriving in the inbox, decided about by hand in Inbox Zero like anything else. Nothing reaches "Projects", "Tasks" or "Next actions" without having been accepted there. This is also what covers recurring actions and projects.

Fields:
- Text: (required) free-form, what will land in the inbox. It is a capture, so it stays raw - not a title, not an action, not a project
- When: (required) either a single **date**, or a **cron expression** at day granularity - day of month, month, day of week, and an optional fourth field, the year. No times: nothing in this app has an hour, so neither does this. The first three are the calendar fields of standard cron, with standard syntax and semantics - `*`, lists, ranges, steps, and the standard rule that when both day-of-month and day-of-week are restricted, either one matching fires. Standard where it can be, so that the behaviour of an expression can be looked up rather than guessed. The expression is validated when the schedule is saved, and the "Scheduler" shows it in readable form - the way it was written, so that a run reads as a run: `9-23 9 *` comes back as "the 9th-23rd of September", not as a fortnight of ordinals
- **the year field is deliberately not standard cron.** Standard cron has no year, because it describes a machine's recurring work and machines are not told when to stop. A person's is: "chase this every day until the 23rd of September" is a real thing to want, and without a year the only two shapes on offer were a single day and forever. `9-23 9 * 2026` is the fortnight; `9-23 9 *` is that fortnight every year, which is what leaving the field off still means - so every rule written before the field existed goes on meaning exactly what it meant. The syntax of the field is the same syntax the other three use, and the years it may name run 2000-2099: a schedule is something you will actually be reminded of, so outside that range a four-digit number is a typo and being told so is worth more than being able to schedule 2317
- Suffix: (optional, empty by default) appended to the text when the capture is made. `YYYY`, `MM` and `DD` are replaced with the date of the occurrence being fired; everything else is literal, including any leading space. An empty suffix makes every occurrence produce the same string, which is what collapses a repeated chore to a single inbox item; ` YYYY-MM` on the rent makes each month produce its own
- Creation date: (required)
- lastFiredAt: (optional) when it last put something in the inbox, empty until it first does. It is what shows at review time that a schedule is actually working
- lastReviewedAt: (required) see "Time fields"

Rules:
- **it fires lazily**, on the first use of the app on a day whose occurrence has passed. This is the rule `#today` clearing already uses, for the same reason: a scheduler that works only while a process happens to be running is one that cannot be trusted, and a firing that did not happen is invisible
- **a schedule with nothing left to fire deletes itself.** A single date is the ordinary case - it fires once and goes - but it is not a special case: a rule that named its years does the same the day after its last occurrence, and so does one that can never fire at all. Anything left in the list forever would turn the "Scheduler" into a graveyard of things that already happened. The deletion is audited like any other, so what fired and when is still answerable from the "Audit log"
- **a cron schedule with no year persists** and keeps firing
- **every missed occurrence fires**, oldest first. After an absence a schedule does not have to be careful about how many went by: without a suffix the captures are identical and collapse in the inbox to one item, and with one they stay distinct, because a suffix is how you said the instances count - see "Duplicate captures"
- **an occurrence only counts if the rule was in force when it fell.** Occurrences are counted from the creation date, so a schedule created today does not back-fire for dates before it existed, and editing "When" restarts the count from the edit: past occurrences of a rule that was not yet in place were never missed
- **a rule the app cannot read is refused, and the form comes back saying why.** Nothing is written, and a schedule being edited keeps the rule it had. "When" is the one field in this app you can get wrong by typing something entirely reasonable - a date written in the wrong order, or three cron fields that mean something other than what they look like - so a refusal that said nothing would read as a Create button that does not work, which is exactly how this was found
- it is edited and deleted like anything else - see "Editing items"

The three parts compose deliberately, and each stays dumb on its own. The schedule fires per occurrence and knows nothing else. The inbox drops a capture identical to one already waiting. The suffix is the one place where you declare that instances are distinct, and it is visible in the text that arrives, so two rent items say which month each is for. Nothing anywhere tracks instances.

The cost lands where you put it: a suffix on a daily schedule is how you ask to be told about every single day you were away. Use one only where the instances genuinely count.

Instances are not linked to each other. A schedule knows when it last fired and nothing about what became of what it produced, so "when did I last change the tyres" is answered by searching the "Archive" for the action, not by asking the schedule. That is the right place for it: what you did is a completed commitment, and the schedule only ever made the reminder.

A schedule can not express "again three days after I last did it". Cron describes the calendar and not your last completion, and that case is already covered without it: complete the action, and put a `snoozeUntil` on the next one.

### Someday/maybe item
A raw idea that is worth looking at some time, but that you are not ready to work on now.

A someday/maybe item is not a project and not an action - it is the same unclarified capture as an inbox item, carrying the area of responsibility it belongs to and nothing more. Clarifying it would mean defining an outcome and a next action for something you have deliberately decided not to commit to, which is wasted work and is exactly the friction that makes a someday/maybe go unused. It therefore stays unclarified until you decide to move on it, and a tag does not clarify anything: it says what an idea is *about*, never what you have undertaken to do about it. What it buys is the only thing a list of parked ideas is ever read for - which part of your life this pile is quietly filling up with (see "Tags").

Fields:
- Text: (required) free-form, the idea as captured, editable
- Tags: (optional) the areas of responsibility the idea belongs to
- Creation date: (required) it shows the age of the idea
- lastReviewedAt - see "Time fields"

Nothing else an action carries is here - no context, no size, no deadline, nobody it is waiting on. Every one of those describes doing something, and this is precisely the thing you have decided not to do yet; the item takes them on when it becomes an action, and not before.

**There is no `snoozeUntil` either**, and that is the one omission worth arguing for, since projects and actions both have one. A snooze says "this is already a commitment, do not bother me about it until then" - and a someday/maybe item is the opposite of a commitment, so there is nothing here to be shelved. What a date on one would actually do is hide an idea from the walk that exists to look at ideas, on a list nobody is bothered by in the first place: this view is opened deliberately and never nags. The date also asked a question the item cannot answer honestly - "when does this become worth looking at" is exactly what you do not know about something you have not committed to. So the list is walked whole, on its own cadence, and an idea that is not worth a second of that walk is trashed instead of postponed.

Rules:
- items become someday/maybe items from the inbox, as one of the outcomes of Inbox Zero, and the wording and the tags are settled there, as part of that answer (see Inbox Zero)
- both stay editable afterwards, on the item's own page. Rewording an idea and moving it to the area it turns out to belong to are things noticed while reading the list it sits in, and neither is a commitment being made
- **there is one way out, and it is the inbox.** When an idea stops being something for later - because you have decided to move on it, or decided it is worthless - it goes back to the inbox and is answered there, by the same branches every other capture is answered by, trash included (see "Reshaping items"). Deciding about it *here* would be a second processing screen, reached from a list of things explicitly not being decided about, offering answers that only make sense for one kind of item. One place where decisions are made is worth more than a shortcut
- someday/maybe items are reviewed on their own, longer cadence - a month by default rather than the week everything else gets (see "Weekly review")

### Action
An action is a single (non-breakable into smaller parts) task, that should be done in order to move to the desired goal.
The action should have visible effect. So "thinking about design" is not an action. Use "Write draft a MD with design" instead.
Action has following fields:
- Title: ideally it should start with a verb and be fully self-descriptive, avoiding letting something to be in context. So when looking at the action title you don't have to think before you start doing it.
- Context: (optional) the physical prerequisite for doing the action, at most one - see "Contexts"
- Duration: (optional) how big the action is, as one of three sizes: `short`, `medium`, `long`. Sizes and not minutes, and deliberately with no unit named anywhere: a bucket labelled with a number is still asking how long something takes, which is a question with no honest answer and one you have to stop and work out - where "is this a small thing or a big thing" is something you already know when you look at it. Three and not more for the same reason: the moment two buckets sit next to each other on the same scale, choosing between them is a decision, and this field is only worth having if it costs nothing to fill in. `long` also carries the original meaning: do not start this unless there is enough time to finish it in one sitting, like reading a long article.
- Needs focus: (optional) marks an action that can not be done while tired. Deliberately a single flag rather than a low / normal / high scale - having to grade the energy of every action puts pressure on capture, which is exactly the friction worth avoiding.
- Description: (optional) any extra materials needed to be referenced (like URL, link to email, reference to PDF etc) that could be useful during the action. A URL written here is followed from the screen, not copied out of it - see "Following a link".
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
- snoozeUntil: (optional, projects and actions) marks the item as not yet ready to be worked on, until that date passes
- completedAt: (optional, projects and actions) when the item was completed. Being set is what makes the item done - there is no separate status flag

Every one of these is a date and never a time of day, and they are all read against a single clock: the timezone the app is configured with. "Today" therefore means the same day everywhere it is asked - the `#today` clearing, lazy schedule firing, "due today" and overdue all share the one boundary, and a capture sent from a phone in another timezone lands on the app's day, not the phone's. Two clocks would mean two opinions about whether something is overdue, which is the kind of disagreement that makes a view stop being trusted.

A due date and a `snoozeUntil` may be **written as a word** rather than as a date: `tomorrow`, a day name, or a number of days. Every one of them is counted off against that same clock and stored as the date it lands on - the app knows what day it is, and making you work out that Friday is the 18th is exactly the friction that ends with the date not being written at all. The word is resolved on the way in and never kept, because a field that still said "friday" a week later would be a second opinion about when this is, and the whole point of one clock is that there is only ever one.

- **a day name means the next day of that name, never the day being stood on.** "Snooze until Monday" typed on a Monday is about the week ahead; resolving it to today would answer a question nobody asked and quietly leave the item awake.
- **`today` is a real answer to a deadline and no answer at all to a snooze.** A snooze is a claim that this is not worth looking at yet, so it names a day that is still ahead; `snooze:today` is refused by name rather than stored as a date that has already arrived and means nothing.
- **an explicit date in the past is not refused**, on either field. That is a claim that went stale rather than one that was never meant, and catching a stale claim is what the weekly review is for - see "Weekly review".

`snoozeUntil` means the same thing everywhere it appears - on projects and on actions: do not bother me about this until that date. It is how an already clarified commitment is shelved for a while without losing its DOD, its actions and the material collected in it. It belongs to commitments only: a someday/maybe item has none, for the reason given in "Someday/maybe item", and an inbox item has none, for the reason given below.

`snoozeUntil` is also what covers deferral - "there is no point looking at this before Tuesday" - so there is no separate defer date.

A snoozed item is **not hidden** where what it belongs to is read. A snoozed action stays in its project's action list, shown differently to say that it is not yet ready to be worked on: that list is the project's plan, and an action missing from it would make the plan look like something it is not - a project whose only action had become invisible would look stalled while the app insists it is not. The same holds for a snoozed project in the projects list.

What a snooze actually does:
- a snoozed **project** is exempt from the stalled project check
- a snoozed **action** still counts as a next action of its project, so deferring a single action does not make the whole project look stalled. The stalled project check knows about snoozed actions. This is the same exemption a waiting for action gets, and for the same reason
- a snoozed **action** is left out of the "Next actions" view, and out of that view only. That view answers "what do I do next", and a snoozed action is one that cannot be done yet - it is not an answer to that question, so it does not belong on that list. It is deferred rather than lost: it is still in its project's action list, in Tasks and in the Calendar, and the weekly review still walks it, which is where a snooze date that turned out wrong is caught

What a snooze does **not** do is exempt the item from the weekly review. The snooze date is a claim about the future, and claims go stale like everything else: a wrong one either wakes the item at a moment that no longer means anything or keeps it asleep past the moment that did. The review is the only place that can be noticed, so a snoozed item is walked like any other - and checking its date is part of what walking it means.

An inbox item has no `snoozeUntil` at all: snoozing one is the same thing as making it a someday/maybe item. Emptying the inbox is a non-negotiable rule and must not be avoidable by snoozing.

### Contexts
A context is a physical prerequisite for doing an action: something that has to be true before the action is possible at all. If the action could be done without it, it is not a context.

Contexts apply to **actions only**. A project is not something you do, so it has no context.

An action has **at most one** context. Notation is `@name`: `@home`, `@garage`, `@online` (an internet connection is needed, on any device), `@computer` (a real computer is needed, a phone will not do).

Context names come from a remembered list - never typed fresh, otherwise `@home` and `@Home` drift into two contexts. Writing `@home` on an action's meta line sets the context only if `home` is on that list; if it is not, the line is refused rather than saved with the name quietly ignored (see "Writing an action"). Adding a new name is therefore a deliberate act, and it should be offered where it is wanted rather than as a trip to another screen: when what was written matches nothing, the app offers to create it there, behind an explicit confirm - deliberate enough to stop drift, cheap enough not to fight capture. The list is editable, so that a context no longer used can be removed; one still carried by actions can not be, since removing it would be editing those actions behind their back.

#### Parameters
A context may carry a parameter: `@person(Andres)`, `@grocery(Selver)`. This keeps the context namespace small and scannable, which is the only reason contexts are useful at all - putting every person and every shop chain at the top level would destroy that.

- the parameterised form is **narrower** than the bare one. An action written `@grocery` can be done in Selver, one written `@grocery(Prisma)` can not. It runs the same way through the filter, where `@grocery(Selver)` finds the Selver errands and the shop-agnostic ones, and the bare `@grocery` finds every errand under the type - see "Filtering by context"
- the bare form is not always meaningful. Bare `@grocery` is useful ("buy milk, any shop"), bare `@person` is not. Some context types will in practice always carry a parameter, and that is fine
- parameter values are picked from a remembered list per context type, never typed fresh, otherwise `@person(Andres)`, `@person(andres)` and `@person(Andres P.)` become three different contexts
- that set of values has to be editable, so that values that are no longer used can be removed

#### Filtering
The "Next actions" view filters by context, one at a time - see "Filtering by context" for the control and what it hides. An action carries a single context, so "where am I" has a single answer, and the question the filter asks is "which actions can be done here" - or, when the line names a type without a parameter, "which actions belong to this kind of place".

The filter itself takes several and combines them with **OR** - at home, with a computer and an internet connection is `@home OR @computer OR @online` - but that is now only reachable through the read API, where the caller states its own situation and may well be in more than one of them at once. The screen offers one, because a row of exclusive answers is read at a glance and a set of checkboxes has to be interpreted. The cost of the single context on an action is that one needing two prerequisites at once has to name the scarcer one; this is accepted.

### Tags
A tag is a label used to filter and categorise. Unlike a context it is not a precondition - it says nothing about whether an item can be done, only about what it is about.

Notation is `#name`: `#car`, `#finance`, `#hobby`, `#programming`.

- tags apply to **projects, actions and someday/maybe items**. The first two are commitments and the third is not, and it carries them anyway: an area you have parked six ideas about and committed to none of is exactly the kind of thing the review is there to notice, and a someday list that can only be read whole is a list that ends up not being read
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
- **the moment recorded is the moment the work was finished**, which is all but always the moment of the answer and is not the same thing. Accepting a completion request stamps the time the item was ticked off elsewhere rather than now (see "Completion requests", "Inbox Zero"): the Archive and the completed filters exist to say when work happened, so answering Tuesday's tick on Friday has to leave them saying Tuesday. This is the only path where the two moments come apart, which is why it is the only one that has to say which of them it means

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

### Following a link
An item's text often holds a link. Mail capture writes one into every item it makes (see "External capture"), a reminder's note can arrive with one, and an action's description is named for exactly this - "any extra materials needed to be referenced (like URL, link to email...)", see "Action". Wherever that text is shown, its links are **followed by pressing them**, not by selecting, copying, switching window and pasting. Four moves is enough to stop the link being followed at all, which makes carrying it with the item pointless - and the moment it is wanted is the moment the item is being decided about or done, which is precisely when leaving the app to fetch it costs most.

- **only what is written as a link is one.** `https://`, and everything up to the next space. Never a bare `example.com`, never an address like `marju@gmail.com`. This is the rule the meta line already follows for names (see "Writing an action"): the app does not invent notation out of prose. A rule that guessed would guess wrong on the first sentence that merely mentions a domain, and prose that turned itself into a control is worse than prose that stayed prose
- **`http://` is not a link.** This is a judgement and not an omission. A plain-text address today is either a mistake or bait, and the app has no way to tell which - so making it pressable would be the app vouching for something it knows nothing about, at the one moment its guard is down: a link arrives inside an item captured out of a mail nobody has read yet. Left as words it is still readable, still selectable, still followable by anyone who has decided to - which is exactly the amount of work something worth looking at twice should cost
- **nothing is stored and nothing is rewritten.** The text is exactly what was captured or typed, character for character. A link is a way of *reading* that text, not a field parsed out of it - so there is no second copy to keep in step, editing the words edits the link, and deleting them deletes it
- **a link opens in a place of its own** and never in the middle of the app. An item is followed while something else is in progress - an inbox being emptied, a review being walked - and a reference that replaced the screen you were on would cost the thing you were doing
- **a link out of the app never looks like a link inside it.** Nearly every link on a screen moves between views, and those two acts have nothing in common: one keeps you here, the other hands you to a browser tab and a mail client. Which one a word is has to be visible before it is pressed, and without reading the address first
- **a link is offered even where the text is only ever written in.** An action's description is never shown as prose anywhere - it is a box, and a box is characters and nothing else - and it is also the field most likely to be holding the link. So the links a field holds are listed beside the field. They are the links in the *saved* text: one just pasted is being written rather than followed, and it goes live when the item is saved
- **the inbox list still shows the first line and no more.** A mail capture keeps its link in the body, which no list has ever shown (see "Inbox item"), and that does not change here. The link is live on the screen the item is read in full on, which is the processing screen - one keystroke away, and the screen where the decision is being made anyway
- **the keyboard follows one too, with `ctrl-o`.** Pressing a link is a thing done with a pointer, and most of the screens an item is read on are walked with `j` and `k` - so the key follows the link belonging to whatever the cursor is on: the row selected on a list, or the one item a screen is about. Ctrl rather than a bare letter, because `o` already opens the selected row, and because the moment the link is wanted is often the moment the item is open with a box being typed in, which is exactly what a modifier is spent on (see implementation.md, "The keys")
- **the key reaches what the item holds, not only what the screen drew.** It is on the screens showing least of an item that its link is wanted most: a next action is a title and some badges, the doing screen is a title alone, and a mail capture keeps its link in a body no list has ever shown. A key that could follow only what was already drawn would be missing from precisely the screens the link is carried for. Nothing is shown that was not shown before - the lists still show what they showed, and the rule above still holds - the key simply knows which item it is standing on
- **and `links.reach` narrows it back to the screen.** Both readings are defensible: a key that opens something you cannot see first is a key you have to trust, and trusting a link is the one thing this whole section says not to do without looking. `shown` is that answer - `ctrl-o` follows only what is on the screen, and on a next-actions row or the doing screen it then does nothing at all. The default is `any`, because a key that is missing on the two screens it exists for is not worth the key
- **more than one link is a question, and it is asked.** One link opens straight away, which is nearly always what an item holds. Where there are several they are put up as a list, lettered in the order they read, and the next key opens one - nothing is opened until it has been chosen, because a key that opened four tabs would not be a key you press to find out what an item is carrying. The letters go on a list of their own rather than beside the links themselves, since under the default reach most of them are not on the screen to be marked
- **the key is not offered where the item holds no link**, the way no key here is ever offered without working (see implementation.md, "The keys"). An item with nothing to follow is most items, so a `ctrl-o` that was always advertised would be advertising nothing most of the time

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
| `snooze:2026-09-20` | not ready to be worked on until then |
| `due:tomorrow` `snooze:friday` `due:3days` | either date, counted off from today |

The point is that the form asks for nothing that has to be decided. A row of controls asks every question of every action, and most of them have no answer worth giving - a line asks one question, and you write only what is true. It is also the same gesture capture already is, so the two ends of the process are typed the same way.

- **the meta line and the description are two fields because they are read for two different reasons.** The description is read to remember what an action is about; the meta line is read to see what the app thinks it is. They shared one box until it became clear that neither could be looked at without the other in the way: the notation had to be found again at the bottom of the prose on every edit, and the prose could not be rewritten without editing notation by accident.
- **a name is notation only if it is already known.** `@name` and `#name` are read as metadata when the name is on the remembered list (see "Contexts" and "Tags"). This is the same rule that already said names are never typed fresh - without it a written field is the widest possible door for `@home` and `@Home` to walk through separately.
- **the meta line and the filter line are the same box.** They are the same notation asking two different questions - what is this, and which of these do I want - so they behave the same way: the app completes the names it knows - and the words a date takes - as they are typed, marks the ones it does not where they are written, and asks about each of those before anything is saved or applied (see "The filter line"). What each box accepts differs, because an action carries things a project does not and a filter can ask for things neither carries; being told which is which is the box's job, not something to remember.
- **the meta line refuses what it cannot read.** Whatever is left on it once the notation has been taken out is reported and nothing is saved, whether that is an unknown name, a typo or a sentence. While the two shared a box this question did not arise - anything unknown stayed prose, which is what kept `marju@gmail.com` from becoming a context and `invoice #12345` from becoming a tag. On a line that holds nothing but names there is no prose left for it to stay as, so the choice is between saying so and swallowing it silently, and being told that `#hobbies` is not `#hobby` is worth more than a tag that quietly did not apply.
- **the description is prose, and nothing is read out of it.** `@home` written there is a word like any other. Nothing an action carries can be changed by editing it, which is what makes it safe to write in freely - and it is why it, not the meta line, is where a sentence that happens to mention a context belongs.
- **the fields are still fields.** What is written on the meta line is read into them when the action is saved, and written back out of them when it is opened. Every view, filter and sort works on the fields exactly as before - nothing queries text. This is also why the app can still change them on its own: picking for today, a detach stamping a parked action, a delegation restamping the clock all move a field, and the box simply shows the new truth next time it is opened.
- **a contradiction is refused, never guessed at.** Two contexts, two sizes, `@waitingFor` with nobody named, `#parked` on a standalone action - each is reported and nothing is saved. Guessing which one was meant would be the app deciding something the person is in the middle of deciding.
- **a date may be written as a word, and reads back as a date.** `tomorrow`, a day name and `3days` are the dates you actually pick when writing an action, and they are the ones a calendar is needed for; the word is resolved when the line is saved, so the line you open tomorrow says the day rather than the word that chose it - see "Time fields".
- **the notation is written back in a fixed order.** Opening and saving an action twice cannot shuffle or lose anything, which is what makes the line safe to keep editing.

### Writing a project
A project is written in three fields: its **title**, its **definition of done**, and a **meta** line - plus the actions under it, which are written as actions and not as part of the project (see below). The meta line is the same notation read the same way, narrowed to what a project has:

| written | means |
|---|---|
| `#name` | a tag |
| `snooze:2026-09-20` | not ready to be worked on until then |
| `snooze:friday` | the same, counted off from today |

- **a project's line is short because a project carries little.** It has no context, no size, no deadline and nobody it is waiting on: those describe doing something, and a project is not something you do - it is the outcome that a list of actions is aimed at. Writing one of them here is **refused by name** rather than dropped, because a size written on a project is a mistake about where the thing belongs, and a silent drop would leave that mistake believed.
- **it is the same line in both places it is written.** The project screen of Inbox Zero and a project's own page use it identically, so a project's tags are not written one way while it is being made and another way afterwards.
- **the actions are written as actions.** Adding one opens the same form an action is written in anywhere else, with the project already answered, and each carries its own meta line - which is how a delegated action is delegated here, with `@waitingFor(who)`, exactly as it would be anywhere else.
- **every action written while the project is being made becomes a next action of it**, so `#parked` is refused there: parking means "written down in advance, not yet next", and a project whose actions were all parked would be created already stalled, which is a contradiction (see "Inbox Zero"). Once the project exists, parking is one keystroke away.


## Views
Every list the app shows is a view: a query over the items. No **item** is ever stored in a view, and a view can not be created, renamed or deleted - which is what makes the ones below permanent fixtures, and what makes each of them free.

Opening a single item to work on it is not a view, and not an exception to this either: an item shown in full is the item, holding exactly what it held in the list, and nothing is kept there - see "Editing items". The guided processes are the same, showing one item at a time - see "Processes".

A due date is shown in every view the item carrying it appears in, and an overdue one is marked loudly, the same way a stalled project is. A deadline is the one thing that can not wait for the right screen to be opened, which is why it is not left to the "Calendar" alone - that view is where deadlines are ordered and asked about, not where they are learned of.

Filters are the one piece of state a view remembers, and they are not items: they decide which items a query returns, and never what exists. Nothing is created, moved or lost by filtering, and turning every filter off gives the complete list back. A filtered view says so loudly - it shows the line it is filtered by and how many of its items are on the screen, with taking it all off one keystroke away - because a view quietly showing part of itself is exactly how a view stops being trusted. See "The filter line".

**Ages are hidden until they are asked for.** Every list can say how old the things on it are - how long an action has been next, how long an idea has sat, when an entry was written - and that answer decides something two or three times a week and is noise on every other read. So the app carries one flag for it, and one for the whole app rather than one per view: ages off, which is how it starts, or ages on. It is not a filter and must not be read as one. Filtering changes which items the view returns and is therefore something the view has to confess to; this leaves a field off rows that are all still there, and hides nothing that could be acted on. Where the flag stands is written in the corner of the key bar, because a screen that can be either way has to say which way it is, and it is remembered the way a filter set is - the answer to "show me the dates" should not have to be given again after every jump between views.

The flag covers an item opened in full as well as the lists, because an item shown in full is the item and not a different place (see above): an action's `created` and `next for`, an idea's `captured`, a schedule's `created` and `last fired`. A flag that says "ages hidden" while two of them are still on the screen you are looking at is not one flag, it is a flag and a list of exceptions. What it does not cover is anything that is not an age: a completed marker, a schedule that has never fired, and the date a schedule fires next are facts about the item rather than answers to "how old", and a date in the future is not an age at all.

**An item's dates are the last thing on its page, not the first.** They sit under the fields and directly above the buttons that operate on it. An item is opened to change what it says, and the dates are the one part of it that cannot be changed there - putting them first spends the top of the screen on the line that answers the question you did not come with. Above the buttons they are still read before the item is completed or deleted, which is the moment "next for three weeks" is worth knowing.

**A view that is open keeps itself current.** Items arrive without the app being asked for them: a schedule fires at the day boundary, and anything holding the capture API can put a line in the inbox from outside - a phone, a script, the Reminders import. A screen that went on showing the number it was rendered with would be answering "how much is waiting there" with how much *was* waiting, which is the one thing the navigation's counts exist to say. So the counts come current on their own, and the list under them with them.

**Except while you are working in it.** A list that reordered itself under a cursor, or replaced a row halfway through a decision about it, costs more than a stale number ever does - so the list holds still for as long as something is selected, being typed into, or being asked about in a dialog, and only the counts move. Letting the cursor go is what lets the list catch up, which makes holding it still something you can choose rather than something that happens to you.

### Panels
Around every view sit three pieces of the app's own furniture, and each one answers a different question:

- **the title bar** - *where am I*. The view's name, with the same count the navigation shows beside it, and then every step taken inside it: "Inbox / Processing / Action / Create project". A screen reached from inside another one is not a new place, it is a deeper one, and the trail is the only thing that says which. It matters most on the screens that have no nav entry of their own - processing, doing - which without it can only be told apart by what happens to be on them
- **the navigation** - *where else could I be*, and how much is waiting there
- **the key bar** - *what can I press here*

Each of the three is shown or hidden on its own, and how they stand is remembered the way a filter set is: it is an answer about how you want to work, not about this screen, so it survives leaving the view. One key opens the chooser and one letter each turns them on and off - a panel that is awkward to bring back is a panel nobody dares turn off in the first place.

**Zen mode is all three off at once, and it is one answer rather than three.** Turning them off by hand costs three presses and turning them back on costs remembering which of them were on, which is exactly the friction that stops a screen from ever being cleared. So the app carries a single state for "none of it, for now", it remembers the screen you had, and leaving zen gives that screen back exactly - including a panel that was already off.

Some screens open in zen without being asked, because they are screens you are in the middle of one item on: doing and processing, by default, and which ones is a setting. The rail is a list of other places you could be and the bar is a list of other things you could press, and neither is the question being answered while one item is in front of you. It is not a lock: zen can be turned off by hand on those screens like anywhere else, and it stays off for as long as you are there. And a zen that *you* asked for is never undone by the app - it is put back only when the app was the one that put it up.

**Nothing else changes when a panel goes.** Every key still works with the bar hidden, every view is still reachable with the rail hidden: the bar lists the keys, it does not own them, and hiding a list of where you could go does not close the doors. A panel is a thing shown, never a thing enabled - which is what makes turning them off safe enough to be worth offering.

### The filter line
Filtering is one line, typed, and there is nothing on the screen until it is asked for. A view opens as its list and nothing else; one key puts up a bar above it, and the same key takes the bar away and every filter with it.

The bar holds three things and no labels: **how many items are on the screen**, the **line**, and **apply**.

- **the count is the first thing, and it is one number when nothing is filtered.** Filtered, it reads `3 of 41` - what you are looking at, out of what the view holds. That is the loudness "Views" asks for, said in the place you are already looking rather than in a sentence underneath
- **the line is written in the notation an item is written in** (see "Writing an action"): `@home` for the context, `#car` for a tag, `#short` and `#focus` for the fields that wear a tag's notation, `due:thisweek` for a window of time, and everything else is words to match the name by. One notation for describing a thing and for asking for it, so there is nothing extra to learn and no second set of names
- **a line may only say what its view filters by.** Each view offers a subset (see the view's own section), and the line offers exactly that subset: `@home` on "Tasks" is a question rather than a token quietly ignored, because a filter that silently did nothing would be a list you cannot trust for the same reason a hidden filter is
- **apply is dead until the line has changed.** A button that can always be pressed says nothing about whether pressing it would do anything; this one says whether what you see is what you asked for
- **the app completes the names it knows**, because they are the names it will accept - see "Contexts" and "Tags", where the rule that a name comes off a remembered list rather than being typed fresh comes from
- **a name it does not know stops the line**, marked where it is written. It is either a name that is new or a name that is mistyped, and only the person typing knows which, so the app asks: create it, use one of at most three near ones, or take it out. Nothing is filtered until it is answered, because a filter with a name in it that means nothing is a list you cannot trust. The same question is asked of the meta line an item is written in (see "Writing an action"), because it is the same box asking about the same names - and answering it does not cost the thing that was interrupted: what you pressed happens as soon as the line is clean
- **closing the bar clears the filters.** A view narrowed by a box that is not on the screen is the quiet, untrustworthy filtering this document exists to avoid, and it is the reason the bar is the *only* thing that can hide filters - a view that is filtered opens with its bar up, whatever you left it as

The filter set is still remembered per view (see "Views"), so coming back to a view finds it as you left it, filtered and saying so. What is remembered is the filters, not the line: the line is written back out of them, in one fixed order, so the same filter set always reads the same way whatever order it was typed in.

### Filtering by name
Every view that can grow long carries the same name filter, and it behaves identically in all of them: **Someday/Maybe**, **Projects**, **Tasks**, **Next actions**, **Waiting for**, the **Calendar**, the **Scheduler** and the **Archive**.

Matching is case insensitive. Several words may be given and **all** of them have to be present, in any order and anywhere in the name - `call bank` finds "Call the bank about the mortgage". Each word matches as a substring and not as a whole word, so `mortg` still finds it. Substrings and not fuzzy matching, so that it is always obvious why something matched. Every word of the line that is not a name is part of it, and emptying the line is how it resets.

What counts as the name is whatever names the item on that screen: the title of an action or a project, and for a someday/maybe item its text, since that is all it has. For a **project**, the titles of the actions under it count as part of its name as well - a project is remembered by a step in it at least as often as by its outcome, and hiding a project whose action matched would be hiding the answer.

The **Inbox** deliberately has no name filter. It is worked through one item at a time, oldest first, until it is empty, and a filter there would only be a way to look away from something. Processing a single item ahead of the queue is a different thing and is allowed - it takes nothing out of sight - see "Inbox Zero". Neither does **Today**, for a related reason - see "Today".

### Filtering by tag
Tags are the other shared filter, written `#car` in the line, as many as you like. It is carried by every view whose items have tags on them - **Projects**, **Tasks**, **Next actions**, **Waiting for**, the **Calendar**, the **Archive** and **Someday/Maybe** - and behaves identically in all of them. It is what answers the review question "which part of my life am I starving?", which is why it reaches all of them and not only the working view.

- selected tags combine with **AND**: `#car #finance` is the items that are about both. A second tag narrows the question rather than widening it, which is what everything else on the line already does - the name filter requires all of its words, and a filter added to the line takes items away. It combined with OR first, and that made a tag the one thing on the line that could only ever make the list longer; the narrower question - the one a pile of tagged items is actually read with - had no way of being asked at all. The union is still one keystroke away, because taking a tag back out is how every filter here is loosened
- an item with **no** tags is excluded as soon as any tag is selected: the filter asks "is this about #car", and "about nothing in particular" answers no. The context filter behaves the same way and for the same reason - see "Filtering by context", where the opposite was tried first
- taking a tag out of the line is how it resets, and no tag in the line means all tags again, never none
- it matches the item's **own** tags. In "Projects" this deliberately differs from the name filter: a project is matched by the title of an action under it, but never by that action's tags. The name filter is a recall aid - a project is remembered by a step in it - while a tag says what the commitment itself belongs to, and a project does not belong to an area because one action in it happens to

The **Inbox** does not carry it, for the same reason it carries so little else: its items are raw captures, decided about one at a time in the order they arrived, and nothing on them has been answered yet - the tag included. **Today** carries no filters at all - see "Today".

### Filtering by context
Only **Next actions** carries it, because it is the only view that asks "what can I do now" - see "Contexts" for what a context is and "Next actions" for the rest of that screen's filters.

**One context at a time**, written `@home` in the line. An action carries one context and standing somewhere is one answer, so a second one in the line is refused the way an unknown name is - it would be asking for the actions that need two places at once, which is none of them.

- **an action with no context is shown only when no context is asked for.** The opposite was tried first, on the argument that "nothing required" is doable everywhere and a filter about prerequisites has nothing to exclude it by. In use it read as a leak: asking for `@home` and being shown four things that are not about being at home makes the answer to "what can I do here" longer than it should be, and the actions with no context are exactly the ones that are never *not* available, so they are never the ones you are looking for by asking. The unfiltered list is where they live, and it is one keystroke away
- **a bare `@grocery` in the line covers every parameter under it**: the Lidl errands and the Prisma ones as well as the shop-agnostic ones, while `@grocery(Lidl)` narrows to Lidl and the shop-agnostic ones (see "Parameters"). The bare name in the line is the context *type*, not an unspecified place being stood in. It read the other way first - bare `@grocery` meaning "at some shop, no idea which", so it found only the errands that named no shop - and that made the filter undo the one thing parameters are for: they exist to keep a type from splitting into a dozen top-level names, and a line that can ask about `@grocery(Lidl)` but never about groceries has split it again, in the one place it matters. The cost is that there is no way to ask for the shop-agnostic errands alone, which is a question nobody asks: those are the errands any shop satisfies, so every answer that would contain them contains them already
- **turning it off is taking it out of the line**, like every other filter. There is no second control for resetting one filter, because there is one control for all of them: the line

The views:

### Inbox
The inbox items that have not been decided about yet, oldest first.

This is the only view with a rule attached to being non-empty: it must be emptied, see "Inbox Zero".

### Someday/Maybe
The someday/maybe items - raw ideas worth revisiting some time, but not now.

- it is reviewed during the weekly review, on its own, longer cadence - see "Weekly review"
- the age shown is the age of the idea, from its creation date
- it is a plain list, filtered by the same line every long view is filtered by (see "The filter line"), and that line may ask about names and tags: those are the two things a someday/maybe item has to be narrowed by, and asking about a context or a size here would be asking about fields it deliberately does not carry
- each line shows the tags the idea carries, the way every other list shows them. It is the one thing on the line that is not the idea itself, and it is what turns a list of forty parked ideas into an answer about one area
- **a line carries no controls.** Everything that can be done to an idea - rewording it, changing the area, sending it back to the inbox - happens on its own page, one keystroke away. A list of ideas is read to think, not to act: the one row control this view had ("move on it") was a second, faster route into a decision that deserves the screen it now takes, and it sat on every row for the one occasion a year each is used

### Projects
The active projects, one to a line, with stalled ones loudly marked and snoozed ones shown differently to mark them as not yet ready. A project leaves this view the moment its `completedAt` is set, and is found in the "Archive" from then on.

**A line says what a project is, not what is in it.** The actions under each one used to be listed here, which made this view a page per project: the question it answers is "what am I running, and is any of it stuck", and that is a list you read down. What a project holds is on the project's own page, which is a keystroke away and where a project is worked on anyway (see "Editing items"). The line carries what the question needs - the title, whether it is stalled, whether it is snoozed, how many actions are open, and what it is about - and nothing else.

It carries the filter line, which matches a project by its title or by the title of any action under it - see "Filtering by name" - and by the project's own tags - see "Filtering by tag". It cannot ask for a context, a size or a focus: a project has none of them, and being told so is the box's job (see "The filter line").

### Tasks
The standalone actions: `completedAt` is empty and no project is set. Together with "Projects" this covers every commitment in the app.

Tasks is deliberately unremarkable, and each of its properties falls out of it being a view rather than a container:

- **it has no DOD.** It is not an outcome. It is not one commitment, it is the pile of small ones, and there is nothing to define done for.
- **it never shouts.** The stalled project check does not apply here. An empty Tasks view means there is nothing outstanding outside the projects, which is a good state and not a problem to fix.
- **it can not be deleted, and it does not have to be created.** It is a query, so it is simply always there - the same way the Inbox is.

Snoozed standalone actions appear here, shown differently to mark them as not yet ready. Standalone waiting for actions appear here too: Tasks answers "where does this action live", not "is it mine to act on".

It is a plain list, filtered by the same line every long view is filtered by (see "The filter line"), and that line may ask about tags and names only: a context or a size answers "what can I do now", and that is the question the "Next actions" view exists for.

Every standalone action is a next action (see "Standalone actions"), so all of them are already covered by the "Next actions" view and by step 4 of the weekly review. Tasks needs no review step of its own.

### Next actions
The main working view, and the one the app is used from day to day: the actions that are on you to act on. `becameNextActionAt` is set, `completedAt` is empty and "assigned to" is empty. Actions inside a project and standalone ones appear side by side - what matters here is that they are next, not where they live.

Note the distinction in naming. A waiting for action is still a next action of its project - that is what keeps a delegated project from counting as stalled - but it does not appear in this view, because this view is only the actions that are yours to act on.

Snoozed actions do **not** appear here, and this is the one view they are left out of. The question is "what do I do next", and an action that is snoozed cannot be done yet - it is not an answer to it. They are not hidden anywhere else: they still appear in their project's action list and in Tasks, they still count as a next action of their project for the stalled project check, and the weekly review still walks them - see "Time fields".

There is no separate "what can I do right now" screen. It was this same query with a few filters applied, and a second view that can quietly disagree with the first about what is next is exactly the kind of thing that stops being trusted. Asking "what can I do right now" is narrowing this view, not going somewhere else.

"Today" is not that second screen. It does not re-ask this view's question with the filters set differently: it shows what has run out of time, and what you decided this morning to aim at. That decision is recorded on the item and is derivable from nothing, so "Today" is a view over a field, the way every other view is. What was rejected here is a view over filter state.

#### Filters
The filters are what make one view enough. All of them are optional and combine with **AND** - each one narrows what the ones before it left. Every filter is reachable and resettable from the keyboard, since this is the screen the app is used from.

- **context** - `@home`, one at a time - see "Filtering by context". An action with no context is shown only when the line asks for no context
- **tags** - `#car`, as many as you like, combining with **AND** - see "Filtering by tag"
- **name** - every word in the line that is not a name, all of them having to match - see "Filtering by name"
- **duration** - `#short`, `#medium`, `#long`, one or several, combined with OR: what fits in the time available
- **needs focus** - `#focus` keeps only the actions that need real attention. An hour of it is worth spending on those, and nothing is more wasteful than spending it on things that could have been done half asleep. The opposite question - *drop what I cannot do while tired* - is not in the line yet, and is the one filter this notation still owes

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
- it is a plain list, filtered by the same line every long view is filtered by (see "The filter line"), and that line may ask about tags and names only - what is here is not yours to act on, so a context or a size has nothing to narrow
- the name filter is on the title and not on "assigned to": the field is free text, so filtering by it would be filtering by however the name happened to be typed that day

### Calendar
Everything with a real deadline, soonest first: the actions whose due date is set and whose `completedAt` is empty. It answers "what is coming at me", which is a question no other view asks - "Next actions" is ordered by how long something has been available, not by when it runs out of time.

Every kind of action stands side by side here - standalone, inside a project, and parked - because what matters is the date, not where the action lives. Waiting for actions appear as well: chasing a delegation at a specific moment is exactly what the due date is for (see "Waiting for"), and the date is no less real for the ball being in somebody else's court.

Overdue items are loudly marked, the same way stalled projects are. A due date is by definition a date with consequences outside your control, so one that has passed is the loudest thing the app has to say.

Snoozed items appear here too, shown differently to mark them as not yet ready. A `snoozeUntil` reaching past the due date is an error state and is marked as one - see "Error state".

Projects are never here, and neither are inbox or someday/maybe items, because none of them carries a due date - see "Deliberate omissions".

#### Filters
Written on the same line every long view is filtered by (see "The filter line"), combining with **AND**:

- **name** - every word of the line that is not notation, matching the action title - see "Filtering by name"
- **due** - `due:today`, `due:tomorrow`, `due:thisweek`, `due:nextweek`; leaving it out is anytime, which is how it resets. A word rather than a date, because the question is what is coming at me and the answer moves with the day. As in the "Archive" these are calendar periods and not rolling windows - "this week" is the week you are in, Monday to Sunday, and "next week" the one after it, not the next seven days.
- **tags** - `#car`, as many as you like - see "Filtering by tag"

The Calendar carries neither context, nor duration, nor needs focus. Those three ask whether something can be done right now, which is not the question this view asks.

**Overdue items are shown whatever the due filter says.** They are not what the filter is about: it asks what is coming, and something already late is not coming, it has arrived. Letting "today" hide an item that was due yesterday would be the app helping you look away from the one thing it exists to shout about.

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

- it shows the text, the rule in readable form, when it next fires and when it last did. A rule with nothing ahead of it says so rather than leaving the space blank - it is a schedule on its last day in the list, and a blank would read as a missing value rather than as an answer
- it carries the filter line, matching the text of the schedule - see "Filtering by name" and "The filter line". By text and by nothing else: a schedule has no context and no tag to narrow it by, so its line is words, and the box says so rather than ignoring a name quietly
- it carries no tag cloud. A schedule has no tags: it is not a commitment and belongs to no area of responsibility. What it produces does, once accepted
- it is reviewed during the weekly review, at step 6

### The read API
The views are readable from outside the app, so that an AI can analyse what is going on without anything being copied out by hand. It is the counterpart of the capture API (see "External capture"), which stays the only way in.

- what it returns is a **view**. The caller states its own filters as request parameters - the same filters the view itself offers, with the same semantics, and nothing beyond them - and no parameters means the complete, unfiltered view
- the caller's filters are its own: the screen's filter state is the screen's, and a read neither sees it nor touches it. An AI reading "Next actions" is asking its own question, not looking over your shoulder, and its answer must not depend on what you left toggled on last night
- the caller may spell its filters out one parameter at a time, or write the same line the screen is filtered with (see "The filter line") - `?q=@home #car` - which is the shorter way to say the same thing and the way a person would say it. Either way **nothing can be asked for that a view does not already offer**. Every possible response is a state the corresponding screen could be put in by setting its filters, so the API can never show a list the app itself could not - which is the property that matters. A caller composing arbitrary queries would be looking at a screen that cannot exist in the app, and that is the same reason saved filters are out of scope (see "Deliberate omissions")
- it is read only. Nothing is created, edited or completed through it. Whatever an outside tool wants to put into the app arrives in the inbox as a capture, and is decided about by hand in Inbox Zero
- reads are not audited. The audit log records what happened to an item, and a read makes no change worth recording. The one thing it can trigger is the daily clearing of `#today`, since an API read counts as first use of a new day, and that is never audited either - see "#today"

## Processes

### Inbox Zero
A dedicated mode that processes captured items one at a time, oldest first. It processes the inbox and nothing else: an idea that has become worth deciding about is sent back to the inbox first (see "Reshaping items"), so there is one screen where things are decided and one kind of item it decides about. A second entry point, from a list of things deliberately not being decided about, was tried and removed - it offered a branch or two that only made sense there, and it meant the app had two answers to "where do decisions happen".
While the process runs everything else is hidden from view - only the current item is shown.
For each item the only question asked is: what is it? The answer is one of:

The three branches that create something - Action, Project and Someday/Maybe - **read the captured item's first line as notation on the way into their form**: whatever that item's meta line can hold moves into it, and the words that are left become the title. An action's line holds all of it; a project's and a someday item's hold the tags, and a context or a size written on such a capture stays in the title, where it is seen and dealt with by hand (see "Writing a project", "Someday/maybe item"). A name that is not on a remembered list is prose and stays put, which is the same rule the meta line itself obeys and what keeps `marju@gmail.com` out of the context box (see "Contexts").

**Only that line is read as notation, and the rest of the text never is.** A body arrives from wherever the capture came from and was not written to be read: a link to a mail carries a `#`, the address it came from carries an `@`, and a note copied off a reminder can carry either. Reading only the first line means a body cannot reach the meta line at all, rather than being kept out of it by the remembered lists happening not to hold what it says (see "Contexts") - which is the difference between a rule and a piece of luck.

**The rest of the text goes where that branch keeps material**, and each of the three has somewhere real to put it:
- **Action**: the description, which is already where a link, a reference or anything else worth having to hand while doing it belongs (see "Action")
- **Project**: the description of the first action written on the form, and never the definition of done. A project has no description of its own on purpose, and material a project needs belongs to whichever of its actions needs it (see "Deliberate omissions") - which is where a promoted action's description lands too, so the two ways a project is written start from the same seed (see "Reshaping items"). The DOD stays empty and stays required: it is the one sentence the form exists to force out of you, and pre-filling it would let a captured body satisfy the check that makes a project a project, leaving a definition of done that defines nothing and is read at every review from then on
- **Someday/Maybe**: the text, all of it, into the one box the form has. An unclarified idea is a single free-form field and its tags, so there is nothing to split the capture into and nothing gained by splitting it

Nothing is decided by this. It fills in a form that is still answered by hand, and a line the notation cannot account for - two contexts, an unreadable date - is left alone entirely rather than half moved: what was dropped would be invisible, and this is the one moment the item is being looked at deliberately.

**Trash, Send to reference materials and the two-minute rule ignore it**, and there is nothing to fix there: those three create no object, so the line has nowhere to be read into. The item leaves exactly as it was captured, notation and body and all, and that is what the audit entry keeps.

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
- **Someday/Maybe**: worth looking at some time, but not now. The item becomes a someday/maybe item, still unclarified, and the answer is written on a form of its own: the **text**, which may be reworded to formulate the idea more clearly, and the **tags** that say which areas of responsibility it belongs to. It is the same form the item is edited on afterwards, so nothing asked here is asked in a shape it has nowhere else (see "Editing items").
  - **filing it was one click before, and the tags are what changed that.** The old rule was that answering "what is it?" is the decision being asked for and that wording an idea better is a separate act, done later on the item itself - which was right about the wording and wrong about the area, because an idea filed without one is an idea the monthly walk can neither group nor narrow, and the tag is not extra thinking: it is the thinking that just produced this answer. It is therefore asked for once, at the moment it is cheapest, and never asked for again
  - **neither field is required.** An idea with no area yet is filed with no tags, exactly as it was before, and the form costs one `Enter` in that case. What the step buys is the chance to say it while you are still holding the thought - not an obligation to have one

**A completion request is not asked what it is.** A captured line reporting an item finished somewhere else (see "Completion requests") is not a thing to do, to file or to throw away: it is a claim about an item that already exists, and none of the six answers above fits it. The item is looked up by the identity the request carries, and what is asked instead is whether the claim is true.

- **the item is there and open**: the screen shows what the request says - what the item was called, and when it was finished elsewhere - and the answer is to complete it or to ignore the request. Completing is the ordinary completion with everything that hangs off it (see "Completing a next action"), stamped with the time the request carries (see "Completion")
- **ignoring throws the request away and touches nothing else.** The item stays open exactly as it was, because that is what the answer said. Nothing is recorded about having been asked: "not done" is a state and not a decision, so the program that reported it may well find the item still open and say so again - which is correct, and is how a tick given by accident undoes itself
- **the item is gone, or was completed here already**: the screen says which, and the only answer is to throw the request away. There is nothing left to confirm, and the request has already done the one useful thing it could by saying that the two sides disagreed. An item promoted into a project is gone in this sense - the identity the request holds was the action's, and the project is not that item (see "Reshaping items")
- **an item created after the work was finished is not that item**, whatever identity the request carries, and the request is answered as one about something gone. An identity is only unique among the items that exist: one given back when an item is deleted or promoted can be handed out again to the next item created, and a request that outlived its item would otherwise be answered against whatever took its place. What rules that out is the request's own time - the item it is about existed before the work on it was finished (see implementation.md, "A completion request asks one question")
- **it is one item in the inbox and takes part in the run like any other**, oldest first, and it can be picked out and processed alone. The inbox is emptied whole, and a kind of item that could only be dealt with somewhere else would be a second place where the inbox gets emptied

The process ends when the inbox is empty. The inbox should be emptied regularly, and always as part of the weekly review.

Oldest first is the default way through, and the reason is that it removes a decision: the mode exists to make deciding cheap, and choosing what to decide about next is a decision like any other. It is not a lock. A single item may be picked out of the inbox and processed on its own, ahead of the queue.

That escape hatch is deliberate, and it is not the name filter the "Inbox" view rejects (see "Filtering by name"). A filter hides items, and what it hides is what you did not want to look at; picking one item hides nothing - the rest of the inbox is still in front of you, still oldest first, and still has to be emptied. What it exists for is the case where holding the queue would do harm: something has arrived that has to be decided now, and working down to it means making every decision before it in a hurry. A rushed decision about the wrong thing is worse than one item taken out of order, and the whole point of the mode is that each decision gets made properly, once.

Both are reachable directly and neither is hidden behind the other, but the app says which is which - the interface offers the run first and processing one picked item second, so the default reads as the default (see implementation.md, "Processing from the Inbox").

The screen can also be left at any point, on any item, without answering the question. Nothing is written when it is and the item stays exactly as it was, because leaving is not an answer - it is declining to give one yet. This is the same reasoning as processing out of order, and it is worth stating in its own right: the mode exists to get each decision made **properly**, and an answer forced out of someone who does not have one yet is a wrong answer, not a decided item. An item sat with and left alone is in a better state than one filed hastily into the wrong branch, because the hasty one is now out of the inbox and out of sight, and nothing will bring it back for a second look.

Neither escape hatch weakens the rule that the inbox must be emptied. That rule is kept by the person and not by the software - see "The protocol is followed, not enforced" - and what the app owes it is a state that cannot be misread and a cheap way to act, which is the loud inbox, the count on the nav, and the review step that asks.

### Doing one action
Every list in this app is a list of things not yet done, and reading one is deciding. Doing is the opposite of deciding, and the screen that is right for choosing is wrong for working: with an action selected in any view that shows actions, one key puts that action alone on the screen, large, and takes everything else off it - the other items, the counts, the filters, the badges on its own row.

- **it is a view, and it opens in zen mode.** It was first built as a mode: a box put on top of the view you were in, holding nothing that was not already in the row, with a pair of settings of its own for how much of the app's furniture went with it. Zen mode is that idea generalised (see "Panels"), and once every view can be stripped there is nothing left for a mode to be. So doing is a place you go: it has a name, it can be come back to, and what makes it bare is the setting that names it - the same setting any other screen can be named in
- **what it shows is the title and nothing else.** Not the project, not the context, not the due date: those are what you needed in order to pick this action, and this screen is for after the picking. An action whose title does not say what to do is an action that was written badly (see "Writing an action"), and hiding that is not a kindness
- **two keys are its own**: **done**, which completes the action exactly as completing it from the list does - the project check included, see "Completing a next action" - and **back**, which leaves it. Completing also leaves, since the thing being done is finished. A third key of its own would be a decision, and the point of the screen is that there are none left to make on it
- **the app's own keys keep working.** Capture, the view jumps, the help panel, the panels themselves: a screen is not entitled to take the app away, and a mode that swallowed every other key was doing exactly that. There is nothing here to protect - nothing is being typed and nothing is half-written - so the keys that work everywhere work here too
- **leaving goes back to the view it was opened from, with the same row still selected.** You did go somewhere, so the app has to put you back; the row you were on is where you were, and a screen for doing one thing should not cost you your place in the list of the others
- **it is offered on an action and only while the action is open.** Not on an inbox item, which has not been decided about yet; not on a someday/maybe item, which is not committed to; not on something already completed, which has nothing left to do
- **a timer is offered, off by default, and it is never written down.** Switched on, it counts the minutes since this action went on the screen - `07`, and `1:04` once an hour has gone. It is for one thing only: a feel for how long work actually takes, built up by noticing rather than by measuring. Nothing reads it, nothing stores it, no item carries it and no view reports it, so leaving the screen loses it and coming back starts it at `00` again. That is deliberate. A number that was kept would become a record of how long you took, and an action would arrive with an expectation attached - an estimate to beat, then a target, then a reason to feel late about work that was never promised for a time. This app does not do that to next actions (see "#today"), and a stopwatch is the shortest route back to it
- **the setting is where the timer starts, not where it stays.** The same key that shows and hides the ages on a list shows and hides it here, because it is the same question asked twice - *am I being told the time right now* - and one answer to it should not need two keys. There are no ages on this screen, so the key has exactly one meaning wherever it is pressed. What the key decides is not written down either: it holds for as long as the app is open and the settings file is the first word again after that. How the number is written is a setting too, since minutes, `H:MM` and plain elapsed minutes are different answers to what you are trying to notice

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
3. **Projects** - for each active project: is the DOD still what you want, and does it have a next action? This is where stalled projects, and projects left without a DOD, are fixed. Snoozed projects are walked too - the snooze date is one of the things being asked about.
4. **Next actions** - still valid, still a real physical next action? An action that has been next for weeks without moving usually means the action is phrased wrong, not that you are lazy. Standalone actions are covered here, since every one of them is a next action.
5. **Someday/Maybe** - is this still worth keeping, and is it still about what it says? An idea that has become live, and one that is dead, leave the same way: back to the inbox, to be answered there. This step runs on its own, longer cadence - see below.
6. **Scheduler** - walk the schedules: is this still wanted, and is the rule still right? A schedule set eight months ago goes on firing whether or not the reason for it still exists, and this is the only place that can be noticed before it lands in the inbox again.

The review is resumable. It can be interrupted at any point and continued later, and does not have to be finished in one sitting.

Progress is tracked by the per-item `lastReviewedAt`, stamped as each item is walked through and prefilled with the creation date when the item is created. There is no global "last weekly review" record: an item whose `lastReviewedAt` is older than its review period is simply outstanding, and that is also how the app shows that a review is due. A freshly created item is by construction not outstanding - it was consciously looked at when it was made.

The review period is a week for everything except someday/maybe items, which get a month by default (a setting - one number, in days). A parked idea does not change from week to week, and being asked every single review about a list that mostly answers "still parked" is the kind of chore that gets the whole review skipped - which would cost the views their trustworthiness, the one thing the review exists to protect. Snoozing does not stretch the period for the items that have one: a snoozed project or action is still walked when its period runs out, because its snooze date is one of the claims being reviewed.

### Editing items
Every item stays editable after it is created, and every edit is recorded in the audit log (see "Audit entry"). Nothing in the app is written once.

Editing happens in two places:

**Inline, in the views.** The cheap changes are made where the item is already shown: renaming an action, toggling a tag, setting a `snoozeUntil` or a due date, marking an action as next or parking it. These are the changes noticed while scanning a list, and making them cost a screen transition is the friction that ends with them not being made at all.

**In the item itself.** Opening a project or an action shows every field it has, editable, and for a project the full list of actions under it: add one, delete one, rename one, detach one (see "Reshaping items"). This is where a project is actually worked on. The DOD is prose and it is the field step 3 of the weekly review asks about, so it needs the room a list does not have.

**A someday/maybe item has a page of the same kind**, holding the two fields it has: the idea and its tags. It is where an idea is reworded, moved to the area it turns out to belong to, or sent back to the inbox - and it is the same form the Someday/Maybe branch of Inbox Zero files it on, because an item is written in one form wherever it is written (see the rule below). It is also the only screen that acts on an idea: the list it sits in carries no controls at all.

**Adding an action to a project opens the action form as its own screen**, from a control under the project's action list. The project is already answered there, the way it is for an action opened from a list. It is a screen and not a box on the project's page because of the rule below: an action is written in one form wherever it is written, and a form that had to be unfolded first was that form in a shape it has nowhere else - on the screen where actions are added most often. What a project's page holds is the project and its list; writing a new action is a step away from it, and coming back is where the new action already is.

A project is reachable this way from everywhere it appears - "Projects", the "Calendar", the "Archive" - and from any of its actions, wherever that action is seen.

**One form per kind of item.** An action is written in the same form wherever it is written - decided out of the inbox, added to a project, or opened from a list - and so is a project. The same fields in the same order, the same buttons in the same places. What differs between the screens is only what has already been answered: deciding an inbox item is where an action's project is *chosen*, and an action opened from a list shows its project without offering to change it, because moving an action between projects is Detach and Attach and not an edit (see "Reshaping items"). A form that looked different in each place would be four forms to keep true, and the fourth would be the one missing a field.

**Leaving without saving is always offered.** Nothing is written until it is saved, the way out is the same key that leaves any screen, and it costs nothing - an item sat with and left alone is exactly the state it was in. Saving is offered only when there is something to save, so a screen that has not been changed cannot be "saved" into an audit entry that records nothing.

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

**Back to the inbox** - a someday/maybe item becomes an inbox item again. It is the only way out of the someday/maybe list, and it covers both reasons for leaving: the idea has become live - the money is there, the boat is finally for sale - or it has died and should be trashed. Either way the answer is a decision, decisions are made in Inbox Zero, and the inbox is the one list that has to be emptied, so an idea put there gets answered rather than settling back among the parked ones.

**The tags go with it, written into the text**: "Restore the bicycle #hobby". An inbox item has no tags of its own (see "Someday/maybe item"), and dropping them would make the trip back cost something - you would arrive at the processing screen having lost a decision already made about this idea, and be asked to make it again. Written into the line they are still there to read, and still there to retype into the meta line of whatever the item becomes.

A text that is already sitting in the inbox collapses into the item that is there, by the rule every other way in obeys (see "Duplicate captures") - the idea is in the inbox either way, which is what was asked for.

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
