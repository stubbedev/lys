package viewer

import (
	"fmt"
	"log"
	"time"

	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/media"
	"github.com/stubbedev/lys/ui/widget"
)

func (v *Viewer) Key(a app.Accel) bool {
	if v.doc.typing > 0 && !v.whileTyping[a] {
		return false
	}
	act, ok := v.keys[a]
	return ok && act()
}

var whileTyping = []string{"Escape", "F3", "shift+F3", "ctrl+f", "ctrl+g", "ctrl+shift+g", "F9", "F11"}

func (v *Viewer) bindings() map[app.Accel]func() bool {
	d := v.doc
	table := []struct {
		keys []string
		act  func() bool
	}{
		{[]string{"Left"}, func() bool { return d.scrollKey(-1, 0) || v.pan(widget.KeyLeft) || v.step(-1) }},
		{[]string{"Right"}, func() bool { return d.scrollKey(1, 0) || v.pan(widget.KeyRight) || v.step(1) }},
		{[]string{"Up"}, func() bool { return d.scrollKey(0, -1) || v.pan(widget.KeyUp) || v.volume(volumeStep) }},
		{[]string{"Down"}, func() bool { return d.scrollKey(0, 1) || v.pan(widget.KeyDown) || v.volume(-volumeStep) }},
		{[]string{"Prior"}, func() bool { return d.pageStep(-1) || v.step(-1) }},
		{[]string{"BackSpace"}, func() bool { return v.step(-1) }},
		{[]string{"Next"}, func() bool { return d.pageStep(1) || v.step(1) }},
		{[]string{"Home"}, func() bool { return d.firstPage() || v.jump(0) }},
		{[]string{"End"}, func() bool { return d.lastPage() || v.jump(v.files.End()) }},
		{[]string{"space"}, func() bool { return d.scrollScreen() || v.playOrNext() }},
		{[]string{"shift+Left"}, func() bool { return v.seek(-seekStep) }},
		{[]string{"shift+Right"}, func() bool { return v.seek(seekStep) }},
		{[]string{"m"}, v.mute},
		{[]string{"l"}, v.toggleLoop},
		{[]string{"F11"}, v.toggleFullscreen},
		{[]string{"Escape"}, func() bool { return d.endSearch() || v.leaveFullscreen() }},
		{[]string{"Delete"}, func() bool { v.trashCurrent(); return true }},
		{[]string{"ctrl+z"}, v.undoTrash},
		{[]string{"ctrl+c"}, func() bool {
			if !d.copySelection() {
				v.copyPicture()
			}
			return true
		}},
		{[]string{"ctrl+f"}, d.startSearch},
		{[]string{"F3", "ctrl+g"}, func() bool { return d.findNext(1) }},
		{[]string{"shift+F3", "ctrl+shift+g"}, func() bool { return d.findNext(-1) }},
		{[]string{"F9"}, d.toggleOutline},
		{[]string{"plus", "shift+plus", "equal", "KP_Add"}, func() bool { return d.zoom('+') || v.zoom('+') }},
		{[]string{"minus", "KP_Subtract"}, func() bool { return d.zoom('-') || v.zoom('-') }},
		{[]string{"0", "KP_0"}, func() bool { return d.zoom('0') || v.zoom('0') }},
		{[]string{"1", "KP_1"}, func() bool { return d.zoom('1') || v.zoom('1') }},
	}
	keys := make(map[app.Accel]func() bool, 2*len(table))
	for _, b := range table {
		for _, k := range b.keys {
			a, err := app.ParseAccel(k)
			if err != nil {
				panic(fmt.Sprintf("viewer: binding %q: %v", k, err))
			}
			if _, taken := keys[a]; taken {
				panic(fmt.Sprintf("viewer: %q is bound twice", k))
			}
			keys[a] = b.act
		}
	}
	v.whileTyping = make(map[app.Accel]bool, len(whileTyping))
	for _, k := range whileTyping {
		a, err := app.ParseAccel(k)
		if err != nil {
			panic(fmt.Sprintf("viewer: binding %q: %v", k, err))
		}
		v.whileTyping[a] = true
	}
	return keys
}

func (v *Viewer) picture() *widget.ImageView {
	if r := v.view.Renderer(); r != preview.Image && r != preview.Thumbnail {
		return nil
	}
	return v.view.Picture()
}

func (v *Viewer) pan(k widget.KeyAction) bool {
	p := v.picture()
	if p == nil || p.Fitted() {
		return false
	}
	p.KeyAction(k, 0)
	return true
}

func (v *Viewer) zoom(r rune) bool {
	p := v.picture()
	if p == nil {
		return false
	}
	p.InsertRune(r)
	return true
}

func (v *Viewer) playing(act func(*media.Stream)) bool {
	s := v.view.Stream()
	if s == nil {
		return false
	}
	act(s)
	return true
}

func (v *Viewer) playOrNext() bool {
	return v.playing(func(s *media.Stream) {
		if s.Playing() {
			s.Pause()
			return
		}
		s.Play()
	}) || v.step(1)
}

func (v *Viewer) volume(delta float64) bool {
	return v.playing(func(s *media.Stream) { s.SetVolume(s.Volume() + delta) })
}

func (v *Viewer) mute() bool {
	return v.playing(func(s *media.Stream) { s.SetMuted(!s.Muted()) })
}

func (v *Viewer) seek(delta time.Duration) bool {
	return v.playing(func(s *media.Stream) {
		if !s.Seekable() {
			return
		}
		to := max(s.Position()+delta, 0)
		if d := s.Duration(); d > 0 {
			to = min(to, d)
		}
		if err := s.Seek(to); err != nil {
			log.Printf("viewer: seek to %v: %v", to, err)
		}
	})
}

func (v *Viewer) toggleLoop() bool {
	return v.playing(func(*media.Stream) { v.loop.SetActive(!v.loop.Active()) })
}

func (v *Viewer) leaveFullscreen() bool {
	if !v.fullscreen {
		return false
	}
	v.setFullscreen(false)
	return true
}
