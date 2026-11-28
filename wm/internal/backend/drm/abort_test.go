package drm

import "testing"

func TestAnAbortedFrameLeavesItsBufferWithNoAge(t *testing.T) {
	r := start(t)
	o := r.b.Outputs()[0]
	for i, seq := range []uint32{77, 78} {
		if err := o.ScheduleFrame(); err != nil {
			t.Fatal(err)
		}
		r.committedSince("a frame", o, len(r.dev.commits))
		r.dev.flip(t, 10, seq)
		r.until("the flip to land", func() bool { return !o.flipping && len(r.rec.presents) > i })
	}
	f, err := o.BeginFrame()
	if err != nil {
		t.Fatal(err)
	}
	if f.Age == 0 {
		t.Fatal("a buffer shown before begins a frame with no age, want its age since then")
	}
	o.AbortFrame()
	again, err := o.BeginFrame()
	if err != nil {
		t.Fatal(err)
	}
	defer o.AbortFrame()
	if again.Target != f.Target {
		t.Fatal("the frame after an aborted one draws in another buffer")
	}
	if again.Age != 0 {
		t.Errorf("the buffer an aborted frame may have drawn in begins the next with age %d, want 0", again.Age)
	}
}
