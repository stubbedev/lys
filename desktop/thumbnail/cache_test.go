package thumbnail

import (
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fileInfo struct{ size int64 }

func (fileInfo) Name() string       { return "" }
func (f fileInfo) Size() int64      { return f.size }
func (fileInfo) Mode() os.FileMode  { return 0 }
func (fileInfo) ModTime() time.Time { return time.Unix(1700000000, 0) }
func (fileInfo) IsDir() bool        { return false }
func (fileInfo) Sys() any           { return nil }

// A file's thumbnails are named by the MD5 of its file URI under the
// cache's size directories, in $XDG_CACHE_HOME when it is absolute, else
// in ~/.cache.
func TestTheCachePathIsTheMd5OfTheFileURI(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/cache")
	tc, err := cacheFor("/home/user/Photos/me.png", fileInfo{size: 7})
	if err != nil {
		t.Fatal(err)
	}
	sum := md5.Sum([]byte("file:///home/user/Photos/me.png"))
	if want := filepath.Join("/cache/thumbnails/normal", hex.EncodeToString(sum[:])+".png"); tc.path("normal") != want {
		t.Errorf("path = %s, want %s", tc.path("normal"), want)
	}
	if want := filepath.Join("/cache/thumbnails/fail/lys-shell", hex.EncodeToString(sum[:])+".png"); tc.failPath() != want {
		t.Errorf("fail path = %s, want %s", tc.failPath(), want)
	}
	if tc.mtime != "1700000000" || tc.size != "7" {
		t.Errorf("Thumb::MTime %s, Thumb::Size %s", tc.mtime, tc.size)
	}
	t.Setenv("XDG_CACHE_HOME", "relative")
	t.Setenv("HOME", "/home/u")
	if tc, _ := cacheFor("/x.png", fileInfo{}); tc.root != "/home/u/.cache/thumbnails" {
		t.Errorf("a relative cache home = %s", tc.root)
	}
}

func TestFlavorForIsTheSmallestSizeAtLeastAsLarge(t *testing.T) {
	for px, want := range map[int]string{1: "normal", 128: "normal", 129: "large", 256: "large", 900: "xx-large", 5000: "xx-large"} {
		if got := flavorFor(px).dir; got != want {
			t.Errorf("flavorFor(%d) = %s, want %s", px, got, want)
		}
	}
}
