package settings

import (
	"context"
	"fmt"
	"log"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/intconv"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/secrets/client"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	passwordsID      = "passwords"
	itemsPerPage     = 50
	copiedSecretLife = 30 * time.Second
	repromptPath     = "passwords.reveal-reprompt-minutes"
	schemaAttribute  = "xdg:schema"
	hiddenText       = "••••••••"
	itemTimeLayout   = "2006-01-02 15:04"
)

type keyringSchema struct{ name, icon string }

const (
	itemKeyIcon  = "ld-key-round-symbolic"
	itemLockIcon = "ld-lock-symbolic"
)

var knownSchemas = map[string]keyringSchema{
	"chrome_libsecret_os_crypt_password_v2":     {"Chromium Safe Storage", "ld-globe-symbolic"},
	"chrome_libsecret_os_crypt_password_v1":     {"Chromium Safe Storage", "ld-globe-symbolic"},
	"org.freedesktop.NetworkManager.Connection": {"NetworkManager", "ld-wifi-symbolic"},
	"org.gnome.keyring.NetworkPassword":         {"GNOME network passwords", "ld-wifi-symbolic"},
	"org.git.Password":                          {"git", "ld-code-symbolic"},
	genericSchema:                               {"", itemKeyIcon},
}

var accountAttributes = []string{usernameAttr, "user"}

func passwordsPage(*config.Config) pageSpec {
	return pageSpec{
		id: passwordsID, navKey: "settings-nav-passwords", icon: "ld-key-round-symbolic",
		header: "settings-page-passwords",
		lead:   passwordsKeyring,
		sections: []sectionSpec{
			{title: "settings-section-passwords-reveal", rows: []rowSpec{field(repromptPath, durationChips(inMinutes, after(time.Minute), after(5*time.Minute), after(15*time.Minute), after(time.Hour), zeroAs(never)))}},
		},
	}
}

func storedBy(attrs map[string]string) string {
	schema := attrs[schemaAttribute]
	if known, ok := knownSchemas[schema]; ok {
		return known.name
	}
	return schema
}

func itemIcon(it client.Item) string {
	if it.Locked {
		return itemLockIcon
	}
	if known, ok := knownSchemas[it.Attributes[schemaAttribute]]; ok {
		return known.icon
	}
	return itemKeyIcon
}

func itemGroup(it client.Item) string {
	if by := storedBy(it.Attributes); by != "" {
		return by
	}
	return locale.T().Get("settings-passwords-group-other")
}

func shownAttributes(attrs map[string]string) []string {
	return slices.DeleteFunc(slices.Sorted(maps.Keys(attrs)), func(k string) bool { return k == schemaAttribute })
}

func itemTime(unix uint64, zone *time.Location) string {
	return time.Unix(intconv.Must[int64](unix), 0).In(zone).Format(itemTimeLayout)
}

func keyringItemTexts(it client.Item) []string {
	texts := []string{it.Label}
	for _, k := range slices.Sorted(maps.Keys(it.Attributes)) {
		texts = append(texts, it.Attributes[k])
	}
	return texts
}

var openKeyring = func(ctx context.Context) (*client.Client, func(), error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, nil, fmt.Errorf("connect to the session bus: %w", err)
	}
	c, err := client.OpenEncrypted(ctx, conn)
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	return c, func() {
		c.Close()
		_ = conn.Close()
	}, nil
}

type passwordsPanel struct {
	*widget.Box
	k          *kit
	c          *client.Client
	release    func()
	watch      *dbusx.Watch
	colls      []client.Collection
	coll       dbus.ObjectPath
	paths      []dbus.ObjectPath
	items      []client.Item
	shownTo    int
	seen       map[dbus.ObjectPath]time.Time
	now        func() time.Time
	problem    *widget.Label
	sidebar    *widget.ItemGroup
	collRows   map[dbus.ObjectPath]*settingRow
	collTitle  *widget.Label
	add        *widget.Button
	lock       *widget.Button
	lockFace   *widget.ButtonContent
	filter     *widget.SearchEntry
	list       *widget.ItemGroup
	state      *cardState
	rows       []widget.Widget
	more       *widget.Button
	expanded   dbus.ObjectPath
	detail     *widget.Box
	detailRow  *insetRow
	revealed   []byte
	secretLbl  *widget.Label
	revealFace *widget.ButtonContent
}

func passwordsKeyring(k *kit) widget.Widget {
	t := locale.T()
	p := &passwordsPanel{Box: sectionStack(), k: k, seen: map[dbus.ObjectPath]time.Time{}, now: time.Now}
	p.problem = k.errorText("")
	p.problem.SetVisible(false)
	p.Append(p.problem, false)
	p.sidebar = widget.NewItemGroup()
	p.sidebar.AddClass("password-collections")
	p.sidebar.SetAccessibleName(t.Get("settings-passwords-collections"))
	p.collTitle = k.label("", "password-collection-title", css.H4)
	p.collTitle.SetEllipsize(widget.EllipsizeEnd)
	p.lock, p.lockFace = k.contentButton(roleSecondary, "ld-lock-symbolic", t.Get("settings-passwords-lock"), p.toggleLock)
	change, _ := k.contentButton(roleSecondary, "ld-key-round-symbolic", t.Get("settings-passwords-change"), func() {
		coll := p.coll
		p.act(func(ctx context.Context) error { return p.c.ChangeLock(ctx, coll) })
	})
	p.add, _ = k.contentButton(roleSecondary, "ld-plus-symbolic", t.Get("settings-passwords-add"), p.openAddItem)
	head := gapBox(widget.Row)
	head.AppendAligned(p.collTitle, true, widget.AlignCenter)
	for _, b := range []*widget.Button{p.add, p.lock, change} {
		head.AppendAligned(b, false, widget.AlignCenter)
	}
	p.filter = widget.NewSearchEntry(k.face, k.textPx(), t.Get("settings-passwords-filter"))
	p.filter.SetDelay(0)
	p.filter.OnSearchChanged = func(string) { p.showItems() }
	p.state = k.cardState()
	p.list = widget.NewItemGroup(newInsetRow(p.filter), p.state)
	p.more, _ = k.contentButton(roleSecondary, "ld-chevron-down-symbolic", t.Get("settings-passwords-more"), p.loadMore)
	p.more.SetHAlign(widget.AlignCenter)
	p.more.SetVisible(false)
	items := gapBox(widget.Column)
	items.Append(head, false)
	items.Append(p.list, false)
	items.Append(p.more, false)
	body := gapBox(widget.Row)
	body.AppendAligned(p.sidebar, false, widget.AlignStart)
	body.Append(items, true)
	p.Append(newSection(k, "settings-section-passwords-keyring", body), false)
	p.detail = widget.NewBox(widget.Column, widget.Current().Spacing, 0)
	p.detail.AddClass("password-detail")
	p.detailRow = newInsetRow(p.detail)
	p.open()
	return p
}

type keyringOpened struct {
	c       *client.Client
	release func()
	colls   []client.Collection
	err     error
}

func (p *passwordsPanel) open() {
	async(p.k, func() keyringOpened {
		ctx, cancel := context.WithTimeout(context.Background(), sysCallTimeout)
		defer cancel()
		c, release, err := openKeyring(ctx)
		if err != nil {
			return keyringOpened{err: err}
		}
		colls, err := c.Collections(ctx)
		if err != nil {
			release()
			return keyringOpened{err: err}
		}
		return keyringOpened{c: c, release: release, colls: colls}
	}, func(o keyringOpened) {
		if o.err != nil {
			log.Printf("settings: passwords: %v", o.err)
			p.state.Unavailable("ld-key-round-symbolic", locale.T().Get("settings-passwords-unavailable"), o.err)
			return
		}
		p.c, p.release = o.c, o.release
		p.setCollections(o.colls)
		if len(p.colls) > 0 {
			p.pick(p.colls[0].Path)
		}
	})
}

func collectionTitle(c client.Collection) string {
	switch {
	case c.Locked && c.Label == "":
		return locale.T().Get("settings-passwords-locked-collection")
	case c.Locked:
		return locale.T().Get("settings-passwords-locked", i18n.Str("label", c.Label))
	}
	return c.Label
}

func collectionIcon(c client.Collection) string {
	if c.Locked {
		return itemLockIcon
	}
	return itemKeyIcon
}

func (p *passwordsPanel) setCollections(colls []client.Collection) {
	t := locale.T()
	p.colls = colls
	p.sidebar.Clear()
	p.collRows = make(map[dbus.ObjectPath]*settingRow, len(colls))
	for _, c := range colls {
		if _, ok := p.seen[c.Path]; !ok && !c.Locked {
			p.seen[c.Path] = p.now()
		}
		row := p.k.row(collectionTitle(c), t.Get("settings-passwords-count", i18n.Int("count", int64(c.Items))))
		row.AddClass("password-collection")
		row.SetMedia(p.k.icon(collectionIcon(c)), false)
		path := c.Path
		row.SetOnActivate(func() {
			if path != p.coll {
				p.pick(path)
			}
		})
		p.sidebar.Append(row)
		p.collRows[c.Path] = row
	}
	p.refreshLock()
}

func (p *passwordsPanel) current() (client.Collection, bool) {
	i := slices.IndexFunc(p.colls, func(c client.Collection) bool { return c.Path == p.coll })
	if i < 0 {
		return client.Collection{}, false
	}
	return p.colls[i], true
}

func (p *passwordsPanel) refreshLock() {
	t := locale.T()
	for path, row := range p.collRows {
		row.SetState(widget.StateSelected, path == p.coll)
	}
	c, ok := p.current()
	p.lock.SetVisible(ok)
	p.add.SetVisible(ok)
	p.collTitle.SetText(collectionTitle(c))
	if c.Locked {
		p.lockFace.SetIconName("ld-lock-open-symbolic")
		p.lockFace.SetLabel(t.Get("settings-passwords-unlock"))
	} else {
		p.lockFace.SetIconName("ld-lock-symbolic")
		p.lockFace.SetLabel(t.Get("settings-passwords-lock"))
	}
}

func (p *passwordsPanel) close() {
	p.hideSecret()
	p.unwatch()
	if p.release != nil {
		p.release()
		p.release = nil
	}
	p.c = nil
}

func (p *passwordsPanel) unwatch() {
	if p.watch == nil {
		return
	}
	err := p.watch.Stop()
	p.watch = nil
	if err != nil {
		p.fail(err)
	}
}

func (p *passwordsPanel) fail(err error) {
	p.problem.SetVisible(err != nil)
	if err != nil {
		p.problem.SetText(err.Error())
	}
}

func (p *passwordsPanel) pick(coll dbus.ObjectPath) {
	p.unwatch()
	p.coll = coll
	p.hideSecret()
	p.expanded = ""
	p.refreshLock()
	watch, err := p.c.WatchItems(context.Background(), coll, func() { p.k.invoke(p.reload) }, func(err error) {
		p.k.invoke(func() { p.fail(err) })
	})
	if err != nil {
		p.fail(err)
	}
	p.watch = watch
	p.load(false)
}

type itemsListing struct {
	paths []dbus.ObjectPath
	items []client.Item
	colls []client.Collection
	err   error
}

func (p *passwordsPanel) reload() { p.load(true) }

func (p *passwordsPanel) load(collections bool) {
	if p.c == nil {
		return
	}
	c, coll, colls := p.c, p.coll, p.colls
	async(p.k, func() itemsListing {
		ctx, cancel := context.WithTimeout(context.Background(), sysCallTimeout)
		defer cancel()
		if collections {
			var err error
			if colls, err = c.Collections(ctx); err != nil {
				return itemsListing{err: err}
			}
		}
		paths, err := c.Search(ctx, coll)
		if err != nil {
			return itemsListing{err: err}
		}
		items, err := c.Describe(ctx, paths[:min(len(paths), itemsPerPage)])
		return itemsListing{paths: paths, items: items, colls: colls, err: err}
	}, func(l itemsListing) {
		if l.err != nil {
			p.fail(l.err)
			return
		}
		p.fail(nil)
		p.setCollections(l.colls)
		p.paths, p.items, p.shownTo = l.paths, l.items, len(l.items)
		p.showItems()
	})
}

func (p *passwordsPanel) loadMore() {
	c, next := p.c, p.paths[p.shownTo:min(len(p.paths), p.shownTo+itemsPerPage)]
	async(p.k, func() itemsListing {
		ctx, cancel := context.WithTimeout(context.Background(), sysCallTimeout)
		defer cancel()
		items, err := c.Describe(ctx, next)
		return itemsListing{items: items, err: err}
	}, func(l itemsListing) {
		if l.err != nil {
			p.fail(l.err)
			return
		}
		p.items = append(p.items, l.items...)
		p.shownTo += len(l.items)
		p.showItems()
	})
}

func (p *passwordsPanel) showItems() {
	t := locale.T()
	for _, r := range p.rows {
		p.list.Remove(r)
	}
	p.rows = p.rows[:0]
	add := func(w widget.Widget) {
		p.list.Append(w)
		p.rows = append(p.rows, w)
	}
	filter := newWordFilter(p.filter.Text())
	shown := slices.DeleteFunc(slices.Clone(p.items), func(it client.Item) bool { return !filter.matches(keyringItemTexts(it)...) })
	slices.SortStableFunc(shown, func(a, b client.Item) int { return strings.Compare(itemGroup(a), itemGroup(b)) })
	expandedShown := false
	group := ""
	for i, it := range shown {
		if g := itemGroup(it); i == 0 || g != group {
			group = g
			add(newInsetRow(p.k.label(group, "password-group", css.CaptionHeading, css.DimLabel)))
		}
		add(p.itemRow(it))
		if it.Path == p.expanded {
			add(p.detailRow)
			expandedShown = true
		}
	}
	if !expandedShown {
		p.expanded = ""
		p.hideSecret()
	}
	if len(p.rows) == 0 {
		p.state.Empty("ld-key-round-symbolic", t.Get("settings-passwords-none"))
	} else {
		p.state.Clear()
	}
	p.more.SetVisible(p.shownTo < len(p.paths))
}

func (p *passwordsPanel) itemRow(it client.Item) *settingRow {
	title := it.Label
	if title == "" {
		title = locale.T().Get("settings-passwords-locked-item")
	}
	row := p.k.row(title, itemSummary(it.Attributes))
	row.AddClass("password-item")
	row.SetMedia(p.k.icon(itemIcon(it)), false)
	var more *widget.Button
	more = p.k.iconButton("ld-ellipsis-symbolic", "settings-passwords-item-options", func() {
		widget.OpenMenu(more, more, p.k.face, p.k.textPx(), p.itemMenu(it), widget.MenuOptions{Gap: widget.PopoverGap})
	})
	more.SetAccessibleName(locale.T().Get("settings-passwords-item-options"))
	row.AddAction(more)
	p.k.rowDisclosure(row.Item, it.Path == p.expanded, func(open bool) {
		if open != (it.Path == p.expanded) {
			p.toggleDetail(it)
		}
	})
	return row
}

func itemAccount(attrs map[string]string) (string, bool) {
	for _, k := range accountAttributes {
		if v := attrs[k]; v != "" {
			return v, true
		}
	}
	return "", false
}

func (p *passwordsPanel) itemMenu(it client.Item) []widget.MenuItem {
	t := locale.T()
	details := "settings-passwords-details-show"
	if it.Path == p.expanded {
		details = "settings-passwords-details-hide"
	}
	items := []widget.MenuItem{{Label: t.Get("settings-passwords-copy-secret"), OnClick: func() { p.reveal(it.Path, true) }}}
	if account, ok := itemAccount(it.Attributes); ok {
		items = append(items, widget.MenuItem{Label: t.Get("settings-passwords-copy-username"), OnClick: func() { p.fail(p.k.copyText(account)) }})
	}
	return append(items,
		widget.MenuItem{Label: t.Get(details), OnClick: func() { p.toggleDetail(it) }},
		widget.MenuSeparator(),
		widget.MenuItem{Label: t.Get("settings-passwords-delete"), OnClick: func() { p.confirmDelete(it) }},
	)
}

func (p *passwordsPanel) confirmDelete(it client.Item) {
	path := it.Path
	err := p.k.confirm(fmt.Sprintf("delete the keyring item %q", it.Label), "settings-passwords-delete-item", func() {
		p.act(func(ctx context.Context) error { return p.c.DeleteItem(ctx, path) })
		if p.expanded == path {
			p.expanded = ""
			p.showItems()
		}
	}, i18n.Str("label", it.Label))
	if err != nil {
		p.fail(err)
	}
}

// summaryAttributes are the attributes an item's row names it by, in order.
var summaryAttributes = []string{usernameAttr, "user", serviceAttr, "server", "application"}

// itemSummary is the line under an item's name: the account and place its
// attributes name, else the program that stored it.
func itemSummary(attrs map[string]string) string {
	var parts []string
	for _, k := range summaryAttributes {
		if v := attrs[k]; v != "" && !slices.Contains(parts, v) {
			parts = append(parts, v)
		}
	}
	if len(parts) == 0 {
		return storedBy(attrs)
	}
	return strings.Join(parts, " · ")
}

func (p *passwordsPanel) toggleDetail(it client.Item) {
	if p.expanded == it.Path {
		p.expanded = ""
		p.hideSecret()
	} else {
		p.showDetail(it)
	}
	p.showItems()
}

func (p *passwordsPanel) showDetail(it client.Item) {
	t := locale.T()
	p.hideSecret()
	p.expanded = it.Path
	p.detail.Clear()
	grid := p.k.formGrid()
	field := func(name string, value widget.Widget, more ...widget.Widget) {
		p.detail.Append(grid.row(grid.label(name), value, true, more...), false)
	}
	text := func(name, value string) {
		if value != "" {
			field(name, p.k.label(value))
		}
	}
	label := p.k.entry()
	label.SetText(it.Label)
	label.SetAccessibleName(t.Get("settings-passwords-label"))
	label.SetEnabled(!it.Locked)
	label.OnActivate = func(text string) {
		path := it.Path
		p.act(func(ctx context.Context) error { return p.c.SetLabel(ctx, path, strings.TrimSpace(text)) })
	}
	field(t.Get("settings-passwords-label"), label)
	for _, k := range shownAttributes(it.Attributes) {
		value := it.Attributes[k]
		if !slices.Contains(accountAttributes, k) || value == "" {
			text(k, value)
			continue
		}
		cp, _ := p.k.contentButton(roleSecondary, "ld-copy-symbolic", t.Get("settings-passwords-copy-username"), func() { p.fail(p.k.copyText(value)) })
		field(k, p.k.label(value), cp)
	}
	text(t.Get("settings-passwords-stored-by"), storedBy(it.Attributes))
	if it.Created != 0 {
		text(t.Get("settings-passwords-created"), itemTime(it.Created, time.Local))
	}
	if it.Modified != 0 {
		text(t.Get("settings-passwords-modified"), itemTime(it.Modified, time.Local))
	}
	p.secretLbl = p.k.label(hiddenText, css.Monospace)
	var reveal *widget.Button
	reveal, p.revealFace = p.k.contentButton(roleSecondary, "ld-eye-symbolic", t.Get("settings-passwords-reveal"), func() {
		if p.revealed != nil {
			p.hideSecret()
			return
		}
		p.reveal(it.Path, false)
	})
	cp, _ := p.k.contentButton(roleSecondary, "ld-copy-symbolic", t.Get("settings-passwords-copy"), func() { p.reveal(it.Path, true) })
	field(t.Get("settings-passwords-secret"), p.secretLbl, reveal, cp)
	del := p.k.removeButton("settings-passwords-delete", func() { p.confirmDelete(it) })
	del.SetHAlign(widget.AlignEnd)
	p.detail.Append(del, false)
}

func (p *passwordsPanel) repromptDue() bool {
	minutes, _ := p.k.store.value(repromptPath).(int64)
	if minutes == 0 {
		return false
	}
	seen, ok := p.seen[p.coll]
	return !ok || p.now().Sub(seen) > time.Duration(minutes)*time.Minute
}

type revealed struct {
	secret []byte
	err    error
}

func (p *passwordsPanel) reveal(item dbus.ObjectPath, copyIt bool) {
	c, coll, again := p.c, p.coll, p.repromptDue()
	async(p.k, func() revealed {
		ctx, cancel := context.WithTimeout(context.Background(), sysCallTimeout)
		defer cancel()
		if again {
			if err := c.Lock(ctx, []dbus.ObjectPath{coll}); err != nil {
				return revealed{err: err}
			}
		}
		s, err := c.Secret(ctx, item)
		return revealed{s, err}
	}, func(r revealed) {
		if r.err != nil {
			p.fail(r.err)
			return
		}
		p.seen[coll] = p.now()
		if copyIt {
			err := p.k.copySecret(string(r.secret))
			clear(r.secret)
			p.fail(err)
			return
		}
		p.hideSecret()
		p.revealed = r.secret
		p.secretLbl.SetText(string(r.secret))
		p.revealFace.SetIconName("ld-eye-off-symbolic")
		p.revealFace.SetLabel(locale.T().Get("settings-passwords-hide"))
	})
}

func (p *passwordsPanel) hideSecret() {
	clear(p.revealed)
	p.revealed = nil
	if p.secretLbl != nil {
		p.secretLbl.SetText(hiddenText)
		p.revealFace.SetIconName("ld-eye-symbolic")
		p.revealFace.SetLabel(locale.T().Get("settings-passwords-reveal"))
	}
}

func (p *passwordsPanel) toggleLock() {
	c, ok := p.current()
	if !ok {
		return
	}
	p.act(func(ctx context.Context) error {
		if c.Locked {
			return p.c.Unlock(ctx, []dbus.ObjectPath{c.Path})
		}
		return p.c.Lock(ctx, []dbus.ObjectPath{c.Path})
	})
}

func (p *passwordsPanel) act(fn func(context.Context) error) {
	if p.c == nil {
		return
	}
	async(p.k, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), sysCallTimeout)
		defer cancel()
		return fn(ctx)
	}, func(err error) {
		p.fail(err)
		p.reload()
	})
}
