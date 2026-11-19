package settings

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/login/dmsocket"
	"github.com/stubbedev/lys/login/greeterconfig"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func fontsPage(*config.Config) pageSpec {
	return pageSpec{
		id: "fonts", navKey: "settings-nav-fonts", icon: "ld-type-symbolic",
		header: "settings-page-fonts",
		sections: []sectionSpec{
			{title: "settings-section-fonts", rows: []rowSpec{field("general.font-sans", fontRow), field("general.font-mono", monoFontRow)}},
			{title: "settings-section-scale-rounding", rows: []rowSpec{field("styling.scale"), field("styling.rounding", roundingChoice)}},
		},
	}
}

// lockPage is pages/lock.
func lockPage(*config.Config) pageSpec {
	return pageSpec{
		id: "lock", navKey: "settings-nav-lock-page", icon: "ld-lock-symbolic",
		header: "settings-page-lock",
		sections: []sectionSpec{
			{title: "settings-section-general", rows: fields("login.lock.enabled", "login.lock.fingerprint",
				"login.lock.show-media", "login.lock.show-battery", "login.lock.show-switch-user")},
			{title: "settings-section-background", rows: []rowSpec{
				field("login.background-mode", pictureChoice(lockBackgroundPictures)),
				field("login.background-image", filePath, shownWhen("login.background-mode", valueIn(string(config.BackgroundImage)))),
				field("login.background-color", shownWhen("login.background-mode", valueIn(string(config.BackgroundColor)))),
				field("login.blur", softPixels(0, lockBlurSoftMax), stackedRow, shownWhen("login.background-mode", valueIn(string(config.BackgroundImage), string(config.BackgroundWallpaper)))),
			}},
			{title: "settings-section-clock", rows: []rowSpec{
				field("login.show-clock"),
				field("login.clock-format", shownWhen("login.show-clock", switchedOn)),
				field("login.date-format", shownWhen("login.show-clock", switchedOn)),
			}},
			{title: "settings-section-security", rows: []rowSpec{
				field("login.lock.grace-period-ms", durationChips(inMilliseconds, zeroAs(immediately), after(5*time.Second), after(30*time.Second), after(time.Minute), after(5*time.Minute))),
				field("login.max-attempts"), field("login.show-failed-attempts"),
				field("login.lock.blank-timeout-ms", durationChips(inMilliseconds, zeroAs(never), after(30*time.Second), after(time.Minute), after(5*time.Minute), after(10*time.Minute))),
			}},
			{title: "settings-section-lock-advanced", advanced: true, rows: fields("login.lock.pam-service", "login.lock.lock-on-start")},
			{title: "settings-section-animation", advanced: true, rows: surfaceAnimationRows("animations.lock")},
			{title: "settings-section-login-screen", rows: fields("login.greeter.show-user-list", "login.greeter.show-power-buttons", "login.greeter.fingerprint", "login.greeter.screen-reader")},
			{title: "settings-section-cursor", rows: fields("login.greeter.cursor-theme", "login.greeter.cursor-size")},
		},
		footer: greeterApplyFooter, footerOnRows: true,
	}
}

const lockBlurSoftMax = 100

var lockBackgroundPictures = map[string]picture{
	string(config.BackgroundColor): drawn(func(cv *render.Canvas, r render.Rect) {
		screenFrame(cv, r, func(s render.Rect) { cv.FillRect(s, previewDark) })
	}),
	string(config.BackgroundImage): drawn(func(cv *render.Canvas, r render.Rect) {
		screenFrame(cv, r, func(s render.Rect) { photo(cv, s) })
	}),
	string(config.BackgroundWallpaper): drawn(func(cv *render.Canvas, r render.Rect) {
		screenFrame(cv, r, func(s render.Rect) {
			photo(cv, s)
			cv.FillRect(render.Rect{X: s.X, Y: s.Y, W: s.W, H: max(3, s.H/8)}, previewDarkInk)
		})
	}),
}

const (
	greeterApplyLabel        = "settings-greeter-apply"
	greeterApplyHint         = "settings-greeter-apply-hint"
	greeterApplyDone         = "settings-greeter-apply-done"
	greeterApplyFailed       = "settings-greeter-apply-failed"
	greeterApplyNoDaemon     = "settings-greeter-apply-no-daemon"
	greeterApplyInvalid      = "settings-greeter-apply-invalid"
	greeterApplyUnauthorized = "settings-greeter-apply-unauthorized"
	greeterApplyDismissed    = "settings-greeter-apply-dismissed"
)

var loginRuntimeDir = dmsocket.DefaultRuntimeDir

func greeterApplyFooter(k *kit) widget.Widget {
	box := gapBox(widget.Column)
	box.AddClass("settings-section")
	t := locale.T()
	hint := k.label(t.Get(greeterApplyHint), "settings-section-title")
	hint.SetWrap(true)
	apply := func() {
		async(k, func() error { return applyGreeter(k.store, loginRuntimeDir) }, func(err error) {
			if err != nil {
				log.Printf("settings: greeter apply: %v", err)
			}
			hint.SetText(greeterApplyOutcome(t, err))
		})
	}
	box.AppendAligned(k.button(k.label(t.Get(greeterApplyLabel)), apply, css.SuggestedAction), false, widget.AlignStart)
	box.Append(hint, false)
	return box
}

func greeterApplyOutcome(t *i18n.Loader, err error) string {
	if err == nil {
		return t.Get(greeterApplyDone)
	}
	if errors.Is(err, dmsocket.ErrNoDaemon) {
		return t.Get(greeterApplyNoDaemon)
	}
	because := func(id, reason string) string { return t.Get(id, i18n.Str("reason", reason)) }
	if errors.Is(err, greeterconfig.ErrInvalid) {
		return because(greeterApplyInvalid, err.Error())
	}
	refused, ok := errors.AsType[*dmsocket.RefusedError](err)
	if !ok {
		return because(greeterApplyFailed, err.Error())
	}
	switch refused.Refusal {
	case dmsocket.Invalid:
		return because(greeterApplyInvalid, refused.Reason)
	case dmsocket.Unauthorized:
		return because(greeterApplyUnauthorized, refused.Reason)
	case dmsocket.Dismissed:
		return t.Get(greeterApplyDismissed)
	default:
		return because(greeterApplyFailed, refused.Reason)
	}
}

func applyGreeter(s store, runtimeDir string) error {
	login := make(map[string]any, len(config.GreeterApplyKeys))
	for _, key := range config.GreeterApplyKeys {
		if v := s.value("login." + key); v != nil {
			if err := config.InsertPath(login, key, v); err != nil {
				return fmt.Errorf("stage login.%s: %w", key, err)
			}
		}
	}
	body, err := config.YAMLDocument(map[string]any{"login": login})
	if err != nil {
		return fmt.Errorf("stage the login screen settings: %w", err)
	}
	c, err := greeterconfig.NewChange(body)
	if err != nil {
		return fmt.Errorf("stage the login screen settings: %w", err)
	}
	if _, err := dmsocket.Apply.Send(runtimeDir, c); err != nil {
		return fmt.Errorf("apply the login screen settings: %w", err)
	}
	return nil
}

func idlePage(*config.Config) pageSpec {
	return pageSpec{
		id: "idle", navKey: "settings-nav-idle", icon: "ld-moon-symbolic",
		header: "settings-page-idle",
		sections: []sectionSpec{
			{title: "settings-section-general", rows: []rowSpec{field("idle.enable"), field("idle.lock-before-sleep", idleEnabled)}},
			{title: "settings-section-idle-when", rows: []rowSpec{
				idleTimeoutsRow(idleDim), idleTimeoutsRow(idleLock), idleTimeoutsRow(idleScreenOff), idleTimeoutsRow(idleSuspend),
				field("idle.dim-percent", idleEnabled), field("idle.suspend-action", idleEnabled),
			}},
			{title: "settings-section-idle-lid", rows: []rowSpec{wmField("lid_action")}},
		},
	}
}

var idleEnabled = enabledBy("idle.enable")
