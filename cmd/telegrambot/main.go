// Command telegrambot is todoistik in a Telegram chat: a message is a capture
// into the inbox, and a command reads a view.
//
// It is the phone's way in and its way to read, and nothing else. Nothing is
// processed, completed or reviewed from a chat (design.md, "Design
// principles"), so it talks to the app the way every program outside it
// does — the capture API in, the read API out, under the bearer token — and
// can do nothing those two cannot.
//
// It polls Telegram rather than being called by it, so it runs beside the app,
// reaches it on localhost, and needs no address of its own on the internet.
//
// See implementation.md, "Telegram".
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"todoistik/internal/apiclient"
)

func main() {
	fs := flag.NewFlagSet("telegrambot", flag.ExitOnError)
	var (
		chat      = fs.String("chat", os.Getenv("TELEGRAM_CHAT"), "the one chat id the bot answers; empty answers nobody and prints who wrote")
		tokenFile = fs.String("token-file", "", "a file holding the bot token; otherwise TELEGRAM_TOKEN")
		base      = fs.String("url", env("TODOISTIK_URL", "http://127.0.0.1:8390"), "the running app's base URL")
		token     = fs.String("token", os.Getenv("TODOISTIK_TOKEN"), "the app's bearer token; empty for a server started without one")
	)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `telegrambot answers one Telegram chat: a message is captured into the inbox,
a command reads a view.

  TELEGRAM_TOKEN=... telegrambot -chat 123456789

The bot token is never a flag: a flag is on the process list for anything on
the machine to read.

`)
		fs.PrintDefaults()
	}
	fs.Parse(os.Args[1:])

	botToken, err := readToken(*tokenFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "telegrambot: %v\n", err)
		os.Exit(2)
	}
	var chatID int64
	if strings.TrimSpace(*chat) != "" {
		if chatID, err = strconv.ParseInt(strings.TrimSpace(*chat), 10, 64); err != nil {
			fmt.Fprintf(os.Stderr, "telegrambot: -chat %q is not a chat id\n", *chat)
			os.Exit(2)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	r := &runner{
		tg:     newTelegram(botToken),
		bot:    &bot{app: apiclient.New(apiclient.Base(*base), *token)},
		chat:   chatID,
		out:    os.Stdout,
		errOut: os.Stderr,
		retry:  5 * time.Second,
	}
	if chatID == 0 {
		fmt.Printf("answering nobody: -chat is not set. Write to the bot, and its chat id is printed here\n")
	} else {
		fmt.Printf("answering chat %d, for %s\n", chatID, apiclient.Base(*base))
	}
	if err := r.run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "telegrambot: %v\n", err)
		os.Exit(1)
	}
}

type runner struct {
	tg     *telegram
	bot    *bot
	chat   int64
	out    io.Writer
	errOut io.Writer
	retry  time.Duration
}

// run answers messages until ctx ends or Telegram refuses the bot outright.
//
// It prints what happened and nothing else: an idle hour is an empty log, so
// anything in it is something that moved — the rule remindersync's loop keeps.
func (r *runner) run(ctx context.Context) error {
	if err := r.tg.setCommands(ctx, views); err != nil {
		if errors.Is(err, errTelegramFatal) {
			return err
		}
		// the menu is a convenience; a command typed out works without it
		r.fail("the command menu was not set: %v", err)
	}

	var (
		offset  int64
		failing bool
	)
	for ctx.Err() == nil {
		updates, err := r.tg.updates(ctx, offset, pollSeconds)
		if ctx.Err() != nil {
			break
		}
		if err != nil {
			if errors.Is(err, errTelegramFatal) {
				return err
			}
			// a network that is away is said once, when it goes and when it
			// comes back, not every five seconds in between
			if !failing {
				r.fail("cannot reach telegram, retrying: %v", err)
				failing = true
			}
			select {
			case <-time.After(r.retry):
			case <-ctx.Done():
			}
			continue
		}
		if failing {
			r.log("telegram is reachable again")
			failing = false
		}
		for _, u := range updates {
			offset = u.ID + 1
			if u.Message != nil {
				r.handle(ctx, u.Message)
			}
		}
	}

	// what was answered is confirmed before leaving, or the next start would
	// answer the last batch a second time
	if offset > 0 {
		confirm, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		r.tg.updates(confirm, offset, 0)
	}
	return nil
}

func (r *runner) handle(ctx context.Context, m *message) {
	if r.chat == 0 || m.Chat.ID != r.chat {
		// a bot is found by anyone who searches its name. A stranger gets no
		// answer — not even one saying no — and the id is printed, which is
		// also how the owner's own id is found on the first run
		who := ""
		if m.Chat.Username != "" {
			who = " (@" + m.Chat.Username + ")"
		}
		r.fail("ignored a message from chat %d%s", m.Chat.ID, who)
		return
	}
	o := r.bot.answer(m.Text)
	if o.failed {
		r.fail("%s", o.log)
	} else {
		r.log("%s", o.log)
	}
	for _, reply := range o.replies {
		if err := r.tg.send(ctx, m.Chat.ID, reply); err != nil {
			r.fail("the reply was not sent: %v", err)
			return
		}
	}
}

func (r *runner) log(format string, args ...any) {
	fmt.Fprintf(r.out, "%s %s\n", time.Now().Format(time.DateTime), fmt.Sprintf(format, args...))
}

func (r *runner) fail(format string, args ...any) {
	fmt.Fprintf(r.errOut, "%s %s\n", time.Now().Format(time.DateTime), fmt.Sprintf(format, args...))
}

// readToken takes the bot token from a file when one is named, and from the
// environment otherwise. Never from a flag: whoever holds it can read
// everything the bot is sent, and a flag is on the process list.
func readToken(file string) (string, error) {
	if strings.TrimSpace(file) != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		if tok := strings.TrimSpace(string(b)); tok != "" {
			return tok, nil
		}
		return "", fmt.Errorf("%s holds no token", file)
	}
	tok := strings.TrimSpace(os.Getenv("TELEGRAM_TOKEN"))
	if tok == "" {
		return "", errors.New("no bot token: set TELEGRAM_TOKEN or -token-file (from @BotFather)")
	}
	return tok, nil
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
