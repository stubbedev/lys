package settings

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/displays"
	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestANewVirtualDisplayTakesTheLowestFreeName(t *testing.T) {
	if got := nextVirtualName(nil); got != "VIRT-1" {
		t.Errorf("with none taken, %s", got)
	}
	if got := nextVirtualName([]string{"VIRT-1", "VIRT-3", "DP-1"}); got != "VIRT-2" {
		t.Errorf("with VIRT-1 and VIRT-3 taken, %s", got)
	}
}

func TestAVirtualDisplaysSizeReadsAsWords(t *testing.T) {
	if got := unisolated(virtualModeLabel("1920x1080@60")); got != "1920 × 1080, 60 Hz" {
		t.Errorf("1920x1080@60 reads %q", got)
	}
	if got := virtualModeLabel("1920x1080"); got != "1920x1080" {
		t.Errorf("a mode without a rate reads %q, want it as written", got)
	}
}

func TestAVirtualDisplayIsEditedAndRemovedByName(t *testing.T) {
	_, w, _, files := wmKit(t, "virtual_outputs:\n  - {name: VIRT-1, mode: 1920x1080@60}\n  - {name: VIRT-2, mode: 1280x720}\n")
	if got := virtualNames(w); !slices.Equal(got, []string{"VIRT-1", "VIRT-2"}) {
		t.Fatalf("the virtual displays %v", got)
	}
	s := virtualSlot(w, "VIRT-2")
	if err := nestedSlot(s, []string{"mode"}).set("2560x1440@60"); err != nil {
		t.Fatal(err)
	}
	if got := fileText(t, files.runtime); !strings.Contains(got, "2560x1440@60") || !strings.Contains(got, "VIRT-1") {
		t.Fatalf("resizing VIRT-2 wrote %q", got)
	}
	s.unset()
	if got := virtualNames(w); !slices.Equal(got, []string{"VIRT-1"}) {
		t.Fatalf("after removing VIRT-2: %v", got)
	}
	if err := virtualSlot(w, "VIRT-9").set(map[string]any{"name": "VIRT-9"}); err == nil {
		t.Error("a display the config does not define was written")
	}
	virtualSlot(w, "VIRT-1").unset()
	if got, _ := w.value(virtualOutputsPath).([]any); len(got) != 0 {
		t.Errorf("removing the last virtual display left %v", got)
	}
}

func TestAVirtualDisplayIsAddedToTheArrangementSizedAndRemoved(t *testing.T) {
	r := startDisplays(t)
	r.p.addVirtual.OnClick()
	r.l.until("lys-wm to add the virtual display", func() bool {
		return slices.ContainsFunc(r.p.heads, func(h displays.Head) bool { return h.Name == "VIRT-1" })
	})
	at := slices.IndexFunc(r.p.draft, func(s displays.Setting) bool { return s.Name == "VIRT-1" })
	mark := locale.T().Get("settings-displays-virtual-mark")
	for i, h := range r.p.canvas.handles {
		marked := strings.Contains(h.AccessibleName(), ", "+mark)
		if want := r.p.canvas.index[i] == at; marked != want {
			t.Errorf("the tile %q is marked virtual %v, want %v", h.AccessibleName(), marked, want)
		}
	}
	r.p.selectOutput(at)
	tr := locale.T()
	labels := treetest.Labels(r.p.extra)
	if !slices.Contains(labels, tr.Get("settings-displays-virtual-mode")) || len(buttonsTitled(r.p.extra, tr.Get("settings-displays-virtual-remove"))) != 1 {
		t.Fatalf("the virtual display's rows: %v", labels)
	}
	if err := nestedSlot(virtualSlot(r.k.wm, "VIRT-1"), []string{"mode"}).set(virtualModes[0]); err != nil {
		t.Fatal(err)
	}
	r.l.until("lys-wm to resize the virtual display", func() bool {
		o, ok := r.outputs(t)["VIRT-1"]
		return ok && o.Rect.Width == 1280
	})
	others := slices.IndexFunc(r.p.draft, func(s displays.Setting) bool { return s.Name == secondOutput })
	r.p.selectOutput(others)
	if len(buttonsTitled(r.p.extra, tr.Get("settings-displays-virtual-remove"))) != 0 {
		t.Error("a real display offers to be removed as a virtual one")
	}
	r.p.selectOutput(slices.IndexFunc(r.p.draft, func(s displays.Setting) bool { return s.Name == "VIRT-1" }))
	removes := treetest.WithClass(r.p.extra, css.DestructiveAction)
	if len(removes) != 1 {
		t.Fatalf("%d remove buttons", len(removes))
	}
	removes[0].(*widget.Button).OnClick()
	r.l.until("lys-wm to remove the virtual display", func() bool {
		return !slices.ContainsFunc(r.p.heads, func(h displays.Head) bool { return h.Name == "VIRT-1" })
	})
	if strings.Contains(string(r.config(t)), virtualOutputsPath) {
		t.Errorf("the config still holds %s:\n%s", virtualOutputsPath, r.config(t))
	}
}
