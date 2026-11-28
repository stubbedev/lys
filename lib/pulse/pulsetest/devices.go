package pulsetest

import "github.com/stubbedev/lys/lib/pulse/native"

// StereoSpec is the s16le 48 kHz stereo sample spec the fixtures use.
func StereoSpec() native.SampleSpec {
	return native.SampleSpec{Format: native.SampleS16LE, Channels: 2, Rate: 48000}
}

// StereoMap is the front-left/front-right channel map.
func StereoMap() native.ChannelMap {
	return native.ChannelMap{native.ChannelFrontLeft, native.ChannelFrontRight}
}

// Sink is a running ALSA sink at 50% volume with two ports, whose
// monitor source has index monitor.
func Sink(index native.SinkIndex, name, desc string, monitor native.SourceIndex) native.SinkInfo {
	return native.SinkInfo{
		Index: index, Name: name, Description: desc, SampleSpec: StereoSpec(), ChannelMap: StereoMap(),
		OwnerModule: 4, Volume: native.CVolume{0x8000, 0x8000}, Monitor: monitor, MonitorName: name + ".monitor",
		Driver: "alsa", Props: native.PropList{"device.description": desc}, BaseVolume: native.VolumeNorm,
		State: native.StateRunning, VolumeSteps: 65537, Card: 0,
		Ports: []native.Port{
			{Name: "speaker", Description: "Speaker", Priority: 100, Available: native.PortAvailableUnknown},
			{Name: "headphones", Description: "Headphones", Priority: 200, Available: native.PortAvailableNo},
		},
		ActivePort: "speaker",
		Formats:    []native.FormatInfo{{Encoding: native.EncodingPCM, Props: native.PropList{}}},
	}
}

// Source is a suspended ALSA source at full volume; monitorOf and
// monitorName name the sink it monitors (InvalidIndex and "" for a
// real input).
func Source(index native.SourceIndex, name, desc string, monitorOf native.SinkIndex, monitorName string) native.SourceInfo {
	return native.SourceInfo{
		Index: index, Name: name, Description: desc, SampleSpec: StereoSpec(), ChannelMap: StereoMap(),
		OwnerModule: native.InvalidIndex, Volume: native.CVolume{native.VolumeNorm, native.VolumeNorm},
		Monitor: monitorOf, MonitorName: monitorName, Driver: "alsa", Props: native.PropList{},
		BaseVolume: native.VolumeNorm, State: native.StateSuspended, Card: native.InvalidIndex,
	}
}
