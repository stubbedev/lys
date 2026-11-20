package settings

import (
	"context"
	"fmt"
	"log"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/lib/pulse/native"
	"github.com/stubbedev/lys/lib/xdg"
	"github.com/stubbedev/lys/shell/service/sounds"
	"github.com/stubbedev/lys/ui/gtksettings"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

var soundNameRow = withEditor(func(k *kit, s slot, meta config.FieldMeta) control {
	return newAlertSoundEditor(k, s, newChoiceEditor(k, s, choiceSpec{list: systemLists[listSoundNames], special: meta.Variants, free: true}))
})

var lookupAlertSound = func(name string) (string, error) {
	gtk, err := gtksettings.Load(xdg.OS)
	if err != nil {
		return "", fmt.Errorf("read the sound theme setting: %w", err)
	}
	return sounds.NewTheme(gtk.SoundThemeName, mime.DataDirs()).Lookup(name)
}

type alertSoundEditor struct {
	*widget.Box
	k       *kit
	slot    slot
	name    *choiceEditor
	play    *widget.Button
	problem *widget.Label
}

func newAlertSoundEditor(k *kit, s slot, name *choiceEditor) *alertSoundEditor {
	e := &alertSoundEditor{Box: widget.NewBox(widget.Column, widget.Current().Spacing/2, 0), k: k, slot: s, name: name}
	e.play = k.iconButton("ld-play-symbolic", "settings-sound-alert-play", e.preview, css.Flat)
	e.play.SetAccessibleName(locale.T().Get("settings-sound-alert-play"))
	e.problem = wrapped(k.label("", css.Error, css.Caption))
	e.problem.SetVisible(false)
	line := gapBox(widget.Row)
	line.AppendAligned(name, true, widget.AlignCenter)
	line.AppendAligned(e.play, false, widget.AlignCenter)
	e.Append(line, false)
	e.Append(e.problem, false)
	return e
}

func (e *alertSoundEditor) refresh() { e.name.refresh() }

func (e *alertSoundEditor) preview() {
	name, _ := e.slot.get().(string)
	volume := e.k.store.svc.Config().Notification.SoundPolicy().Volume
	async(e.k, func() error { return playAlertSound(name, volume) }, func(err error) {
		e.problem.SetVisible(err != nil)
		if err != nil {
			log.Printf("settings: play the alert sound %q: %v", name, err)
			e.problem.SetText(locale.T().Get("settings-sound-alert-play-failed") + ": " + err.Error())
		}
	})
}

func playAlertSound(name string, volume float64) error {
	path, err := lookupAlertSound(name)
	if err != nil {
		return fmt.Errorf("find the sound %q: %w", name, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), soundTimeout)
	defer cancel()
	svc, err := openPulse(ctx)
	if err != nil {
		return fmt.Errorf("%s: %w", locale.T().Get("settings-sound-unreachable"), err)
	}
	defer func() { _ = svc.Close() }()
	play := native.SamplePlay{
		Name:   "lys-settings:alert:" + path,
		Volume: native.LinearVolume(volume),
		Props:  native.PropList{"media.role": "event", "event.id": name, "media.name": name, "application.name": "lys-settings"},
	}
	return svc.PlaySample(ctx, play, func() (native.Sample, error) { return sounds.Decode(path) })
}
