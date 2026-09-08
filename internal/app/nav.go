package app

// NavCounts is what the nav bar's per-view badges show: how many items are
// currently in each view, unaffected by whatever filters happen to be
// saved for that view — a badge that dimmed because of yesterday's filter
// would be exactly the kind of view that stops being trusted. Archive,
// Audit and Settings carry no badge (see implementation.md, "Navigation").
type NavCounts struct {
	Inbox     int
	Today     int
	Next      int
	Projects  int
	Tasks     int
	Waiting   int
	Calendar  int
	Someday   int
	Scheduler int
	Review    int // OutstandingTotal: the aged review steps only, not the inbox
}

// For is the same count by the view's own name, for the callers that have a
// slug rather than a field — the title bar reads it this way, so the number it
// shows cannot come to differ from the badge's (see implementation.md,
// "Panels"). A view with no badge answers zero, which is how nothing is shown.
func (c *NavCounts) For(view string) int {
	switch view {
	case "inbox":
		return c.Inbox
	case "today":
		return c.Today
	case "next":
		return c.Next
	case "projects":
		return c.Projects
	case "tasks":
		return c.Tasks
	case "waiting":
		return c.Waiting
	case "calendar":
		return c.Calendar
	case "someday":
		return c.Someday
	case "scheduler":
		return c.Scheduler
	case "review":
		return c.Review
	}
	return 0
}

func (a *App) NavCounts() (*NavCounts, error) {
	c := &NavCounts{}

	inbox, err := a.Inbox()
	if err != nil {
		return nil, err
	}
	c.Inbox = len(inbox)

	tv, err := a.TodayItems()
	if err != nil {
		return nil, err
	}
	c.Today = len(tv.OutOfTime) + len(tv.Picked)

	next, err := a.NextActions(Filters{})
	if err != nil {
		return nil, err
	}
	c.Next = len(next)

	projects, err := a.ProjectList(Filters{})
	if err != nil {
		return nil, err
	}
	c.Projects = len(projects)

	tasks, err := a.Tasks(Filters{})
	if err != nil {
		return nil, err
	}
	c.Tasks = len(tasks)

	waiting, err := a.WaitingFor(Filters{})
	if err != nil {
		return nil, err
	}
	c.Waiting = len(waiting)

	cal, err := a.Calendar(Filters{})
	if err != nil {
		return nil, err
	}
	c.Calendar = len(cal)

	someday, err := a.SomedayItems("")
	if err != nil {
		return nil, err
	}
	c.Someday = len(someday)

	sched, err := a.Schedules("")
	if err != nil {
		return nil, err
	}
	c.Scheduler = len(sched)

	rc, err := a.ReviewCounts()
	if err != nil {
		return nil, err
	}
	c.Review = rc.OutstandingTotal()

	return c, nil
}
