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
	act, ok := v.keys[a]
	return ok && act()
}

func (v *Viewer) bindings() map[app.Accel]func() bool {
	table := []struct {
		keys []string
		act  func() bool
	}{
		{[]string{"Left"}, func() bool { return v.pan(widget.KeyLeft) || v.step(-1) }},
		{[]string{"Right"}, func() bool { return v.pan(widget.KeyRight) || v.step(1) }},
		{[]string{"Up"}, func() bool { return v.pan(widget.KeyUp) || v.volume(volumeStep) }},
		{[]string{"Down"}, func() bool { return v.pan(widget.KeyDown) || v.volume(-volumeStep) }},
		{[]string{"Prior", "BackSpace"}, func() bool { return v.step(-1) }},
		{[]string{"Next"}, func() bool { return v.step(1) }},
		{[]string{"Home"}, func() bool { return v.jump(0) }},
		{[]string{"End"}, func() bool { return v.jump(v.files.End()) }},
		{[]string{"space"}, v.playOrNext},
		{[]string{"shift+Left"}, func() bool { return v.seek(-seekStep) }},
		{[]string{"shift+Right"}, func() bool { return v.seek(seekStep) }},
		{[]string{"m"}, v.mute},
		{[]string{"l"}, v.toggleLoop},
		{[]string{"F11"}, v.toggleFullscreen},
		{[]string{"Escape"}, v.leaveFullscreen},
		{[]string{"Delete"}, func() bool { v.trashCurrent(); return true }},
		{[]string{"ctrl+z"}, v.undoTrash},
		{[]string{"ctrl+c"}, func() bool { v.copyPicture(); return true }},
		{[]string{"plus", "shift+plus", "equal", "KP_Add"}, func() bool { return v.zoom('+') }},
		{[]string{"minus", "KP_Subtract"}, func() bool { return v.zoom('-') }},
		{[]string{"0", "KP_0"}, func() bool { return v.zoom('0') }},
		{[]string{"1", "KP_1"}, func() bool { return v.zoom('1') }},
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
