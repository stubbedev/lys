package config

// AppIconSource is part of the audio module's configuration.
//
// Icon source for app volume entries in the dropdown.
type AppIconSource string

// AppIconSource values.
const (
	// lys-shell's curated symbolic icons matched by app name.
	AppIconSourceMapped AppIconSource = "mapped"
	// Native application icons reported by PulseAudio.
	AppIconSourceNative AppIconSource = "native"
)

var _ = registerEnum(AppIconSourceMapped, AppIconSourceNative)

// AudioConfig is the audio module.
//
// Speaker volume and mute, the microphone's state, and a capture indicator.
//
// The button shows the default output's volume and mute state and the
// default input's mute state. While an app records the microphone the
// microphone icon takes the capture color; while an app holds a camera
// open or shares the screen, an icon for each shows in the capture color.
// Its dropdown holds the output and input devices, the app volumes, and
// every app capturing, each with a button that stops it.
type AudioConfig struct {
	// Speaker icons for volume levels from low to maximum.
	//
	// The percentage is divided evenly among icons. With 3 icons:
	// 1-33% uses speaker-icons\[0\], 34-66% uses speaker-icons\[1\],
	// 67-100% uses speaker-icons\[2\].
	SpeakerIcons []string `cfg:"speaker-icons"`
	// Speaker icon shown while the output is muted.
	SpeakerIconMuted string `cfg:"speaker-icon-muted"`
	// Microphone icon shown while the input is live (unmuted).
	MicIcon string `cfg:"mic-icon"`
	// Microphone icon shown while the input is muted.
	MicIconMuted string `cfg:"mic-icon-muted"`
	// Show the microphone icon only while an app records the microphone.
	MicHideWhenIdle bool `cfg:"mic-hide-when-idle"`
	// Icon shown while an app holds a camera open.
	CaptureCameraIcon string `cfg:"capture-camera-icon"`
	// Icon shown while an app shares the screen.
	CaptureScreenIcon string `cfg:"capture-screen-icon"`
	// Color of the capture indicator: the microphone icon while an app
	// records it, and the camera and screen icons.
	CaptureColor ColorValue `cfg:"capture-color"`
	// Display border around button.
	BorderShow bool `cfg:"border-show"`
	// Border color token.
	BorderColor ColorValue `cfg:"border-color"`
	// Display the speaker icon.
	IconShow bool `cfg:"icon-show"`
	// Show the module's state in a tooltip on hover.
	Tooltip bool `cfg:"tooltip"`
	// Icon foreground color. Auto selects based on variant for contrast.
	IconColor ColorValue `cfg:"icon-color"`
	// Icon container background color token.
	IconBgColor ColorValue `cfg:"icon-bg-color"`
	// Display the label.
	LabelShow bool `cfg:"label-show"`
	// Label text color token.
	LabelColor ColorValue `cfg:"label-color"`
	// Format string for the label.
	//
	// ## Examples
	//
	// - `"{{ percent }}%"` - "45%"
	// - `"{{ percent }}% · {{ mic_percent }}%"` - "45% · 80%"
	Format string `cfg:"format"`
	// Max label characters before truncation with ellipsis. Set to 0 to disable.
	LabelMaxLength uint32 `cfg:"label-max-length"`
	// Button background color token.
	ButtonBgColor ColorValue `cfg:"button-bg-color"`
	// Action on left click. Default opens the audio dropdown.
	//
	// The `lys-shell audio` commands name their device: `output-mute`
	// and `output-volume <level>` act on the speaker, `input-mute` and
	// `input-volume <level>` on the microphone. A level is 0-100, or
	// `+N` and `-N` to move it.
	LeftClick ClickAction `cfg:"left-click"`
	// Action on right click.
	RightClick ClickAction `cfg:"right-click"`
	// Action on middle click. Default toggles the speaker's mute.
	MiddleClick ClickAction `cfg:"middle-click"`
	// Action on scroll up. Default raises the speaker volume by 5.
	ScrollUp ClickAction `cfg:"scroll-up"`
	// Action on scroll down. Default lowers the speaker volume by 5.
	ScrollDown ClickAction `cfg:"scroll-down"`
	// Icon source for app volume entries in the audio dropdown.
	DropdownAppIcons AppIconSource `cfg:"dropdown-app-icons"`
	// Dynamic color thresholds based on the speaker volume percentage.
	//
	// Entries are checked in order; the last matching entry wins for each
	// color slot. Use `above` for high-value warnings (e.g., boosted volume).
	//
	// ## Example
	//
	// ```yaml
	// modules:
	//   audio:
	//     thresholds:
	//       - above: 100
	//         icon-color: status-warning
	//         label-color: status-warning
	//       - above: 130
	//         icon-color: status-error
	//         label-color: status-error
	// ```
	Thresholds []ThresholdEntry `cfg:"thresholds"`
}

// DefaultsAudio returns the schema defaults.
func DefaultsAudio() AudioConfig {
	return AudioConfig{
		SpeakerIcons: []string{
			"ld-volume-symbolic",
			"ld-volume-1-symbolic",
			"ld-volume-2-symbolic",
		},
		SpeakerIconMuted:  "ld-volume-x-symbolic",
		MicIcon:           "ld-mic-symbolic",
		MicIconMuted:      "ld-mic-off-symbolic",
		MicHideWhenIdle:   false,
		CaptureCameraIcon: "ld-webcam-symbolic",
		CaptureScreenIcon: "ld-monitor-symbolic",
		CaptureColor:      mustColor("status-warning"),
		BorderShow:        false,
		BorderColor:       mustColor("red"),
		IconShow:          true,
		Tooltip:           true,
		IconColor:         mustColor("auto"),
		IconBgColor:       mustColor("red"),
		LabelShow:         true,
		LabelColor:        mustColor("fg-default"),
		Format:            "{{ percent }}%",
		LabelMaxLength:    0,
		ButtonBgColor:     mustColor("bg-surface-elevated"),
		LeftClick:         ParseClickAction("dropdown:audio"),
		RightClick:        ClickAction{},
		MiddleClick:       ParseClickAction("lys-shell audio output-mute"),
		ScrollUp:          ParseClickAction("lys-shell audio output-volume +5"),
		ScrollDown:        ParseClickAction("lys-shell audio output-volume -5"),
		DropdownAppIcons:  AppIconSourceMapped,
		Thresholds:        []ThresholdEntry{},
	}
}

// Clicks returns the five input bindings.
func (c AudioConfig) Clicks() ClickConfig {
	return ClickConfig{c.LeftClick, c.RightClick, c.MiddleClick, c.ScrollUp, c.ScrollDown}
}
