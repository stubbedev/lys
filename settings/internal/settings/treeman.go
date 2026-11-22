package settings

import (
	"os/exec"

	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const treemanBinary = "treeman"

// treemanInstalled reports whether the treeman CLI is on PATH (a seam
// for tests).
var treemanInstalled = func() bool {
	_, err := exec.LookPath(treemanBinary)
	return err == nil
}

func treemanNote(k *kit) widget.Widget {
	t := locale.T()
	box := gapBox(widget.Column)
	box.Append(k.note(t.Get("settings-treeman-about"), "settings-section-description"), false)
	if treemanInstalled() {
		return box
	}
	box.Append(wrapped(k.label(t.Get("settings-treeman-missing"), css.Caption, css.Warning)), false)
	return box
}
