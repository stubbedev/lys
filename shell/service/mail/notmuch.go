package mail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"

	"github.com/stubbedev/lys/shell/internal/desktopnotify"
)

// CLI runs the notmuch binary.
type CLI struct{}

func notmuch(ctx context.Context, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, "notmuch", args...).Output() //nolint:gosec // the queries come from the user's own config
	if err == nil {
		return out, nil
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok && len(exit.Stderr) > 0 {
		return nil, fmt.Errorf("notmuch %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(exit.Stderr)))
	}
	return nil, fmt.Errorf("notmuch %s: %w", strings.Join(args, " "), err)
}

// Count is `notmuch count <query>`.
func (CLI) Count(ctx context.Context, query string) (uint32, error) {
	out, err := notmuch(ctx, "count", query)
	if err != nil {
		return 0, err
	}
	count, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("notmuch count %s: reading the count %q: %w", query, strings.TrimSpace(string(out)), err)
	}
	return uint32(count), nil
}

// Newest is `notmuch search --format=json --sort=newest-first
// --limit=N <query>`, one (sender, subject) per thread.
func (CLI) Newest(ctx context.Context, query string, limit int) ([]Message, error) {
	if limit <= 0 {
		return nil, nil
	}
	out, err := notmuch(ctx, "search", "--format=json", "--sort=newest-first", "--limit="+strconv.Itoa(limit), query)
	if err != nil {
		return nil, err
	}
	messages, err := parseSearch(out)
	if err != nil {
		return nil, fmt.Errorf("notmuch search %s: %w", query, err)
	}
	return messages, nil
}

// parseSearch reads notmuch's thread JSON. `authors` is
// "matched | non-matched"; the sender is the first matched author.
func parseSearch(out []byte) ([]Message, error) {
	var threads []struct {
		Authors string `json:"authors"`
		Subject string `json:"subject"`
	}
	if err := json.Unmarshal(out, &threads); err != nil {
		return nil, fmt.Errorf("parsing the thread JSON: %w", err)
	}
	messages := make([]Message, 0, len(threads))
	for _, thread := range threads {
		matched, _, _ := strings.Cut(thread.Authors, "|")
		first, _, _ := strings.Cut(matched, ",")
		m := Message{Sender: strings.TrimSpace(first), Subject: strings.TrimSpace(thread.Subject)}
		if m.Sender == "" && m.Subject == "" {
			continue
		}
		messages = append(messages, m)
	}
	return messages, nil
}

// DatabasePath is `notmuch config get database.path` (maildir_path).
func (CLI) DatabasePath(ctx context.Context) (string, bool) {
	out, err := notmuch(ctx, "config", "get", "database.path")
	if err != nil {
		log.Printf("mail: no notmuch database to watch: %v", err)
		return "", false
	}
	path := strings.TrimSpace(string(out))
	return path, path != ""
}

// DesktopNotifier fires mail notifications as "lys-shell" through the
// session notification daemon.
type DesktopNotifier struct {
	Sender *desktopnotify.Sender
}

// notifyTimeout bounds one fire-and-forget send.
const notifyTimeout = 5 * time.Second

// Notify sends in the background; failures are logged, never returned,
// as a missing notification must not fail the re-query.
func (d DesktopNotifier) Notify(summary, body, icon string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
		defer cancel()
		if _, err := d.Sender.Send(ctx, i18n.T("notification-source-mail"), summary, body, icon); err != nil {
			log.Printf("mail: notify: %v", err)
		}
	}()
}
