package recorder

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/service/recorder/webcamframe"
)

// Webcam is the picture-in-picture overlay.
type Webcam struct {
	// Device is the V4L2 node; "" auto-selects the first camera.
	Device string
	webcamframe.Placement
}

// Options is one recording's settings.
type Options struct {
	OutputPath string
	Format     config.RecorderFormat
	Framerate  uint32
	ShowCursor bool
	// Microphone captures MicrophoneDevice ("" the default source);
	// SystemAudio the default sink's monitor.
	Microphone       bool
	MicrophoneDevice string
	SystemAudio      bool
	// Webcam is the overlay, nil for none.
	Webcam *Webcam
}
