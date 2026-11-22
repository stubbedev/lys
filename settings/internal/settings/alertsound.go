package settings

import (
	"context"
	"fmt"
	"log"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/lib/pulse/native"
	"github.com/stubbedev/lys/lib/xdg"
	"github.com/stubbedev/lys/shell/service/sounds"
	"github.com/stubbedev/lys/ui/gtksettings"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func soundNameRow(r *rowSpec) {
	r.fullWidth = true
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
		return newAlertSoundList(k, s, systemLists[listSoundNames])
	}
}

var lookupAlertSound = func(name string) (string, error) {
	gtk, err := gtksettings.Load(xdg.OS)
	if err != nil {
		return "", fmt.Errorf("read the sound theme setting: %w", err)
	}
	return sounds.NewTheme(gtk.SoundThemeName, mime.DataDirs()).Lookup(name)
}

type alertSound struct {
	name  string
	radio *widget.CheckButton
	play  *widget.Button
}

type alertSoundList struct {
	*widget.Box
	k       *kit
	slot    slot
	list    lister
	rows    *widget.ItemGroup
	sounds  []alertSound
	state   *cardState
	problem *widget.Label
	syncing bool
}

func newAlertSoundList(k *kit, s slot, list lister) *alertSoundList {
	e := &alertSoundList{Box: widget.NewBox(widget.Column, widget.Current().Spacing/2, 0), k: k, slot: s, list: list}
	e.AddClass("alert-sound-list")
	e.rows = widget.NewItemGroup()
	e.state = k.cardState()
	scroll := widget.NewScroll(e.rows)
	scroll.VerticalOnly = true
	scroll.SetMaxContentHeight(choiceListPx)
	e.problem = k.errorText("")
	e.problem.SetVisible(false)
	e.Append(scroll, false)
	e.Append(e.problem, false)
	e.build(alertSoundNames(list(), s.text()))
	e.refresh()
	return e
}

func alertSoundNames(listed []choice, current string) []string {
	names := make([]string, 0, len(listed)+1)
	for _, c := range listed {
		names = append(names, c.value)
	}
	if current != "" && !slices.Contains(names, current) {
		names = append([]string{current}, names...)
	}
	return names
}

func (e *alertSoundList) refresh() {
	current := e.slot.text()
	names := alertSoundNames(e.list(), current)
	if !slices.EqualFunc(names, e.sounds, func(n string, s alertSound) bool { return n == s.name }) {
		e.build(names)
	}
	e.syncing = true
	defer func() { e.syncing = false }()
	for _, s := range e.sounds {
		s.radio.SetChecked(s.name == current)
	}
}

func (e *alertSoundList) build(names []string) {
	e.rows.Clear()
	e.rows.Append(e.state)
	e.sounds = e.sounds[:0]
	for _, name := range names {
		radio := widget.NewCheckButton(false)
		radio.SetAccessibleName(name)
		if len(e.sounds) > 0 {
			radio.SetGroup(e.sounds[0].radio)
		}
		radio.OnChanged = func(on bool) {
			if on && !e.syncing {
				e.choose(name)
			}
		}
		play := e.k.iconButton("ld-play-symbolic", "settings-sound-alert-play", func() { e.preview(name) })
		play.SetAccessibleName(locale.T().Get("settings-sound-alert-play") + ": " + name)
		row := e.k.row(name, "").withControl(play)
		row.SetMedia(radio, true)
		row.SetOnActivate(func() { radio.SetChecked(true) })
		e.rows.Append(row)
		e.sounds = append(e.sounds, alertSound{name: name, radio: radio, play: play})
	}
	if len(e.sounds) == 0 {
		e.state.Empty("ld-audio-lines-symbolic", locale.T().Get("settings-sound-alert-none"))
		return
	}
	e.state.Clear()
}

func (e *alertSoundList) choose(name string) {
	err := e.slot.set(name)
	e.problem.SetVisible(err != nil)
	if err != nil {
		e.problem.SetText(err.Error())
		e.refresh()
	}
}

func (e *alertSoundList) preview(name string) {
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
