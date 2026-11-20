package pulsetest_test

import (
	"os"
	"strconv"
	"testing"

	"github.com/stubbedev/lys/lib/pulse/native"
	"github.com/stubbedev/lys/lib/pulse/pulsetest"
)

func TestARecordStreamReportsItsClientsPropertiesUnderItsOwn(t *testing.T) {
	srv := pulsetest.New(t)
	srv.PutSource(device(2, "microphone"))
	srv.SetDefaults("", "microphone")
	client := dial(t, srv, native.Options{ClientName: "recorder"})
	spec := native.SampleSpec{Format: native.SampleS16LE, Channels: 2, Rate: 48000}
	channels := native.ChannelMap{native.ChannelFrontLeft, native.ChannelFrontRight}
	own := native.PropList{"media.name": "Capture", "application.name": "Recorder window"}
	record, err := client.CreateRecordStream(t.Context(), native.RecordParams{SampleSpec: spec, ChannelMap: channels, Props: own}, func([]byte) {})
	if err != nil {
		t.Fatal(err)
	}
	st, ok := srv.SourceOutput(record.Info().SourceOutput)
	if !ok {
		t.Fatal("the record stream is not a source output")
	}
	if got := st.Props["application.process.id"]; got != strconv.Itoa(os.Getpid()) {
		t.Errorf("the stream's process id is %q, want the client's", got)
	}
	if got := st.Props["application.name"]; got != "Recorder window" {
		t.Errorf("the stream's application name is %q, want its own over the client's", got)
	}
	if got := srv.Records()[0].Props["application.process.id"]; got != "" {
		t.Errorf("the stream's own properties gained the client's process id %q", got)
	}
}
