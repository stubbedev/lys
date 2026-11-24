package gpu_test

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/fourcc"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/wm/internal/gpu"
)

func TestAMissingLibgbmNamesItsVariable(t *testing.T) {
	paths := gpu.PathsFromEnv(os.Getenv)
	paths.GBM = "/nonexistent/libgbm.so.1"
	if _, err := gpu.LoadGBM(paths); err == nil || !strings.Contains(err.Error(), gpu.GBMEnv) {
		t.Fatalf("a missing libgbm: %v", err)
	}
	if got := gpu.PathsFromEnv(func(string) string { return "" }); got.GBM != "libgbm.so.1" {
		t.Fatalf("paths %+v without the variable", got)
	}
	env := map[string]string{gpu.GBMEnv: "/x/libgbm.so.1"}
	if got := gpu.PathsFromEnv(func(k string) string { return env[k] }); got.GBM != "/x/libgbm.so.1" {
		t.Fatalf("paths %+v", got)
	}
}

func renderNode(t *testing.T) (int, string) {
	t.Helper()
	nodes, err := filepath.Glob("/dev/dri/renderD*")
	if err != nil || len(nodes) == 0 {
		t.Skip("no DRM render node")
	}
	fd, err := unix.Open(nodes[0], unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Skipf("open %s: %v", nodes[0], err)
	}
	t.Cleanup(func() { _ = unix.Close(fd) })
	return fd, nodes[0]
}

func TestGBMAllocatesExportableBuffers(t *testing.T) {
	fd, node := renderNode(t)
	lib, err := gpu.LoadGBM(gpu.PathsFromEnv(os.Getenv))
	if err != nil {
		t.Fatal(err)
	}
	dev, err := lib.Open(fd)
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()
	b, err := dev.Allocate(image.Pt(64, 32), fourcc.XRGB8888, nil, gpu.UseRendering)
	if err != nil {
		t.Fatalf("the %s backend on %s with GBM_BACKENDS_PATH=%q: %v", dev.Backend(), node, os.Getenv("GBM_BACKENDS_PATH"), err)
	}
	if len(b.Planes) == 0 || b.Planes[0].FD < 0 || b.Planes[0].Stride < 64*4 || b.Size != image.Pt(64, 32) {
		t.Fatalf("buffer %+v", b.DMABUF)
	}
	t.Logf("%s: modifier %v, planes %+v", dev.Backend(), b.Modifier, b.Planes)
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := dev.Allocate(image.Pt(0, 4), fourcc.XRGB8888, nil, gpu.UseRendering); err == nil {
		t.Fatal("an empty buffer was allocated")
	}
}

func TestCompilingCallsOnlyTheHookItsOwnPinnedThreadSet(t *testing.T) {
	if err := gpu.CompileThrough(func() {}); err == nil {
		t.Fatal("CompileThrough on a goroutine that pinned nothing succeeded")
	}
	type counts struct{ own, other, afterUnpin int }
	done := make(chan counts, 1)
	failed := make(chan error, 1)
	go func() {
		var got counts
		unpin := gpu.PinThread()
		err := gpu.CompileThrough(func() { got.own++ })
		gpu.Compiling()
		gpu.Compiling()
		elsewhere := make(chan struct{})
		go func() {
			gpu.Compiling()
			close(elsewhere)
		}()
		<-elsewhere
		got.other = got.own - 2
		unpin()
		before := got.own
		gpu.Compiling()
		got.afterUnpin = got.own - before
		failed <- err
		done <- got
	}()
	if err := <-failed; err != nil {
		t.Fatal(err)
	}
	if got := <-done; got != (counts{own: 2}) {
		t.Fatalf("hook calls %+v, want two from the pinned goroutine and none from another or after unpinning", got)
	}
}
