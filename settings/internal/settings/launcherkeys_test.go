package settings

import (
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

func TestEveryLauncherActionHasALabel(t *testing.T) {
	for _, b := range append(append([]config.LauncherBinding{}, config.LauncherDefaultKeybindings...), config.LauncherDefaultMouseBindings...) {
		if got := launcherActionLabel(b.Action); got == "" || strings.HasPrefix(got, "settings-") || got == b.Action {
			t.Errorf("%s reads %q", b.Action, got)
		}
	}
}

func TestALauncherKeyIsRecordedAndResetToTheDefault(t *testing.T) {
	const path = "launcher.keybindings"
	k := testKit(t, "")
	c := fieldControl(k, k.store, field(path, launcherKeysRow)).(*launcherBindings)
	recorders := treetest.All[*widget.ShortcutRecorder](c)
	if len(recorders) != len(config.LauncherDefaultKeybindings) {
		t.Fatalf("%d recorders for %d actions", len(recorders), len(config.LauncherDefaultKeybindings))
	}
	if recorders[0].Accel() != "Return" {
		t.Fatalf("the first action shows %q, want its default Return", recorders[0].Accel())
	}
	recorders[0].OnChanged("Ctrl+J")
	got, _ := k.store.value(path).(map[string]any)
	if got[config.LauncherDefaultKeybindings[0].Action] != "Control+j" {
		t.Fatalf("recording stored %v", got)
	}
	resets := 0
	for _, b := range c.resets {
		if widget.IsEnabled(b.Button) {
			resets++
			b.OnClick()
		}
	}
	if resets != 1 {
		t.Fatalf("%d reset buttons show with one override", resets)
	}
	if got, _ := k.store.value(path).(map[string]any); len(got) != 0 {
		t.Errorf("reset left %v", got)
	}
}

func TestALauncherMouseActionIsPickedFromTheButtons(t *testing.T) {
	const path = "launcher.mouse-bindings"
	k := testKit(t, "")
	c := fieldControl(k, k.store, field(path, launcherMouseRow)).(*launcherBindings)
	picks := treetest.All[*widget.Dropdown](c)
	if len(picks) != len(config.LauncherDefaultMouseBindings) {
		t.Fatalf("%d pickers for %d actions", len(picks), len(config.LauncherDefaultMouseBindings))
	}
	picks[0].OnSelect(2)
	got, _ := k.store.value(path).(map[string]any)
	if got[config.LauncherDefaultMouseBindings[0].Action] != launcherMouseTokens[2] {
		t.Errorf("picking stored %v", got)
	}
}

func TestALauncherActionsFurtherKeysReadUnderItsName(t *testing.T) {
	k := testKit(t, "launcher:\n  keybindings:\n    cancel: \"Escape,Control+q,Control+bracketleft\"\n")
	c := fieldControl(k, k.store, field("launcher.keybindings", launcherKeysRow)).(*launcherBindings)
	labels := strings.Join(treetest.Labels(c), "|")
	if !strings.Contains(labels, "Close the launcher|||Also|Ctrl|Q|Ctrl|[|Esc") {
		t.Errorf("cancel's further keys are not shown as keycaps under it: %q", labels)
	}
	if strings.Contains(labels, "bracketleft") || strings.Contains(labels, "Control") {
		t.Errorf("a further key shows its stored name: %q", labels)
	}
	k2 := testKit(t, "")
	c2 := fieldControl(k2, k2.store, field("launcher.keybindings", launcherKeysRow)).(*launcherBindings)
	if strings.Contains(strings.Join(treetest.Labels(c2), "|"), "Close the launcher|||Also|Ctrl|Q") {
		t.Error("an action without the override shows the override's key")
	}
}
