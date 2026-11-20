package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/pulse/native"
	"github.com/stubbedev/lys/lib/pulse/pulsetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestSpeakersStandAroundTheListenerWhereTheChannelMapPlacesThem(t *testing.T) {
	stereo, listener := speakerLayout(pulsetest.StereoMap())
	if want := []speakerSpot{{0, gridCell{0, 0}}, {1, gridCell{4, 0}}}; !slices.Equal(stereo, want) || listener != (gridCell{2, 1}) {
		t.Errorf("stereo %v around %v, want %v around the listener below them", stereo, listener, want)
	}
	surround := native.ChannelMap{native.ChannelFrontLeft, native.ChannelFrontRight, native.ChannelFrontCenter, native.ChannelLFE, native.ChannelRearLeft, native.ChannelRearRight}
	spots, listener := speakerLayout(surround)
	want := []speakerSpot{{0, gridCell{0, 0}}, {1, gridCell{4, 0}}, {2, gridCell{2, 0}}, {3, gridCell{1, 1}}, {4, gridCell{0, 2}}, {5, gridCell{4, 2}}}
	if !slices.Equal(spots, want) || listener != (gridCell{2, 1}) {
		t.Errorf("5.1 %v around %v, want %v", spots, listener, want)
	}
}

func TestASpeakerWithNoPlaceOrATakenOneStandsInTheLastRow(t *testing.T) {
	spots, listener := speakerLayout(native.ChannelMap{native.ChannelFrontLeft, native.ChannelFrontLeft, native.ChannelAux0})
	want := []speakerSpot{{0, gridCell{0, 0}}, {1, gridCell{0, 2}}, {2, gridCell{1, 2}}}
	if !slices.Equal(spots, want) || listener != (gridCell{2, 1}) {
		t.Errorf("spots %v around %v, want %v", spots, listener, want)
	}
	if spots, _ := speakerLayout(nil); len(spots) != 0 {
		t.Errorf("no channels stand %v", spots)
	}
}

func TestTheDiagramHasOneTestPerChannelOfTheOutput(t *testing.T) {
	srv := soundServer(t)
	surround := pulsetest.Sink(5, headsetSink, "Headset", 6)
	surround.Card, surround.Ports, surround.ActivePort = headsetCardID, nil, ""
	surround.ChannelMap = native.ChannelMap{native.ChannelFrontLeft, native.ChannelFrontRight, native.ChannelFrontCenter, native.ChannelLFE, native.ChannelRearLeft, native.ChannelRearRight}
	surround.Volume = native.CVolume{0x8000, 0x8000, 0x8000, 0x8000, 0x8000, 0x8000}
	srv.PutSink(surround)
	r := startSoundOn(t, srv)
	r.l.until("six channels shown", func() bool { return len(r.p.output.testMap) == 6 })
	buttons := treetest.All[*widget.Button](r.p.output.tests)
	if len(buttons) != 6 || len(treetest.WithClass(r.p.output.tests, "sound-listener")) != 1 {
		t.Fatalf("%d speakers in the diagram, want the six channels around one listener", len(buttons))
	}
	sub := locale.T().Get("settings-sound-test-channel", i18n.Str("channel", locale.T().Get("settings-sound-channel-LFE")))
	treetest.Button(t, r.p.output.tests, sub).OnClick()
	plays := r.srv.Plays()
	if len(plays) != 1 || plays[0].Name != testToneName(surround.ChannelMap, 3) {
		t.Fatalf("plays %+v, want the subwoofer's tone", plays)
	}
	r.p.output.devices.SetSelected(0)
	r.l.until("the stereo speakers shown", func() bool { return len(r.p.output.testMap) == 2 })
	if n := len(treetest.All[*widget.Button](r.p.output.tests)); n != 2 {
		t.Fatalf("%d speakers after switching to stereo", n)
	}
}
