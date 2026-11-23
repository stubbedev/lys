package settings

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/shell/service/mail"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	notmuchTimeout  = 5 * time.Second
	maildirMaxDepth = 4
	listMailTags    = "mail-tag"
	listMailFolders = "mail-folder"
)

type queryTerm struct {
	field  string
	value  string
	negate bool
}

var queryTermPattern = regexp.MustCompile(`^(not\s+)?(tag|folder):("[^"]*"|\S+)$`)

func parseQuery(s string) ([]queryTerm, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, true
	}
	var out []queryTerm
	for part := range strings.SplitSeq(s, " and ") {
		m := queryTermPattern.FindStringSubmatch(strings.TrimSpace(part))
		if m == nil {
			return nil, false
		}
		out = append(out, queryTerm{field: m[2], value: strings.Trim(m[3], `"`), negate: m[1] != ""})
	}
	return out, true
}

func (q queryTerm) String() string {
	v := q.value
	if v == "" || strings.ContainsAny(v, " ()\t") {
		v = `"` + v + `"`
	}
	out := q.field + ":" + v
	if q.negate {
		out = "not " + out
	}
	return out
}

func joinQuery(terms []queryTerm) string {
	parts := make([]string, len(terms))
	for i, q := range terms {
		parts[i] = q.String()
	}
	return strings.Join(parts, " and ")
}

func (q queryTerm) label() string {
	key := "settings-mail-query-" + q.field
	if q.negate {
		key += "-not"
	}
	return locale.T().Get(key, i18n.Str("value", q.value))
}

func notmuchLines(args ...string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), notmuchTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "notmuch", args...).Output() //nolint:gosec // fixed notmuch listing arguments
	if err != nil {
		return nil, fmt.Errorf("notmuch %s: %w", strings.Join(args, " "), err)
	}
	return slices.DeleteFunc(strings.Split(strings.TrimSpace(string(out)), "\n"), func(s string) bool { return s == "" }), nil
}

func mailTags() []choice {
	tags, err := notmuchLines("search", "--output=tags", "*")
	if err != nil {
		log.Printf("settings: the mail tags: %v", err)
		return nil
	}
	out := make([]choice, len(tags))
	for i, tag := range tags {
		out[i] = choice{value: tag, label: tag}
	}
	return sortedChoices(out)
}

func mailFolders() []choice {
	root, err := notmuchLines("config", "get", "database.path")
	if err != nil || len(root) == 0 {
		log.Printf("settings: the mail folders: %v", err)
		return nil
	}
	return maildirFolders(root[0])
}

func maildirFolders(root string) []choice {
	var out []choice
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		if depth > maildirMaxDepth {
			return
		}
		for _, e := range entries(dir) {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".notmuch") {
				continue
			}
			sub := filepath.Join(dir, e.Name())
			if _, err := os.Stat(filepath.Join(sub, "cur")); err == nil {
				rel, _ := filepath.Rel(root, sub)
				out = append(out, choice{value: filepath.ToSlash(rel), label: filepath.ToSlash(rel)})
			}
			walk(sub, depth+1)
		}
	}
	walk(root, 0)
	return sortedChoices(out)
}

type mailQueryEditor struct {
	*widget.Box
	k       *kit
	slot    slot
	terms   []queryTerm
	tags    []choice
	folders []choice
	chips   *widget.ChipBox
	add     *widget.Button
	notice  *widget.Label
	raw     *text
	toggle  *modeToggle
	picker  *searchPicker
	count   *inlineState
	counted string
}

func newMailQueryEditor(k *kit, s slot, optional bool) *mailQueryEditor {
	t := locale.T()
	e := &mailQueryEditor{Box: widget.NewBox(widget.Column, widget.Current().Spacing/2, 0), k: k, slot: s}
	e.AddClass("mail-query-editor")
	e.chips = widget.NewChipBox()
	e.add = k.addButton("settings-mail-query-add", e.pickTerm)
	line := gapBox(widget.Row)
	line.Append(e.chips, true)
	line.AppendAligned(e.add, false, widget.AlignStart)
	e.notice = k.note(t.Get("settings-mail-query-handwritten"))
	e.raw = newText(k, s, optional)
	e.toggle = k.textModeToggle(func(bool) { e.refresh() })
	e.count = k.inlineState()
	e.count.AddClass("mail-query-count")
	e.Append(line, false)
	e.Append(e.count, false)
	e.Append(e.notice, false)
	e.AppendAligned(e.toggle, false, widget.AlignStart)
	e.Append(e.raw, false)
	async(k, systemLists[listMailTags], func(c []choice) { e.tags = c })
	async(k, systemLists[listMailFolders], func(c []choice) { e.folders = c })
	e.refresh()
	return e
}

func (e *mailQueryEditor) stacked() {}

func (e *mailQueryEditor) dirtyBadge() *widget.Badge { return e.raw.badge }

func (e *mailQueryEditor) refresh() {
	terms, ok := parseQuery(e.slot.text())
	e.terms = terms
	e.chips.Clear()
	for i, q := range terms {
		chip := widget.NewChip(e.k.face, q.label())
		chip.SetDismiss(chipRemoveName(locale.T()), func() {
			e.terms = slices.Delete(e.terms, i, i+1)
			e.write()
		})
		e.chips.Append(chip)
	}
	e.chips.SetVisible(ok)
	e.add.SetVisible(ok)
	e.notice.SetVisible(!ok)
	e.toggle.SetVisible(ok)
	e.raw.SetVisible(!ok || e.toggle.isOn())
	e.raw.refresh()
	e.recount()
}

type mailMatches struct {
	n   uint32
	err error
}

func (e *mailQueryEditor) recount() {
	query := strings.TrimSpace(e.slot.text())
	if query == "" {
		e.counted = ""
		e.count.Clear()
		return
	}
	if query == e.counted {
		return
	}
	e.counted = query
	async(e.k, func() mailMatches {
		n, err := e.k.mailCount(query)
		return mailMatches{n, err}
	}, func(m mailMatches) {
		if query != e.counted {
			return
		}
		t := locale.T()
		if m.err != nil {
			e.count.Error(t.Get("settings-mail-query-count-failed"), m.err, nil)
			return
		}
		e.count.Empty("", t.Get("settings-mail-query-count", i18n.Int("n", int64(m.n))))
	})
}

func countMail(query string) (uint32, error) {
	ctx, cancel := context.WithTimeout(context.Background(), notmuchTimeout)
	defer cancel()
	return mail.CLI{}.Count(ctx, query)
}

func (e *mailQueryEditor) write() {
	_ = e.slot.set(joinQuery(e.terms))
	e.refresh()
}

func (e *mailQueryEditor) options() []choice {
	t := locale.T()
	var out []choice
	add := func(q queryTerm, group string) {
		if !slices.Contains(e.terms, q) {
			out = append(out, choice{value: q.String(), label: q.label(), group: t.Get(group)})
		}
	}
	for _, tag := range e.tags {
		add(queryTerm{field: "tag", value: tag.value}, "settings-mail-query-tags")
	}
	for _, tag := range e.tags {
		add(queryTerm{field: "tag", value: tag.value, negate: true}, "settings-mail-query-not-tags")
	}
	for _, f := range e.folders {
		add(queryTerm{field: "folder", value: f.value}, "settings-mail-query-folders")
	}
	return out
}

func (e *mailQueryEditor) pickTerm() {
	if e.k.pickers == nil {
		return
	}
	e.picker = openChoicePicker(e.k, e.add, e.options(), func(v string) {
		terms, ok := parseQuery(v)
		if !ok || len(terms) != 1 {
			log.Printf("settings: the mail query term %q does not read back", v)
			return
		}
		e.terms = append(e.terms, terms[0])
		e.write()
	})
}

var mailQueryRow rowOpt = func(r *rowSpec) {
	r.editor = func(k *kit, s slot, meta config.FieldMeta) control { return newMailQueryEditor(k, s, meta.Optional) }
}
