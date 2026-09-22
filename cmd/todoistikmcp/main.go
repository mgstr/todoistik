// Command todoistikmcp is todoistik as an MCP server: a model reads the views
// and files captures, over the two endpoints everything outside the app uses.
//
// It is the fourth program in cmd/ built the same way as the other three — it
// talks to the running app through internal/apiclient, never to the database —
// and it can do nothing the read and capture APIs cannot. That is the point
// rather than a limitation: a model asking todoistik a question is asking for
// a view, and a model with something to say puts it in the inbox and it is
// decided about by hand in Inbox Zero (design.md, "Design principles").
//
// It speaks MCP over stdin and stdout, so the client starts it as a child
// process and nothing has to be reachable from the network.
//
// See implementation.md, "The app as an MCP server".
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"todoistik/internal/apiclient"
)

// version is what the server calls itself to the client on connect. The app
// has no version anywhere else, so this one is the protocol's own field rather
// than a number the repo keeps.
const version = "1"

// errEmptyCapture is refused here rather than sent, so that the answer is
// about what was asked rather than about a body the app never received.
var errEmptyCapture = errors.New("nothing to capture: text is empty")

func main() {
	fs := flag.NewFlagSet("todoistikmcp", flag.ExitOnError)
	var (
		base      = fs.String("url", env("TODOISTIK_URL", "http://127.0.0.1:8390"), "the running app's base URL")
		token     = fs.String("token", os.Getenv("TODOISTIK_TOKEN"), "the app's bearer token; empty for a server started without one")
		tokenFile = fs.String("token-file", "", "a file holding the bearer token; otherwise -token or TODOISTIK_TOKEN")
	)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `todoistikmcp is todoistik as an MCP server, spoken over stdin and stdout.
It is started by the MCP client, not by hand:

  {"command": "todoistikmcp", "env": {"TODOISTIK_TOKEN": "..."}}

It reads the views and files captures, and can do nothing the read and capture
APIs cannot.

`)
		fs.PrintDefaults()
	}
	fs.Parse(os.Args[1:])

	tok, err := readToken(*tokenFile, *token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "todoistikmcp: %v\n", err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := newServer(apiclient.New(apiclient.Base(*base), tok))
	// Nothing but protocol may go to stdout — the client is parsing it as a
	// message stream, and one stray line ends the session. Everything this
	// program has to say goes to stderr, which is why there is no logging in
	// the tools at all.
	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "todoistikmcp: %v\n", err)
		os.Exit(1)
	}
}

// readToken keeps the bearer token off the process list where it can, for the
// reason the mail password and the bot token are never flags. -token is still
// accepted, since the app's own README has always shown it that way.
func readToken(file, flagToken string) (string, error) {
	if strings.TrimSpace(file) == "" {
		return flagToken, nil
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("-token-file: %v", err)
	}
	return strings.TrimSpace(string(b)), nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
