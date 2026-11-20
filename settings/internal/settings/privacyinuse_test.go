package settings

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/feed"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/pulse/pulsetest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/shell/service/privacy"
	"github.com/stubbedev/lys/shell/service/screenshare"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type fakeCameras struct {
	users    []privacy.CameraUser
	changes  *feed.Tick
	released int
}

func (f *fakeCameras) Users() []privacy.CameraUser          { return f.users }
func (f *fakeCameras) Subscribe() (<-chan struct{}, func()) { return f.changes.Subscribe() }

type fakeShares struct {
	shares   []screenshare.Share
	stopped  []dbus.ObjectPath
	stopErr  error
	changes  *feed.Tick
	released int
}

func (f *fakeShares) List() []screenshare.Share            { return f.shares }
func (f *fakeShares) Subscribe() (<-chan struct{}, func()) { return f.changes.Subscribe() }

func (f *fakeShares) Stop(_ context.Context, session dbus.ObjectPath) error {
	f.stopped = append(f.stopped, session)
	if f.stopErr != nil {
		return f.stopErr
	}
	f.shares = slices.DeleteFunc(f.shares, func(s screenshare.Share) bool { return s.Session == session })
	feed.Notify(f.changes)
	return nil
}

type inUseRig struct {
	srv     *pulsetest.Server
	k       *kit
	l       *loop
	cameras *fakeCameras
	shares  *fakeShares
}

func startInUse(t *testing.T, cameras []privacy.CameraUser, shares []screenshare.Share) *inUseRig {
	t.Helper()
	srv := soundServer(t)
	srv.PutSourceOutput(recorder(60, 10, "Recorder"))
	useSoundServer(t, srv)
	k := testKit(t, "")
	l := &loop{t: t, queue: make(chan func(), 64)}
	k.invoke = func(fn func()) { l.queue <- fn }
	r := &inUseRig{srv: srv, k: k, l: l, cameras: &fakeCameras{users: cameras, changes: feed.NewTick()}, shares: &fakeShares{shares: shares, changes: feed.NewTick()}}
	k.cameras = func() (cameraSource, func(), error) { return r.cameras, func() { r.cameras.released++ }, nil }
	k.screenShares = func() (shareSource, func(), error) { return r.shares, func() { r.shares.released++ }, nil }
	serveGrants(t, &fakeGrants{})
	return r
}

func TestInUseNowListsWhatCapturesTheMicrophoneCameraAndScreen(t *testing.T) {
	r := startInUse(t, []privacy.CameraUser{{PID: 42, Name: "cheese", Device: "/dev/video0"}},
		[]screenshare.Share{{Session: "/s/1", App: "com.obsproject.Studio", Kind: screenshare.KindScreenCast}})
	footer := grantsFooter(r.k).(*grantsList)
	tr := locale.T()
	r.l.until("the recorder listed", func() bool { return slices.Contains(treetest.Labels(footer.inUse), "Recorder") })
	labels := treetest.Labels(footer.inUse)
	for _, want := range []string{
		tr.Get("settings-privacy-in-use-microphone", i18n.Str("source", "Microphone")),
		"cheese", tr.Get("settings-privacy-in-use-camera", i18n.Str("device", "/dev/video0")),
		"com.obsproject.Studio", tr.Get("settings-privacy-in-use-screen"),
	} {
		if !slices.Contains(labels, want) {
			t.Errorf("the strip lacks %q:\n%s", want, strings.Join(labels, "\n"))
		}
	}
	if slices.Contains(labels, tr.Get("settings-privacy-in-use-none")) || footer.inUse.problem.Visible() {
		t.Fatalf("a busy strip reads empty or a problem:\n%s", strings.Join(labels, "\n"))
	}
	treetest.Button(t, footer.inUse, tr.Get("settings-privacy-in-use-stop-app", i18n.Str("app", "com.obsproject.Studio"))).OnClick()
	if !slices.Equal(r.shares.stopped, []dbus.ObjectPath{"/s/1"}) {
		t.Fatalf("stopped %v", r.shares.stopped)
	}
	r.l.until("the stopped share gone", func() bool {
		return !slices.Contains(treetest.Labels(footer.inUse), tr.Get("settings-privacy-in-use-screen"))
	})
	treetest.All[*muteToggle](footer.inUse)[0].OnClick()
	r.l.until("the recorder muted on the server", func() bool {
		st, _ := r.srv.SourceOutput(60)
		return st.Mute
	})
	footer.close()
	if r.cameras.released != 1 || r.shares.released != 1 || r.cameras.changes.Len() != 0 || r.shares.changes.Len() != 0 {
		t.Fatalf("closing released cameras %d, shares %d and left %d, %d subscribers", r.cameras.released, r.shares.released, r.cameras.changes.Len(), r.shares.changes.Len())
	}
}

func TestInUseNowFollowsACameraOpeningAndSaysWhenNothingCaptures(t *testing.T) {
	r := startInUse(t, nil, nil)
	r.k.screenShares = func() (shareSource, func(), error) { return nil, nil, errors.New("no portal") }
	footer := grantsFooter(r.k).(*grantsList)
	tr := locale.T()
	r.l.until("the recorder listed", func() bool { return slices.Contains(treetest.Labels(footer.inUse), "Recorder") })
	want := tr.Get("settings-privacy-in-use-no-screen") + ": no portal"
	if !footer.inUse.problem.Visible() || footer.inUse.problem.Text() != want {
		t.Fatalf("the problem reads %q, shown %v, want %q", footer.inUse.problem.Text(), footer.inUse.problem.Visible(), want)
	}
	r.cameras.users = []privacy.CameraUser{{PID: 7, Name: "zoom", Device: "/dev/video2"}}
	feed.Notify(r.cameras.changes)
	r.l.until("the camera listed", func() bool { return slices.Contains(treetest.Labels(footer.inUse), "zoom") })
	footer.close()
	if r.shares.released != 0 {
		t.Fatal("a share source that never opened was released")
	}
}

func TestInUseNowWithNothingCapturingSaysSo(t *testing.T) {
	k := testKit(t, "")
	serveGrants(t, &fakeGrants{})
	footer := grantsFooter(k).(*grantsList)
	labels := treetest.Labels(footer.inUse)
	if !slices.Contains(labels, locale.T().Get("settings-privacy-in-use-none")) {
		t.Fatalf("an idle strip reads:\n%s", strings.Join(labels, "\n"))
	}
	for _, key := range []string{"settings-privacy-in-use-no-microphone", "settings-privacy-in-use-no-camera", "settings-privacy-in-use-no-screen"} {
		if !strings.Contains(footer.inUse.problem.Text(), locale.T().Get(key)) {
			t.Errorf("the unreachable sources do not say %s: %q", key, footer.inUse.problem.Text())
		}
	}
	footer.close()
}
