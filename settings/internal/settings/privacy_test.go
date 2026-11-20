package settings

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/portal/permissions"
	"github.com/stubbedev/lys/desktop/settingspage"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/render"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// revoked is one Revoke the page made.
type revoked struct {
	app   string
	table permissions.Table
	id    string
}

// fakeGrants is the grants the page lists; a revoke takes the grant out
// unless failRevoke is set, and listErr fails every listing.
type fakeGrants struct {
	conn       *dbus.Conn
	apps       []permissions.App
	revokes    []revoked
	sets       []string
	listErr    error
	failRevoke error
	opened     int
	closed     int
	lists      int
	watches    []*dbusx.Watch
	changed    func()
}

func (f *fakeGrants) watching() int {
	n := 0
	for _, w := range f.watches {
		select {
		case <-w.Done():
		default:
			n++
		}
	}
	return n
}

func (f *fakeGrants) List(context.Context) ([]permissions.App, error) {
	f.lists++
	return f.apps, f.listErr
}

func (f *fakeGrants) Set(_ context.Context, app string, table permissions.Table, id string, perms []string) error {
	f.sets = append(f.sets, app+" "+string(table)+" "+id+" "+strings.Join(perms, ","))
	return nil
}

func (f *fakeGrants) Watch(ctx context.Context, changed func(), failed func(error)) (*dbusx.Watch, error) {
	w, err := dbusx.WatchSignals(ctx, f.conn, dbusx.FromBus(), failed,
		dbusx.OnOwnerChange("org.example.Unowned", func(string, string, dbus.Sequence) error { return nil }))
	if err != nil {
		return nil, err
	}
	f.watches = append(f.watches, w)
	f.changed = changed
	return w, nil
}

func (f *fakeGrants) Revoke(_ context.Context, app string, table permissions.Table, id string) (int, error) {
	f.revokes = append(f.revokes, revoked{app, table, id})
	if f.failRevoke != nil {
		return 0, f.failRevoke
	}
	for i := range f.apps {
		if f.apps[i].ID != app {
			continue
		}
		f.apps[i].Grants = slices.DeleteFunc(f.apps[i].Grants, func(g permissions.Grant) bool { return g.Table == table && g.ID == id })
	}
	f.apps = slices.DeleteFunc(f.apps, func(a permissions.App) bool { return len(a.Grants) == 0 })
	return 1, nil
}

func serveGrants(t *testing.T, f *fakeGrants) {
	t.Helper()
	f.conn = dbustest.Start(t).Conn(t)
	open := openGrants
	openGrants = func() (grantStore, func(), error) {
		f.opened++
		return f, func() { f.closed++ }, nil
	}
	t.Cleanup(func() { openGrants = open })
}

func twoApps() []permissions.App {
	return []permissions.App{
		{ID: "com.obsproject.Studio", Grants: []permissions.Grant{{Table: permissions.ScreenCast, ID: "token-1", Value: "yes"}}},
		{ID: "org.app", Grants: []permissions.Grant{
			{Table: permissions.Devices, ID: "camera", Value: "yes"},
			{Table: permissions.Shortcuts, ID: "toggle", Value: "Toggle recording"},
		}},
	}
}

func grantsByAppFooter(k *kit) *grantsList {
	l := grantsFooter(k).(*grantsList)
	l.view.SetSelected(grantsByApp)
	return l
}

func revokeButtons(root widget.Widget) []*widget.Button {
	var out []*widget.Button
	for _, b := range treetest.All[*widget.Button](root) {
		if slices.Contains(treetest.Labels(b), locale.T().Get("settings-privacy-revoke")) {
			out = append(out, b)
		}
	}
	return out
}

// The Privacy page is listed under its settingspage id and holds the
// screen-share reminder; its footer lists every app's grants with a
// revoke button each, and a revoke takes exactly that grant and lists
// again without it.
func TestPrivacyPageListsAndRevokesGrants(t *testing.T) {
	k := testKit(t, "")
	if !slices.ContainsFunc(pages(k.store.svc.Config(), bothParts), func(p settingspage.Page) bool { return p.ID == settingspage.PrivacyID && p.Title != "" }) {
		t.Fatalf("the page list lacks %q", settingspage.PrivacyID)
	}
	spec, _ := pageByID(layout(k.store.svc.Config(), bothParts), settingspage.PrivacyID)
	if len(spec.sections) == 0 || spec.sections[0].rows[0].path != "portal.screen-cast.reminder-days" {
		t.Errorf("the page's first row is not the reminder: %+v", spec.sections)
	}
	f := &fakeGrants{apps: twoApps()}
	serveGrants(t, f)
	footer := grantsByAppFooter(k)
	labels := strings.Join(treetest.Labels(footer), "\n")
	for _, want := range []string{"com.obsproject.Studio", "org.app", locale.T().Get("settings-privacy-table-devices") + " · camera", "Toggle recording"} {
		if !strings.Contains(labels, want) {
			t.Errorf("the list lacks %q:\n%s", want, labels)
		}
	}
	buttons := revokeButtons(footer)
	if len(buttons) != 3 {
		t.Fatalf("%d revoke buttons, want one per grant", len(buttons))
	}
	buttons[1].ClickAt(widget.Point{})
	if want := []revoked{{"org.app", permissions.Devices, "camera"}}; !slices.Equal(f.revokes, want) {
		t.Errorf("revoked %+v, want %+v", f.revokes, want)
	}
	if labels := strings.Join(treetest.Labels(footer), "\n"); strings.Contains(labels, "· camera") || !strings.Contains(labels, "Toggle recording") {
		t.Errorf("after the revoke the list reads:\n%s", labels)
	}
	if len(revokeButtons(footer)) != 2 {
		t.Error("the revoked grant kept its button")
	}
	footer.close()
	if f.opened != 1 || f.closed != 1 || len(f.watches) != 1 || f.watching() != 0 {
		t.Errorf("opened the grants %d times, closed them %d, %d of %d watches left; want one connection and one watch for the page's life", f.opened, f.closed, f.watching(), len(f.watches))
	}
}

// A revoke that fails says why and leaves the grant listed; a store that
// cannot be read says so; an empty store says nothing was granted.
func TestPrivacyPageFailuresAreShown(t *testing.T) {
	k := testKit(t, "")
	f := &fakeGrants{apps: twoApps(), failRevoke: errors.New("the store went away")}
	serveGrants(t, f)
	footer := grantsByAppFooter(k)
	revokeButtons(footer)[0].ClickAt(widget.Point{})
	labels := strings.Join(treetest.Labels(footer), "\n")
	if !strings.Contains(labels, locale.T().Get("settings-privacy-revoke-failed")+": the store went away") || !strings.Contains(labels, "com.obsproject.Studio") {
		t.Errorf("a failed revoke shows:\n%s", labels)
	}

	serveGrants(t, &fakeGrants{listErr: errors.New("no permission store")})
	labels = strings.Join(treetest.Labels(grantsFooter(k)), "\n")
	if !strings.Contains(labels, locale.T().Get("settings-privacy-unavailable")+": no permission store") {
		t.Errorf("an unreadable store shows:\n%s", labels)
	}

	serveGrants(t, &fakeGrants{})
	if labels := treetest.Labels(grantsFooter(k)); !slices.Contains(labels, locale.T().Get("settings-privacy-none")) || len(revokeButtons(grantsFooter(k))) != 0 {
		t.Errorf("an empty store shows %q", labels)
	}
}

// Every table the model lists has its label.
func TestEveryGrantTableHasALabel(t *testing.T) {
	for _, table := range permissions.Tables {
		key := "settings-privacy-table-" + string(table)
		if got := locale.T().Get(key); got == "" || got == key {
			t.Errorf("%s has no label", table)
		}
	}
}

func appsWithSwitches() []permissions.App {
	return []permissions.App{
		{ID: "org.app", Grants: []permissions.Grant{
			{Table: permissions.Background, ID: "background", Value: "yes"},
			{Table: permissions.Notifications, ID: "notification", Value: "no"},
			{Table: permissions.GameMode, ID: "gamemode", Value: "yes"},
			{Table: permissions.ScreenCast, ID: "token-1", Value: "yes"},
		}},
		{ID: "org.other", Grants: []permissions.Grant{{Table: permissions.Background, ID: "background", Value: "yes"}}},
	}
}

func TestBackgroundAndNotificationsAreSwitchesAndTheStoreIsFollowed(t *testing.T) {
	k := testKit(t, "")
	f := &fakeGrants{apps: appsWithSwitches()}
	serveGrants(t, f)
	footer := grantsByAppFooter(k)
	switches := treetest.All[*widget.Switch](footer)
	if len(switches) != 3 || !switches[0].On() || switches[1].On() {
		t.Fatalf("%d switches, the first on %v, the second on %v", len(switches), switches[0].On(), switches[1].On())
	}
	if len(revokeButtons(footer)) != 1 {
		t.Fatalf("%d revoke buttons, want only the screen share's: game mode is not lys's to revoke", len(revokeButtons(footer)))
	}
	if !slices.Contains(treetest.Labels(footer), locale.T().Get("settings-privacy-read-only")) {
		t.Fatal("game mode is not marked as unused by lys")
	}
	switches[0].SetOn(false)
	if want := []string{"org.app background background no"}; !slices.Equal(f.sets, want) {
		t.Fatalf("sets %q, want %q", f.sets, want)
	}
	lists := f.lists
	f.changed()
	if f.lists != lists+1 {
		t.Fatalf("a Changed signal listed %d times, want once", f.lists-lists)
	}
	footer.close()
	if f.watching() != 0 {
		t.Fatal("a hidden page still follows the store")
	}
	if f.lists != lists+1 {
		t.Fatal("a hidden page listed")
	}
}

func resourceHeader(t *testing.T, root widget.Widget, key string) *widget.Item {
	t.Helper()
	for _, it := range treetest.All[*widget.Item](root) {
		if slices.Contains(treetest.Labels(it), locale.T().Get(key)) {
			return it
		}
	}
	t.Fatalf("no row for %s", key)
	return nil
}

func TestTheGrantsReadByResourceWithHowManyAppsHoldEach(t *testing.T) {
	k := testKit(t, "")
	apps := append(appsWithSwitches(), permissions.App{ID: "org.cam", Grants: []permissions.Grant{
		{Table: permissions.Devices, ID: "camera", Value: "yes"},
		{Table: permissions.Devices, ID: "projector", Value: "no"},
	}})
	serveGrants(t, &fakeGrants{apps: apps})
	footer := grantsFooter(k).(*grantsList)
	tr := locale.T()
	count := func(n int64) string { return tr.Get("settings-privacy-resource-apps", i18n.Int("count", n)) }
	for key, want := range map[string]string{
		"settings-privacy-resource-camera":     count(1),
		"settings-privacy-resource-microphone": count(0),
		"settings-privacy-table-location":      count(0),
		"settings-privacy-table-background":    count(2),
		"settings-privacy-table-devices":       count(1),
	} {
		if labels := treetest.Labels(resourceHeader(t, footer, key)); !slices.Contains(labels, want) {
			t.Errorf("%s reads %q, want %q", key, labels, want)
		}
	}
	labels := treetest.Labels(footer)
	if slices.Contains(labels, tr.Get("settings-privacy-table-wallpaper")) || slices.Contains(labels, "org.other") {
		t.Fatalf("collapsed, a resource no app holds is listed or an app shows:\n%s", strings.Join(labels, "\n"))
	}
	resourceHeader(t, footer, "settings-privacy-table-background").ClickAt(widget.Point{})
	labels = treetest.Labels(footer)
	at := slices.Index(labels, tr.Get("settings-privacy-table-background"))
	if at < 0 || !slices.Contains(labels[at:], "org.app") || !slices.Contains(labels[at:], "org.other") || slices.Contains(labels, "org.cam") {
		t.Fatalf("background open:\n%s", strings.Join(labels, "\n"))
	}
	if n := len(treetest.All[*widget.Switch](footer)); n != 2 {
		t.Fatalf("%d switches under background, want one per app", n)
	}
	resourceHeader(t, footer, "settings-privacy-table-background").ClickAt(widget.Point{})
	if labels := treetest.Labels(footer); slices.Contains(labels, "org.other") {
		t.Fatalf("closed again, background still lists its apps:\n%s", strings.Join(labels, "\n"))
	}
	resourceHeader(t, footer, "settings-privacy-resource-microphone").ClickAt(widget.Point{})
	if n := len(treetest.All[*widget.Switch](footer)) + len(revokeButtons(footer)); n != 0 {
		t.Fatalf("a resource no app holds opened %d controls", n)
	}
}

func TestEveryGrantBelongsToAResource(t *testing.T) {
	for _, table := range permissions.Tables {
		for _, id := range []string{"camera", "microphone", "speakers", "token", ""} {
			if _, ok := resourceOf(permissions.Grant{Table: table, ID: id}); !ok {
				t.Errorf("a %s grant of %q belongs to no resource", table, id)
			}
		}
	}
	if r, _ := resourceOf(permissions.Grant{Table: permissions.Devices, ID: "camera"}); r != cameraResource {
		t.Errorf("a camera grant belongs to %s", r.key)
	}
	if r, _ := resourceOf(permissions.Grant{Table: permissions.Devices, ID: "projector"}); r.key != "settings-privacy-table-devices" {
		t.Errorf("an unknown device belongs to %s, want the devices table", r.key)
	}
	if _, ok := resourceOf(permissions.Grant{Table: "no-such-table"}); ok {
		t.Error("a table the store does not have belongs to a resource")
	}
}

func TestTabReachesEveryGrantSwitch(t *testing.T) {
	k := testKit(t, "")
	serveGrants(t, &fakeGrants{apps: appsWithSwitches()})
	cfg := k.store.svc.Config()
	w := newWindow(k, cfg, func(_ time.Duration, fn func()) { fn() })
	w.stack.SetTransition(widget.StackNone, 0)
	apptheme.New(cfg).Attach(w.root)
	w.show(settingspage.PrivacyID)
	treetest.First[*grantsList](t, w.page).view.SetSelected(grantsByApp)
	w.root.Arrange(render.Rect{W: 1400, H: 6000})
	router := &widget.Router{Root: w.root}
	if err := router.SetFocus(w.search); err != nil {
		t.Fatal(err)
	}
	visited := map[widget.Widget]bool{}
	for range maxTabStops {
		router.FocusNext()
		if router.Focused() == widget.Widget(w.search) {
			break
		}
		visited[router.Focused()] = true
	}
	switches := treetest.All[*widget.Switch](w.page)
	if len(switches) != 3 {
		t.Fatalf("%d switches", len(switches))
	}
	for _, s := range switches {
		if !visited[s] {
			t.Errorf("the switch %q is out of Tab's reach", s.AccessibleName())
		}
	}
}

type countingPermissionStore struct {
	mu      sync.Mutex
	lists   map[string]int
	lookups int
}

func (c *countingPermissionStore) List(table string) ([]string, *dbus.Error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lists[table]++
	if table == string(permissions.Background) {
		return []string{"background"}, nil
	}
	return []string{}, nil
}

func (c *countingPermissionStore) Lookup(string, string) (map[string][]string, dbus.Variant, *dbus.Error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lookups++
	return map[string][]string{"org.app": {"yes"}}, dbus.MakeVariant(byte(0)), nil
}

func (c *countingPermissionStore) counts() (map[string]int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return maps.Clone(c.lists), c.lookups
}

func TestShowingThePageListsEachTableOnceAndAHiddenOneNothing(t *testing.T) {
	bus := dbustest.Start(t)
	service := bus.Conn(t)
	store := &countingPermissionStore{lists: map[string]int{}}
	if _, err := dbusx.Serve(service, dbusx.Service{Name: permissions.StoreName, Path: permissions.StorePath, Interface: permissions.StoreIface, Methods: store}); err != nil {
		t.Fatal(err)
	}
	before := openGrants
	openGrants = func() (grantStore, func(), error) {
		conn, err := dbus.Connect(bus.Address)
		if err != nil {
			return nil, nil, err
		}
		return permissions.Store{Conn: conn}, func() { _ = conn.Close() }, nil
	}
	t.Cleanup(func() { openGrants = before })
	k := testKit(t, "")
	footer := grantsFooter(k).(*grantsList)
	lists, lookups := store.counts()
	for _, table := range permissions.Tables {
		want := 1
		if table == permissions.Shortcuts {
			want = 0
		}
		if lists[string(table)] != want {
			t.Errorf("%s listed %d times, want %d", table, lists[string(table)], want)
		}
	}
	if lookups != 1 {
		t.Errorf("%d lookups for one entry", lookups)
	}
	footer.close()
	footer.reload()
	if again, more := store.counts(); !maps.Equal(again, lists) || more != lookups {
		t.Fatalf("a hidden page asked the store again: %v %d", again, more)
	}
}

func TestAGrantNamesTheAppReadablyEvenWithoutAnID(t *testing.T) {
	l := &grantsList{}
	for id, want := range map[string]string{
		"":              locale.T().Get("settings-privacy-host-apps"),
		unidentifiedApp: locale.T().Get("settings-privacy-unknown-app"),
		"org.app":       "org.app",
	} {
		if got := l.appTitle(id); got != want {
			t.Errorf("%q reads %q, want %q", id, got, want)
		}
	}
}

func TestAGrantReadsAsWordsNotTheStoresValues(t *testing.T) {
	tr := locale.T()
	word := func(v string) string { return tr.Get("settings-privacy-value", i18n.Str("value", v)) }
	lastUsed := tr.Get("settings-privacy-last-used", i18n.Str("date", time.Unix(1700000000, 0).Format(time.DateOnly)))
	for _, c := range []struct {
		grant permissions.Grant
		want  string
	}{
		{permissions.Grant{Table: permissions.Background, Value: "yes"}, word("yes")},
		{permissions.Grant{Table: permissions.Wallpaper, Value: "no,ask"}, word("no") + ", " + word("ask")},
		{permissions.Grant{Table: permissions.Location, Value: "EXACT,1700000000"}, word("exact") + ", " + lastUsed},
		{permissions.Grant{Table: permissions.Inhibit, Value: "logout,suspend"}, word("logout") + ", " + word("suspend")},
		{permissions.Grant{Table: permissions.Shortcuts, Value: "Toggle recording"}, "Toggle recording"},
		{permissions.Grant{Table: permissions.USB, Value: "1234"}, "1234"},
	} {
		if got := grantWords(c.grant); got != c.want {
			t.Errorf("%s %q reads %q, want %q", c.grant.Table, c.grant.Value, got, c.want)
		}
	}
	if word("yes") == "yes" || word("no,ask") != "no,ask" {
		t.Errorf("the value words are not translated, or an unknown value is lost: %q, %q", word("yes"), word("no,ask"))
	}
}

func TestPrivacyLinksToWhatTheLockScreenShowsAndWhatFileSearchReads(t *testing.T) {
	k := testKit(t, "")
	k.nav = layout(k.store.svc.Config(), bothParts)
	var opened []string
	k.openPage = func(id string) { opened = append(opened, id) }
	for _, b := range treetest.All[*widget.Button](linksSection(k, "settings-section-related", privacyLinks)) {
		b.OnClick()
	}
	if !slices.Equal(opened, []string{"lock", "launcher-modes", "media"}) {
		t.Errorf("the related links opened %q", opened)
	}
	k.nav = nil
	if labels := treetest.Labels(linksSection(k, "settings-section-related", privacyLinks)); len(labels) != 0 {
		t.Errorf("without the pages the links read %q", labels)
	}
}
