package pulsetest_test

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/pulse/native"
	"github.com/stubbedev/lys/lib/pulse/pulsetest"
	"github.com/stubbedev/lys/lib/waittest"
)

func device[I, M ~uint32](index I, name string) native.DeviceInfo[I, M] {
	return native.DeviceInfo[I, M]{
		Index: index, Name: name,
		SampleSpec: native.SampleSpec{Format: native.SampleS16LE, Channels: 2, Rate: 48000},
		ChannelMap: native.ChannelMap{native.ChannelFrontLeft, native.ChannelFrontRight},
		Volume:     native.CVolume{native.VolumeNorm, native.VolumeNorm},
		Card:       native.InvalidIndex, OwnerModule: native.InvalidIndex,
		Props: native.PropList{},
	}
}

func sink(index native.SinkIndex, name string) native.SinkInfo {
	return device[native.SinkIndex, native.SourceIndex](index, name)
}

func source(index native.SourceIndex, name string) native.SourceInfo {
	return device[native.SourceIndex, native.SinkIndex](index, name)
}

func scripted[I, D ~uint32](index I) native.StreamInfo[I, D] {
	return native.StreamInfo[I, D]{
		Index: index, Name: "scripted", OwnerModule: native.InvalidIndex, Client: 9, Device: 1,
		SampleSpec: native.SampleSpec{Format: native.SampleS16LE, Channels: 2, Rate: 48000},
		ChannelMap: native.ChannelMap{native.ChannelFrontLeft, native.ChannelFrontRight},
		Volume:     native.CVolume{native.VolumeNorm, native.VolumeNorm},
		Props:      native.PropList{}, HasVolume: true, VolumeWritable: true,
		Format: native.FormatInfo{Encoding: native.EncodingPCM, Props: native.PropList{}},
	}
}

func dial(t *testing.T, srv *pulsetest.Server, opts native.Options) *native.Conn {
	t.Helper()
	opts.Server = srv.Addr()
	c, err := native.Dial(t.Context(), opts)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func indexes[I, D ~uint32](streams []native.StreamInfo[I, D]) []I {
	out := make([]I, len(streams))
	for i, st := range streams {
		out[i] = st.Index
	}
	return out
}

func TestADisconnectedClientsStreamsLeaveTheListings(t *testing.T) {
	srv := pulsetest.New(t)
	srv.PutSink(sink(1, "speakers"))
	srv.PutSource(source(2, "microphone"))
	srv.SetDefaults("speakers", "microphone")
	srv.PutSinkInput(scripted[native.SinkInputIndex, native.SinkIndex](50))
	srv.PutSourceOutput(scripted[native.SourceOutputIndex, native.SourceIndex](51))

	events := make(chan native.SubscribeEvent, 16)
	watcher := dial(t, srv, native.Options{OnSubscribe: func(ev native.SubscribeEvent) { events <- ev }})
	if err := watcher.Subscribe(t.Context(), native.MaskSinkInput|native.MaskSourceOutput); err != nil {
		t.Fatal(err)
	}

	spec := native.SampleSpec{Format: native.SampleS16LE, Channels: 2, Rate: 48000}
	channels := native.ChannelMap{native.ChannelFrontLeft, native.ChannelFrontRight}
	client := dial(t, srv, native.Options{})
	playback, err := client.CreatePlaybackStream(t.Context(), native.PlaybackParams{SampleSpec: spec, ChannelMap: channels})
	if err != nil {
		t.Fatal(err)
	}
	record, err := client.CreateRecordStream(t.Context(), native.RecordParams{SampleSpec: spec, ChannelMap: channels}, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	closed, err := client.CreatePlaybackStream(t.Context(), native.PlaybackParams{SampleSpec: spec, ChannelMap: channels})
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	sinkInput, sourceOutput := playback.Info().SinkInput, record.Info().SourceOutput
	for _, want := range []native.SubscribeEvent{
		{Facility: native.FacilitySinkInput, Operation: native.OpNew, Index: uint32(sinkInput)},
		{Facility: native.FacilitySourceOutput, Operation: native.OpNew, Index: uint32(sourceOutput)},
		{Facility: native.FacilitySinkInput, Operation: native.OpNew, Index: uint32(closed.Info().SinkInput)},
		{Facility: native.FacilitySinkInput, Operation: native.OpRemove, Index: uint32(closed.Info().SinkInput)},
	} {
		if got := waittest.For(t, events, "an event for the client's streams"); got != want {
			t.Fatalf("event %+v, want %+v", got, want)
		}
	}

	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	removed := []native.SubscribeEvent{waittest.For(t, events, "the disconnect's first event"), waittest.For(t, events, "the disconnect's second event")}
	want := []native.SubscribeEvent{
		{Facility: native.FacilitySourceOutput, Operation: native.OpRemove, Index: uint32(sourceOutput)},
		{Facility: native.FacilitySinkInput, Operation: native.OpRemove, Index: uint32(sinkInput)},
	}
	if !slices.Equal(removed, want) {
		t.Errorf("the disconnect announced %+v, want %+v", removed, want)
	}

	inputs, err := watcher.SinkInputs(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	outputs, err := watcher.SourceOutputs(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got := indexes(inputs); !slices.Equal(got, []native.SinkInputIndex{50}) {
		t.Errorf("sink inputs after the disconnect %v, want the scripted 50 alone", got)
	}
	if got := indexes(outputs); !slices.Equal(got, []native.SourceOutputIndex{51}) {
		t.Errorf("source outputs after the disconnect %v, want the scripted 51 alone", got)
	}
	if _, err := watcher.SinkInput(t.Context(), sinkInput); err == nil {
		t.Errorf("sink input %d of the disconnected client is still served", sinkInput)
	}
	if _, ok := srv.SourceOutput(sourceOutput); ok {
		t.Errorf("source output %d of the disconnected client is still held", sourceOutput)
	}
	if n := len(srv.Playbacks()) + len(srv.Records()); n != 0 {
		t.Errorf("%d streams of the disconnected client still live", n)
	}
	select {
	case ev := <-events:
		t.Errorf("the disconnect announced %+v too: the stream the client closed itself was removed twice", ev)
	default:
	}
}
