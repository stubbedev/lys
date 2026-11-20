package settings

import (
	"reflect"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func touchList(t *testing.T, p *settingsPage) *cardList {
	t.Helper()
	c, ok := rowOf(t, p, touchGesturesPath).ctl.(*cardList)
	if !ok {
		t.Fatalf("the %s row holds %T, want gesture cards", touchGesturesPath, rowOf(t, p, touchGesturesPath).ctl)
	}
	return c
}

func touchControls(t *testing.T, c *cardList, i int) (*touchStart, *touchTarget) {
	t.Helper()
	starts, targets := treetest.All[*touchStart](c.cards[i].root), treetest.All[*touchTarget](c.cards[i].root)
	if len(starts) != 1 || len(targets) != 1 {
		t.Fatalf("card %d holds %d start and %d target controls", i, len(starts), len(targets))
	}
	return starts[0], targets[0]
}

func TestATouchSwipeIsBuiltFromPicturesAndAnAction(t *testing.T) {
	k := lysWMKit(t, "")
	p := layoutPage(t, k, "wm-pointer")
	c := touchList(t, p)
	c.appendItem(map[string]any{})
	start, target := touchControls(t, c, 0)
	if start.swipe.Visible() {
		t.Error("a gesture that starts nowhere yet shows the finger count")
	}
	start.starts.choose("bottom")
	target.mode.SetActive(touchFollow)
	target.drives.choose("overview")
	want := []any{map[string]any{"edge": "bottom", "drive": "overview"}}
	if got := k.wm.value(touchGesturesPath); !reflect.DeepEqual(got, want) {
		t.Fatalf("an edge swipe driving the overview wrote %v, want %v", got, want)
	}
	tr := locale.T()
	if got := c.cards[0].title.Text(); got != tr.Get("wm-gestures-touch-edge-title", i18n.Str("edge", "bottom")) {
		t.Errorf("the card reads %q", got)
	}
	start.starts.choose(touchFingers)
	if !start.swipe.Visible() || start.count.slot.get() != int64(3) {
		t.Fatalf("swiping with fingers shows the count %v reading %v, want lys-wm's fewest 3", start.swipe.Visible(), start.count.slot.get())
	}
	start.directions.choose("vertical")
	want = []any{map[string]any{"fingers": 3, "direction": "vertical", "drive": "overview"}}
	if got := k.wm.value(touchGesturesPath); !reflect.DeepEqual(got, want) {
		t.Fatalf("a three-finger vertical swipe wrote %v, want %v", got, want)
	}
	target.mode.SetActive(touchRun)
	if target.drives.Visible() || !target.run.Visible() {
		t.Error("running an action still shows the drives")
	}
	if _, held := c.local[0]["drive"]; held {
		t.Error("running an action keeps the drive, which lys-wm refuses beside an action")
	}
	target.run.choose("kill")
	want = []any{map[string]any{"fingers": 3, "direction": "vertical", "action": "kill"}}
	if got := k.wm.value(touchGesturesPath); !reflect.DeepEqual(got, want) {
		t.Fatalf("a swipe closing the window wrote %v, want %v", got, want)
	}
	start.starts.choose("left")
	if _, held := c.local[0]["direction"]; held {
		t.Error("an edge swipe keeps a direction, which lys-wm refuses")
	}
}

func TestATouchSwipeLysWMRefusesIsHeldAsADraft(t *testing.T) {
	k := lysWMKit(t, "")
	c := touchList(t, layoutPage(t, k, "wm-pointer"))
	c.appendItem(map[string]any{})
	start, _ := touchControls(t, c, 0)
	start.starts.choose(touchFingers)
	if !c.draft.Visible() {
		t.Error("a swipe with no direction and nothing to do shows no reason it is not written")
	}
	if got := k.wm.value(touchGesturesPath); got != nil {
		t.Errorf("a swipe lys-wm refuses was written: %v", got)
	}
}

func TestEveryTouchSwipeChoiceHasAPicture(t *testing.T) {
	k := lysWMKit(t, "")
	for key, known := range map[string]func(string) bool{
		"edge":      func(v string) bool { _, ok := edgeInward[v]; return ok },
		"direction": func(v string) bool { _, ok := directionArrows[v]; return ok },
		"drive":     func(v string) bool { _, ok := drivePictures[v]; return ok },
	} {
		values := touchChoices(k.wm, key)
		if len(values) == 0 {
			t.Errorf("lys-wm offers no %s", key)
		}
		for _, v := range values {
			if !known(v) {
				t.Errorf("lys-wm's %s %q has no picture", key, v)
			}
			if label := touchLabels(key, []string{v})[0]; label == v {
				t.Errorf("lys-wm's %s %q has no label", key, v)
			}
		}
	}
	defer func() {
		if recover() == nil {
			t.Error("a choice with no picture was drawn")
		}
	}()
	touchPictures([]string{"diagonal"}, drawEdgeSwipe, func(v string) bool { _, ok := edgeInward[v]; return ok })
}

func TestTheTouchPictureDrawsTheGestureItShows(t *testing.T) {
	const w, h = touchPictureW, touchPictureH
	paint := func(item map[string]any) []byte {
		p := newTouchPicture()
		p.show(item)
		data := make([]byte, render.Stride(w)*h)
		p.OnDraw(render.New(data, render.Stride(w), w, h), render.Rect{W: w, H: h})
		return data
	}
	bare := paint(map[string]any{})
	for _, item := range []map[string]any{{"edge": "top"}, {"fingers": 4, "direction": "left"}} {
		if slices.Equal(paint(item), bare) {
			t.Errorf("%v draws an empty screen", item)
		}
	}
	if !slices.Equal(paint(map[string]any{"edge": "middle"}), bare) {
		t.Error("an edge lys-wm has not got is drawn")
	}
}
