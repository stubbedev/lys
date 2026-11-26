package notifyview

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
)

func TestAnAttachmentIsTheImagePathTheIconShows(t *testing.T) {
	for _, c := range []struct {
		icon      Icon
		imagePath string
		want      string
	}{
		{Icon{File: "/tmp/shot.png"}, "/tmp/shot.png", "/tmp/shot.png"},
		{Icon{File: "/tmp/shot.png"}, "file:///tmp/shot.png", "/tmp/shot.png"},
		{Icon{File: "/usr/share/icons/app.png"}, "", ""},
		{Icon{File: "/usr/share/icons/app.png"}, "/tmp/shot.png", ""},
		{Icon{Name: "mail"}, "/tmp/shot.png", ""},
	} {
		if got := Attachment(c.icon, c.imagePath); got != c.want {
			t.Errorf("Attachment(%+v, %q) = %q, want %q", c.icon, c.imagePath, got, c.want)
		}
	}
}

// A card with an attachment and a look has a look button that looks
// at it without opening the card; a card without either has none.
func TestALookButtonLooksAtTheAttachment(t *testing.T) {
	const path = "/tmp/shot.png"
	for _, c := range []struct {
		name       string
		attachment string
		look       bool
	}{
		{"attachment and look", path, true},
		{"no attachment", "", true},
		{"no look", path, false},
	} {
		var looked []string
		opened := 0
		h := Handlers{
			Close: func() {}, Action: func(int) {}, Reply: func(string) {}, ReplyFocus: func(bool) {}, Link: func(string) {},
			Post: func(func()) {},
			Open: func() { opened++ },
		}
		if c.look {
			h.Look = func(p string) { looked = append(looked, p) }
		}
		content := sampleContent()
		content.Icon = Icon{File: path}
		content.Attachment = c.attachment
		k := NewCard(config.Defaults(), treetest.Font(t), cardNow, content, h)
		buttons := treetest.WithClass(k.Root, "notification-look")
		if c.attachment == "" || !c.look {
			if len(buttons) != 0 {
				t.Errorf("%s: %d look buttons, want none", c.name, len(buttons))
			}
			continue
		}
		if len(buttons) != 1 {
			t.Fatalf("%s: %d look buttons, want one", c.name, len(buttons))
		}
		r, p := treetest.Pointer(k.Root, buttons[0].(widget.Boundser), 400, 300)
		r.Press(widget.BTNLeft, p, 0)
		r.Release(widget.BTNLeft, p)
		if !slices.Equal(looked, []string{path}) || opened != 0 {
			t.Errorf("%s: the look button looked at %q and opened the card %d times", c.name, looked, opened)
		}
	}
}
