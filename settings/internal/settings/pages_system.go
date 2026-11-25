package settings

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/atomicfile"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/xdg"
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
			{title: "settings-section-scale-rounding", rows: []rowSpec{field("styling.scale", scaleChips()), field("styling.rounding", roundingChoice)}},
		},
	}
}

var scaleSteps = []float64{0.75, 1, 1.25, 1.5, 1.75, 2}

var scaleUnit = customUnit{per: 1, digits: 2, unit: unitScale}

func scaleChips() rowOpt { return presetChips(scaleUnit, scalePicks(scaleSteps)) }

// scalePicks are scale presets reading as percentages.
func scalePicks(values []float64) []preset[float64] {
	out := make([]preset[float64], len(values))
	for i, v := range values {
		out[i] = preset[float64]{value: v, label: func() string { return scalePercent(v) }}
	}
	return out
}

func scalePercent(v float64) string { return unitPercent.format(0)(math.Round(v * 100)) }

// lockPage is pages/lock.
func lockPage(*config.Config) pageSpec {
	return pageSpec{
		id: "lock", navKey: "settings-nav-lock-page", icon: "ld-lock-symbolic",
		header: "settings-page-lock", lead: lockPreview,
		sections: []sectionSpec{
			{title: "settings-section-general", rows: []rowSpec{
				field("login.lock.enabled"), field("login.lock.fingerprint", fingerprintRow),
				field("login.lock.show-media"), field("login.lock.show-battery"), field("login.lock.show-switch-user"),
			}},
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
			{title: "settings-section-lock-advanced", advanced: true, rows: []rowSpec{field("login.lock.pam-service", pamServiceRow), field("login.lock.lock-on-start")}},
			{title: "settings-section-login-screen", rows: []rowSpec{field("login.greeter.show-user-list"), field("login.greeter.show-power-buttons"), field("login.greeter.fingerprint", fingerprintRow), field("login.greeter.screen-reader", screenReaderRow)}},
			{title: "settings-section-cursor", rows: []rowSpec{
				field("login.greeter.match-cursor"),
				field("login.greeter.cursor-theme", ownCursor),
				field("login.greeter.cursor-size", countChips(inPixels, 24, 32, 48, 64, 96), ownCursor),
			}},
		},
		footer: func(k *kit) widget.Widget { return newGreeterFooter(k) }, extrasOnRows: true,
		related:  []linkGroup{{title: "settings-lock-section-related", links: lockLinks}},
		animates: "animations.lock",
	}
}

const lockBlurSoftMax = 100

var ownCursor = shownWhen("login.greeter.match-cursor", switchedOff)

var pamServiceRow rowOpt = func(r *rowSpec) {
	r.vet = func(ctx context.Context, k *kit, v any) rowProblem {
		service, ok := v.(string)
		if !ok {
			return notSaved(fmt.Errorf("settings: %v is no PAM service name", v))
		}
		if err := k.checkPAM(ctx, service); err != nil {
			return rowProblem{summary: locale.T().Get("settings-pam-check-failed", i18n.Str("service", service)), err: err}
		}
		return rowProblem{}
	}
}

var lockBackgroundPictures = map[string]picture{
	string(config.BackgroundColor): drawn(func(cv *render.Canvas, r render.Rect) {
		screenFrame(cv, r, func(s render.Rect) { cv.FillRect(s, previewImageTop) })
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
	greeterPending           = "settings-greeter-pending"
	greeterApplied           = "settings-greeter-applied"
)

var loginRuntimeDir = dmsocket.DefaultRuntimeDir

var lockLinks = []pageLink{
	{"idle", "settings-lock-link-idle-detail"},
	{usersID, "settings-lock-link-users-detail"},
}

type greeterFooter struct {
	*widget.Box
	k       *kit
	status  *widget.Label
	hint    *widget.Label
	outcome *inlineState
}

func newGreeterFooter(k *kit) *greeterFooter {
	t := locale.T()
	f := &greeterFooter{Box: gapBox(widget.Column), k: k}
	f.AddClass("settings-section")
	f.status = wrapped(k.label("", "settings-greeter-status", css.Caption))
	f.hint = k.note(t.Get(greeterApplyHint), "settings-greeter-hint")
	f.outcome = k.inlineState()
	apply := func() {
		async(k, func() error { return applyGreeter(k.store, loginRuntimeDir) }, func(err error) {
			f.showOutcome(err)
			f.configChanged()
		})
	}
	f.Append(f.status, false)
	f.AppendAligned(k.primaryButton(t.Get(greeterApplyLabel), apply), false, widget.AlignStart)
	f.Append(f.hint, false)
	f.Append(f.outcome, false)
	f.configChanged()
	return f
}

// configChanged says whether the login screen shows the config as it is.
func (f *greeterFooter) configChanged() {
	pending, err := greeterPendingChanges(f.k.store)
	if err != nil {
		log.Printf("settings: greeter status: %v", err)
	}
	key, class := greeterApplied, css.DimLabel
	if pending || err != nil {
		key, class = greeterPending, css.Warning
	}
	f.status.RemoveClass(css.DimLabel, css.Warning)
	f.status.AddClass(class)
	f.status.SetText(locale.T().Get(key))
}

// showOutcome says how applying ended: done, or why not, the error
// behind it in the closed detail.
func (f *greeterFooter) showOutcome(err error) {
	t := locale.T()
	f.hint.SetVisible(false)
	if err == nil {
		f.outcome.Empty("", t.Get(greeterApplyDone))
		return
	}
	log.Printf("settings: greeter apply: %v", err)
	f.outcome.Error(t.Get(greeterApplyFailure(err)), err, nil)
}

// greeterApplyFailure is the message of an apply that failed with err.
func greeterApplyFailure(err error) string {
	switch {
	case errors.Is(err, dmsocket.ErrNoDaemon):
		return greeterApplyNoDaemon
	case errors.Is(err, greeterconfig.ErrInvalid):
		return greeterApplyInvalid
	}
	refused, ok := errors.AsType[*dmsocket.RefusedError](err)
	if !ok {
		return greeterApplyFailed
	}
	switch refused.Refusal {
	case dmsocket.Invalid:
		return greeterApplyInvalid
	case dmsocket.Unauthorized:
		return greeterApplyUnauthorized
	case dmsocket.Dismissed:
		return greeterApplyDismissed
	default:
		return greeterApplyFailed
	}
}

func greeterDocument(s store) (string, error) {
	login := make(map[string]any, len(config.GreeterApplyKeys))
	for _, key := range config.GreeterApplyKeys {
		if v := s.value("login." + key); v != nil {
			if err := config.InsertPath(login, key, v); err != nil {
				return "", fmt.Errorf("stage login.%s: %w", key, err)
			}
		}
	}
	body, err := config.YAMLDocument(map[string]any{"login": login})
	if err != nil {
		return "", fmt.Errorf("stage the login screen settings: %w", err)
	}
	return body, nil
}

const greeterAppliedFile = "greeter-applied.yaml"

func greeterAppliedPath() (string, error) {
	dir, ok := xdg.OS.StateDir("lys-settings")
	if !ok {
		return "", errors.New("no state directory to remember the applied login screen settings in")
	}
	return filepath.Join(dir, greeterAppliedFile), nil
}

func greeterPendingChanges(s store) (bool, error) {
	body, err := greeterDocument(s)
	if err != nil {
		return false, err
	}
	path, err := greeterAppliedPath()
	if err != nil {
		return false, err
	}
	applied, err := os.ReadFile(path) //nolint:gosec // lys-settings's own state file
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("read the applied login screen settings: %w", err)
	}
	return body != string(applied), nil
}

func applyGreeter(s store, runtimeDir string) error {
	body, err := greeterDocument(s)
	if err != nil {
		return err
	}
	c, err := greeterconfig.NewChange(body)
	if err != nil {
		return fmt.Errorf("stage the login screen settings: %w", err)
	}
	if _, err := dmsocket.Apply.Send(runtimeDir, c); err != nil {
		return fmt.Errorf("apply the login screen settings: %w", err)
	}
	path, err := greeterAppliedPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("remember the applied login screen settings: %w", err)
	}
	if err := atomicfile.WriteFile(path, []byte(body), 0o600); err != nil {
		return fmt.Errorf("remember the applied login screen settings: %w", err)
	}
	return nil
}

func idlePage(*config.Config) pageSpec {
	return pageSpec{
		id: "idle", navKey: "settings-nav-idle", icon: "ld-moon-symbolic",
		header: "settings-page-idle",
		sections: []sectionSpec{
			{title: "settings-section-general", rows: []rowSpec{field("idle.enable"), field("idle.lock-before-sleep", idleEnabled)}},
			{title: "settings-section-idle-when", preview: idleTimelinePreview, rows: []rowSpec{
				idleTimesRow(),
				field("idle.dim-percent", idleEnabled), field("idle.suspend-action", idleEnabled, sleepActionRow),
			}},
			{title: "settings-section-idle-attention", rows: []rowSpec{field("idle.attention", idleEnabled)}},
			{title: "settings-section-idle-lid", rows: []rowSpec{wmField("lid_action")}},
		},
	}
}

var idleEnabled = enabledBy("idle.enable")
