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
	Review    int // OutstandingTotal: the 7-day-aged steps only, not the inbox
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
