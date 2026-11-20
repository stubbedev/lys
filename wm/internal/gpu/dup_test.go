package gpu_test

import (
	"errors"
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/fdtest"
	"github.com/stubbedev/lys/lib/fourcc"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/wm/internal/gpu"
)

func memfd(t *testing.T, content string) int {
	t.Helper()
	fd, err := unix.MemfdCreate("plane", unix.MFD_CLOEXEC)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := unix.Pwrite(fd, []byte(content), 0); err != nil {
		t.Fatal(err)
	}
	return fd
}

func TestADuplicatedDMABUFOutlivesItsSourceAndClosesOnlyItsOwnDescriptors(t *testing.T) {
	src := gpu.DMABUF{
		Size: image.Pt(4, 2), Format: fourcc.NV12, Modifier: fourcc.Linear, YUV: gpu.YUVEncoding{Coefficients: gpu.BT709},
		Planes: []gpu.Plane{{FD: memfd(t, "luma"), Stride: 4}, {FD: memfd(t, "chroma"), Offset: 8, Stride: 4}},
	}
	planes := make([]fdtest.Watch, len(src.Planes))
	for i, p := range src.Planes {
		planes[i] = fdtest.WatchFD(t, p.FD)
	}
	dup, err := src.Dup()
	if err != nil {
		t.Fatal(err)
	}
	if dup.Size != src.Size || dup.Format != src.Format || dup.Modifier != src.Modifier || dup.YUV != src.YUV || len(dup.Planes) != len(src.Planes) {
		t.Fatalf("the duplicate describes %+v, want what %+v does", dup.DMABUF, src)
	}
	for i, p := range dup.Planes {
		s := src.Planes[i]
		if p.FD == s.FD || p.Offset != s.Offset || p.Stride != s.Stride {
			t.Fatalf("plane %d is %+v, want the layout of %+v behind a descriptor of its own", i, p, s)
		}
		flags, err := unix.FcntlInt(uintptr(p.FD), unix.F_GETFD, 0)
		if err != nil {
			t.Fatal(err)
		}
		if flags&unix.FD_CLOEXEC == 0 {
			t.Fatalf("plane %d's descriptor %d would leak into a child", i, p.FD)
		}
	}
	for _, p := range src.Planes {
		if err := unix.Close(p.FD); err != nil {
			t.Fatal(err)
		}
	}
	for i, want := range []string{"luma", "chroma"} {
		got := make([]byte, len(want))
		if _, err := unix.Pread(dup.Planes[i].FD, got, 0); err != nil || string(got) != want {
			t.Fatalf("plane %d reads %q, %v after its source closed, want %q", i, got, err, want)
		}
	}
	if err := dup.Close(); err != nil {
		t.Fatal(err)
	}
	for i, w := range planes {
		if held := w.Holders(t); len(held) != 0 {
			t.Errorf("descriptors %v still hold plane %d after the source and its duplicate closed", held, i)
		}
	}
}

func TestADuplicateThatFailsNamesThePlaneAndLeaksNoDescriptor(t *testing.T) {
	open := memfd(t, "kept")
	t.Cleanup(func() { _ = unix.Close(open) })
	kept := fdtest.WatchFD(t, open)
	src := gpu.DMABUF{Size: image.Pt(4, 2), Format: fourcc.NV12, Planes: []gpu.Plane{{FD: open, Stride: 4}, {FD: -1, Stride: 4}}}
	dup, err := src.Dup()
	if dup != nil || !errors.Is(err, unix.EBADF) || !strings.Contains(err.Error(), "plane fd -1") {
		t.Fatalf("Dup = %v, %v; want EBADF naming plane fd -1", dup, err)
	}
	if held := kept.Holders(t); !slices.Equal(held, []int{open}) {
		t.Fatalf("descriptors %v hold the first plane after a failed duplicate, want the source's own %d alone", held, open)
	}
	if !kept.Open(t) {
		t.Fatalf("the failed duplicate closed the source's own descriptor %d", open)
	}
}
