package bar

import (
	"context"
	"log"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/desktop/spawn"

	"github.com/stubbedev/lys/shell/internal/audiolevel"
	"github.com/stubbedev/lys/shell/service/brightness"
	"github.com/stubbedev/lys/shell/service/recorder"
	"github.com/stubbedev/lys/shell/service/sessionctl"
	"github.com/stubbedev/lys/shell/shell/powermenu"
)

// spawnQuiet runs a user's shell command line (a seam for tests).
var spawnQuiet = spawn.Quiet

// runClickAction dispatches one binding: brightness steps and toggles
// drive every backlight natively, the shell's builtins run in-process,
// and everything else shells out.
func runClickAction(ctx ModuleContext, action config.ClickAction) {
	switch action.Kind {
	case config.ClickNone:
	case config.ClickDropdown:
		log.Printf("click: dropdown %q not available yet", action.Dropdown)
	case config.ClickShell:
		runShellBuiltin(ctx, action.Command)
	case config.ClickBrightness:
		stepBrightness(ctx, action.Brightness)
	case config.ClickBrightnessToggle:
		toggleBrightness(ctx)
	case config.ClickShellBuiltin:
		runBuiltin(ctx, action.Builtin)
	}
}

// runBuiltin applies b to the running shell's own state; a service the
// shell runs without leaves it undone, and says so.
func runBuiltin(ctx ModuleContext, b config.ShellBuiltin) {
	switch b {
	case config.ShellBuiltinIdleToggle, config.ShellBuiltinIdleToggleIndefinite:
		if ctx.IdleInhibit == nil {
			log.Printf("click: %s: no idle inhibitor", b)
			return
		}
		if ctx.IdleInhibit.Active() {
			ctx.IdleInhibit.Disable()
			return
		}
		ctx.IdleInhibit.Enable(b == config.ShellBuiltinIdleToggleIndefinite)
	case config.ShellBuiltinDndToggle:
		if ctx.Notifications == nil {
			log.Printf("click: %s: no notification service", b)
			return
		}
		ctx.Notifications.ToggleDND()
	case config.ShellBuiltinRecorderToggle, config.ShellBuiltinRecorderStart, config.ShellBuiltinRecorderStop,
		config.ShellBuiltinRecorderPause, config.ShellBuiltinRecorderResume:
		if ctx.Recorder == nil {
			log.Printf("click: %s: no recorder", b)
			return
		}
		runRecorderBuiltin(ctx.Recorder, b)
	default:
		log.Printf("click: unknown builtin %s", b)
	}
}

func runRecorderBuiltin(r *recorder.State, b config.ShellBuiltin) {
	switch b {
	case config.ShellBuiltinRecorderToggle:
		r.Toggle()
	case config.ShellBuiltinRecorderStart:
		r.Start()
	case config.ShellBuiltinRecorderStop:
		r.Stop()
	case config.ShellBuiltinRecorderPause:
		r.SetPaused(true)
	case config.ShellBuiltinRecorderResume:
		r.SetPaused(false)
	}
}

// powerRunner runs the power menu's commands: a built-in in process,
// off the loop, with a failure toasted on it; a user's command through
// the shell.
type powerRunner struct {
	system sessionctl.SystemBus
	invoke func(func())
	toast  func(label, icon string)
}

func (r powerRunner) run(a powermenu.Action, cmd config.PowerCommand) {
	if line, ok := cmd.Shell(); ok {
		spawnQuiet(line)
		return
	}
	b, ok := cmd.Builtin()
	if !ok {
		return
	}
	go func() {
		err := r.builtin(context.Background(), b)
		if err == nil {
			return
		}
		log.Printf("power menu: %v", err)
		if r.toast != nil {
			r.invoke(func() { r.toast(i18n.T("launcher-failed", i18n.Str("what", a.Label())), a.Icon()) })
		}
	}()
}

func (r powerRunner) builtin(ctx context.Context, b config.PowerBuiltin) error {
	return sessionctl.Run(ctx, r.system, b)
}

// runShellBuiltin handles the `lys-shell` screenshot and audio verbs
// in-process, and runs any other command line through the shell.
func runShellBuiltin(ctx ModuleContext, cmd string) {
	verb, _ := splitBuiltin(cmd)
	if mode, target, ok := screenshotBuiltin(verb); ok && ctx.Screenshot != nil {
		ctx.Screenshot(mode, target)
		return
	}
	if b, ok, err := parseAudioBuiltin(verb); ok {
		if err != nil {
			log.Printf("click: %s: %v", cmd, err)
			return
		}
		runAudioBuiltin(ctx, cmd, b)
		return
	}
	spawnQuiet(cmd)
}

// splitBuiltin reports the verb of a `lys-shell <area> <verb> …` command,
// empty when the command is not one.
func splitBuiltin(cmd string) (string, string) {
	const prefix = "lys-shell "
	if len(cmd) < len(prefix) || cmd[:len(prefix)] != prefix {
		return "", cmd
	}
	return cmd[len(prefix):], ""
}

// audioTarget is the device a `lys-shell audio` builtin acts on.
type audioTarget int

const (
	audioSpeaker audioTarget = iota
	audioMic
)

// device is the target's default device.
func (t audioTarget) device(ctx context.Context, src pulse.Source) (pulse.Device, error) {
	if t == audioMic {
		dev, err := src.DefaultSource(ctx)
		return dev.Device, err
	}
	dev, err := src.DefaultSink(ctx)
	return dev.Device, err
}

// audioBuiltin is a `lys-shell audio` verb the bar runs in-process: a
// mute toggle, or a volume level, on the speaker or the microphone.
type audioBuiltin struct {
	target audioTarget
	// level is the volume to reach; nil toggles the mute.
	level *audiolevel.Level
}

// audioBuiltinVerbs are the in-process verbs by their name after
// `audio`: the target, and whether the verb takes a level.
var audioBuiltinVerbs = map[string]struct {
	target audioTarget
	level  bool
}{
	"output-mute":   {audioSpeaker, false},
	"output-volume": {audioSpeaker, true},
	"input-mute":    {audioMic, false},
	"input-volume":  {audioMic, true},
}

// parseAudioBuiltin reads a verb ("audio output-volume +5"); ok is
// false for one the bar does not run in-process (a level read, any
// other area), and err names a level that does not parse.
func parseAudioBuiltin(verb string) (audioBuiltin, bool, error) {
	parts := strings.Fields(verb)
	if len(parts) < 2 || parts[0] != "audio" {
		return audioBuiltin{}, false, nil
	}
	spec, ok := audioBuiltinVerbs[parts[1]]
	if !ok {
		return audioBuiltin{}, false, nil
	}
	b := audioBuiltin{target: spec.target}
	if !spec.level {
		return b, len(parts) == 2, nil
	}
	if len(parts) != 3 {
		return audioBuiltin{}, false, nil
	}
	level, err := audiolevel.Parse(parts[2])
	if err != nil {
		return audioBuiltin{}, true, err
	}
	b.level = &level
	return b, true, nil
}

// runAudioBuiltin applies b off the loop; cmd names it in the log.
func runAudioBuiltin(ctx ModuleContext, cmd string, b audioBuiltin) {
	src := ctx.Pulse
	if src == nil {
		log.Printf("click: %s: no audio source", cmd)
		return
	}
	go func() {
		if err := applyAudioBuiltin(context.Background(), src, b); err != nil {
			log.Printf("click: %s: %v", cmd, err)
		}
	}()
}

// applyAudioBuiltin toggles the target's mute or moves its volume.
func applyAudioBuiltin(ctx context.Context, src pulse.Source, b audioBuiltin) error {
	dev, err := b.target.device(ctx, src)
	if err != nil {
		return err
	}
	if b.level == nil {
		return src.SetDeviceMute(ctx, dev.Key, !dev.Muted)
	}
	next := b.level.Apply(dev.Volume.AveragePercentage())
	return src.SetDeviceVolume(ctx, dev.Key, pulse.VolumeFromPercentage(next, dev.Volume.Channels()))
}

// stepBrightness moves every backlight by delta percent, floored at
// the configured minimum (the dimmer can never scroll fully dark; 0%
// is reserved for the toggle).
func stepBrightness(ctx ModuleContext, delta int32) {
	if ctx.Brightness == nil {
		return
	}
	change := brightness.Relative(delta)
	floor := float64(ctx.Config.Brightness.MinBrightness)
	go func() {
		devices, err := ctx.Brightness.Devices(context.Background())
		if err != nil || len(devices) == 0 {
			return
		}
		for _, device := range devices {
			if err := ctx.Brightness.Set(context.Background(), device.Name, change.Target(device, floor)); err != nil {
				log.Printf("click: brightness %s: %v", device.Name, err)
			}
		}
	}()
}

// toggleBrightness decides one target state from the whole set (any
// monitor lit means blackout all, all dark restores all) so the
// monitors stay in lockstep.
func toggleBrightness(ctx ModuleContext) {
	if ctx.Brightness == nil {
		return
	}
	go func() {
		devices, err := ctx.Brightness.Devices(context.Background())
		if err != nil || len(devices) == 0 {
			return
		}
		goDark := false
		for _, device := range devices {
			if device.Brightness > 0 {
				goDark = true
				break
			}
		}
		for _, device := range devices {
			target := 0.0
			if !goDark {
				target = 100
			}
			if err := ctx.Brightness.Set(context.Background(), device.Name, target); err != nil {
				log.Printf("click: brightness toggle %s: %v", device.Name, err)
			}
		}
	}()
}
