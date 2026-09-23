package main

import (
	"context"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"todoistik/internal/apiclient"
)

// source is what this program is called in an inbox item's "source" (design.md,
// "Inbox item"): every capture made here is an agent's, whichever agent it is.
// Naming the model or the session instead would make the channel a different
// one every month, and the count exists to be read over a year.
const source = "mcp"

// Three tools, and the app's whole outside in them: the situation, one view,
// and the one way in.
//
// Not one tool per view. Eleven tools would put the rule that a view answers
// only by its own filters into the schemas themselves, which is worth
// something — but it would also put eleven near-identical schemas into every
// session that ever loads this server, to buy a refusal the app already gives
// in words a caller can read (`@home is not a filter this view has`). The
// three below cost a fraction of that and are refused just as clearly.
//
// Nothing here can change an item. A model with something to say files a
// capture and it is decided about by hand on the processing screen — the same
// round trip a reminder ticked off on a phone makes (design.md, "Completion
// requests"). That is not a restriction imposed on the model; it is the only
// way into the app there has ever been.

// views is what todoistik_read_view will answer, and the enum the client's
// schema carries — so a wrong name is a thing the model cannot send rather
// than a refusal it has to read. It is the app's own list, in the order the
// navigation rail has them.
var views = []string{
	"inbox", "today", "next", "projects", "tasks",
	"waiting", "calendar", "someday", "scheduler", "review", "archive",
}

type contextArgs struct {
	Archive string `json:"archive,omitempty"`
}

type readViewArgs struct {
	View string `json:"view"`
	Q    string `json:"q,omitempty"`
}

type captureArgs struct {
	Text string `json:"text"`
}

func newServer(c *apiclient.Client) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "todoistik", Version: version}, nil)

	mcp.AddTool(s, &mcp.Tool{
		Name: "todoistik_context",
		Description: "Read the whole GTD situation from todoistik: every view at once — " +
			"inbox, today, next actions, projects, standalone tasks, what is waiting on " +
			"somebody, the calendar, someday/maybe, schedules, the weekly review's counts, " +
			"and the recent archive. Use this to find out what is going on before answering " +
			"anything about the user's commitments. Items are named ::41 and can be referred " +
			"to by that.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"archive": {
					Type: "string",
					Description: "How far back the archive section reaches: a date (2026-09-13), " +
						"a day name, today, yesterday, week, month, year, or 2weeks/3months/2years " +
						"for that period and the ones before it. 'none' leaves the archive out. " +
						"Default: 3months.",
				},
			},
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in contextArgs) (*mcp.CallToolResult, any, error) {
		out, err := c.ContextText(in.Archive)
		if err != nil {
			return nil, nil, err
		}
		return text(out), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "todoistik_read_view",
		Description: "Read one view of todoistik, optionally filtered. Prefer this over " +
			"todoistik_context when the question is about one list. Each view narrows by its " +
			"own subset of filters; a filter a view does not offer is named in the answer " +
			"rather than applied, and so is a tag or context the app does not know.",
		InputSchema: &jsonschema.Schema{
			Type:     "object",
			Required: []string{"view"},
			Properties: map[string]*jsonschema.Schema{
				"view": {
					Type:        "string",
					Enum:        enumOf(views),
					Description: "Which view to read.",
				},
				"q": {
					Type: "string",
					Description: "The filter line, written the way it is typed on the screen: " +
						"'@home' a context, '#car' a tag, '#short'/'#medium'/'#long' a size, " +
						"'#focus' needing focus, 'due:today|tomorrow|thisweek|nextweek', " +
						"'completed:week' on the archive, and any bare words a title must contain. " +
						"Example: '@home #car milk'. Omit for the whole view.",
				},
			},
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in readViewArgs) (*mcp.CallToolResult, any, error) {
		out, err := c.ReadText(in.View, in.Q)
		if err != nil {
			return nil, nil, err
		}
		return text(out), nil, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "todoistik_capture",
		Description: "Put text into todoistik's inbox. This is the only way anything gets " +
			"into the app, and it decides nothing: the user processes the inbox by hand and " +
			"says what the item is. So a suggestion, a reminder or a finding goes here as " +
			"plain words — do not try to write it as a finished action, and do not use this " +
			"to complete, edit or delete anything. Several lines are one item, of which the " +
			"first is what the inbox shows.",
		InputSchema: &jsonschema.Schema{
			Type:     "object",
			Required: []string{"text"},
			Properties: map[string]*jsonschema.Schema{
				"text": {Type: "string", Description: "The text to capture."},
			},
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in captureArgs) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(in.Text) == "" {
			// said here rather than sent, because the app's refusal for this
			// would be about a body it never received
			return nil, nil, errEmptyCapture
		}
		status, err := c.Capture(in.Text, source)
		if err != nil {
			return nil, nil, err
		}
		// "duplicate" is not a failure and must not read like one: the loop it
		// describes is already in the inbox waiting to be decided about
		// (design.md, "Duplicate captures"), and a model told it failed would
		// try again.
		switch status {
		case "duplicate":
			return text("Already in the inbox, so nothing was added. The identical text is already there waiting to be processed."), nil, nil
		default:
			return text("Added to the inbox."), nil, nil
		}
	})

	return s
}

func text(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}

func enumOf(names []string) []any {
	out := make([]any, 0, len(names))
	for _, n := range names {
		out = append(out, n)
	}
	return out
}
