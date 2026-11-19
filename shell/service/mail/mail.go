// Package mail is the unread-mail service: notmuch queries (one per
// account, or the single query), re-run when the notmuch maildir
// changes, with optional desktop notifications for newly-arrived
// messages. The bar module and the dropdown both read one Service.
package mail

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/feed"
	"github.com/stubbedev/lys/desktop/fswatch"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/icons"

	"github.com/stubbedev/lys/shell/internal/jinja"
)

// Debounce coalesces a maildir-sync burst into one re-query.
const Debounce = 500 * time.Millisecond

// NotifyMax caps how many per-message notifications one arrival burst
// fires, so a large sync cannot flood the notification daemon.
const NotifyMax = 5

// AccountUnread is one account's resolved icon and unread count; Err
// is why the count could not be read, and Count is then 0.
type AccountUnread struct {
	Name  string
	Icon  string
	Count uint32
	Err   error
}

// State is one published read: the per-account counts in config order
// (empty without accounts), the total of the counts that were read, and
// Err, every query that failed, nil when all were read.
type State struct {
	Accounts []AccountUnread
	Total    uint32
	Err      error
}

// Message is one newest-first search hit.
type Message struct {
	Sender  string
	Subject string
	Thread  string
}

// Notmuch is the query seam.
type Notmuch interface {
	// Count is `notmuch count <query>`.
	Count(ctx context.Context, query string) (uint32, error)
	// Newest returns up to limit newest matches.
	Newest(ctx context.Context, query string, limit int) ([]Message, error)
	// DatabasePath is `notmuch config get database.path`; ok=false
	// when there is no database to watch.
	DatabasePath(ctx context.Context) (string, bool)
}

// Notifier fires one desktop notification, fire-and-forget.
type Notifier interface {
	Notify(summary, body, icon, click string)
}

// Service holds the counts and runs the watch loop.
type Service struct {
	cfg      config.MailConfig
	notmuch  Notmuch
	notifier Notifier
	messages *i18n.Loader
	debounce time.Duration

	mu    sync.Mutex
	state State
	ticks *feed.Tick
}

// New builds a service; Run starts it. A nil notifier disables
// notifications regardless of the notify key.
func New(cfg config.MailConfig, notmuch Notmuch, notifier Notifier) *Service {
	return &Service{
		cfg:      cfg,
		notmuch:  notmuch,
		notifier: notifier,
		messages: i18n.Shell(),
		debounce: Debounce,
		ticks:    feed.NewTick(),
	}
}

// State returns the latest counts.
func (s *Service) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return State{Accounts: append([]AccountUnread(nil), s.state.Accounts...), Total: s.state.Total, Err: s.state.Err}
}

// PerAccount reports whether the accounts are counted, rather than the
// single query.
func (s *Service) PerAccount() bool { return len(s.cfg.Accounts) > 0 }

// Subscribe ticks after every publish; stop unregisters.
func (s *Service) Subscribe() (<-chan struct{}, func()) { return s.ticks.Subscribe() }

// Run seeds the counts without notifying (existing unread mail at
// startup is not "new"), then re-queries after each settled burst of
// maildir changes until ctx ends. Without a notmuch database the seed
// is all there is. The watch is armed before
// the seed query so a change during it is not lost.
func (s *Service) Run(ctx context.Context) {
	watcher := s.watchMaildir(ctx)
	s.recompute(ctx, false)
	if watcher == nil {
		<-ctx.Done()
		return
	}
	defer func() { _ = watcher.Close() }()
	for {
		select {
		case <-ctx.Done():
			return
		case _, open := <-watcher.Changed():
			if !open {
				<-ctx.Done()
				return
			}
		}
		// Settle the burst, then drain what queued before re-querying.
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.debounce):
		}
		select {
		case <-watcher.Changed():
		default:
		}
		s.recompute(ctx, true)
	}
}

// watchMaildir arms a recursive watch on the notmuch database path;
// nil when there is no database or it cannot be watched.
func (s *Service) watchMaildir(ctx context.Context) *fswatch.Watcher {
	path, ok := s.notmuch.DatabasePath(ctx)
	if !ok {
		return nil
	}
	watcher, err := fswatch.New(fswatch.Changes, true)
	if err != nil {
		log.Printf("mail: cannot create maildir watcher: %v", err)
		return nil
	}
	if err := watcher.Add(path); err != nil {
		log.Printf("mail: cannot watch maildir %s: %v", path, err)
		_ = watcher.Close()
		return nil
	}
	return watcher
}

// recompute re-runs the queries and publishes; with notify (and the
// notify key) set, every query whose count rose since its last read
// fires its batch. A query that failed, now or on the read before,
// notifies nothing.
func (s *Service) recompute(ctx context.Context, notify bool) {
	notify = notify && s.cfg.Notify && s.notifier != nil
	previous := s.State()

	var next State
	var batch []newMail
	var failures []error
	if len(s.cfg.Accounts) == 0 {
		count, err := s.notmuch.Count(ctx, s.cfg.Query)
		next.Total = count
		if err != nil {
			failures = append(failures, err)
		} else if notify && previous.Err == nil && count > previous.Total {
			batch = s.buildBatch(ctx, s.cfg.Query, s.cfg.IconName, s.messages.Get("mail-notification-sender"), previous.Total, count)
		}
	} else {
		prevCounts := make(map[string]uint32, len(previous.Accounts))
		for _, account := range previous.Accounts {
			if account.Err == nil {
				prevCounts[account.Name] = account.Count
			}
		}
		for _, account := range s.cfg.Accounts {
			icon := account.ResolvedIcon()
			count, err := s.notmuch.Count(ctx, account.Query)
			if err != nil {
				err = fmt.Errorf("account %s: %w", account.Name, err)
				failures = append(failures, err)
				next.Accounts = append(next.Accounts, AccountUnread{Name: account.Name, Icon: icon, Err: err})
				continue
			}
			next.Total += count
			if prev, read := prevCounts[account.Name]; notify && read && count > prev {
				batch = append(batch, s.buildBatch(ctx, account.Query, icon, account.Name, prev, count)...)
			}
			next.Accounts = append(next.Accounts, AccountUnread{Name: account.Name, Icon: icon, Count: count})
		}
	}
	next.Err = errors.Join(failures...)
	if next.Err != nil {
		log.Printf("mail: cannot count unread mail: %v", next.Err)
	}

	s.mu.Lock()
	s.state = next
	s.mu.Unlock()

	for _, item := range batch {
		s.notifier.Notify(
			renderNotification(s.cfg.NotifySummary, item),
			renderNotification(s.cfg.NotifyBody, item),
			notifyIconArg(item.icon),
			clickCommand(s.cfg.NotifyClick, item),
		)
	}

	// Subscribers hear about a pass once it is complete.
	feed.Notify(s.ticks)
}

// newMail is one arrived message rendered into a notification.
type newMail struct {
	icon    string
	sender  string
	subject string
	thread  string
	count   uint32 // total unread for the query after the arrival
	new     uint32 // how many arrived in this burst
}

// buildBatch fetches up to NotifyMax of the newest matches, falling
// back to one count-only entry when no details can be read.
func (s *Service) buildBatch(ctx context.Context, query, icon, fallbackSender string, previous, total uint32) []newMail {
	arrived := total - previous
	messages, err := s.notmuch.Newest(ctx, query, min(int(arrived), NotifyMax))
	if err != nil {
		log.Printf("mail: cannot read the newest messages, notifying the count only: %v", err)
	}
	if len(messages) == 0 {
		return []newMail{{
			icon:    icon,
			sender:  fallbackSender,
			subject: s.messages.Get("mail-notification-arrived", i18n.Int("new", int64(arrived)), i18n.Int("count", int64(total))),
			count:   total,
			new:     arrived,
		}}
	}
	batch := make([]newMail, 0, len(messages))
	for _, m := range messages {
		batch = append(batch, newMail{icon: icon, sender: m.Sender, subject: m.Subject, thread: m.Thread, count: total, new: arrived})
	}
	return batch
}

// renderNotification substitutes {{ sender }}, {{ subject }},
// {{ count }} and {{ new }}.
func renderNotification(format string, m newMail) string {
	return jinja.ReplaceVars(format, notificationVars(m))
}

func clickCommand(format string, m newMail) string {
	if format == "" || m.thread == "" {
		return ""
	}
	return jinja.ReplaceVars(format, map[string]string{"thread": m.thread})
}

func notificationVars(m newMail) map[string]string {
	return map[string]string{
		"sender": m.sender, "subject": m.subject,
		"count": strconv.FormatUint(uint64(m.count), 10), "new": strconv.FormatUint(uint64(m.new), 10),
	}
}

// notifyIconArg resolves an icon name to its SVG in lys-shell's icon
// registry (notification daemons do not search lys-shell's private icon
// dir by name), else the bare name for system-themed icons.
func notifyIconArg(icon string) string {
	registry, err := icons.NewRegistry()
	if err != nil {
		return icon
	}
	path := filepath.Join(registry.IconsDir(), icon+".svg")
	if _, err := os.Stat(path); err != nil {
		return icon
	}
	return path
}
