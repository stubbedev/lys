package vfstest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"path"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/vfs"
)

// bigSize is the file Conform round-trips whole: past the chunks a
// protocol moves at once.
const bigSize = 3<<20 + 17

// Conform checks fsys behaves as vfs.FS says, in the folder dir it
// may fill: folders and files made, listed, stated, read and renamed,
// never over what is there, and removed, each failure the defined
// error.
func Conform(t *testing.T, fsys vfs.FS, dir string) {
	t.Helper()
	ctx := context.Background()
	at := func(name ...string) string { return path.Join(append([]string{dir}, name...)...) }
	must := func(err error, what string) {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	wantErr := func(err, want error, what string) {
		t.Helper()
		if !errors.Is(err, want) {
			t.Errorf("%s: %v, want %v", what, err, want)
		}
	}
	write := func(p string, data []byte) {
		t.Helper()
		w, err := fsys.Create(ctx, p)
		must(err, "create "+p)
		_, err = w.Write(data)
		must(errors.Join(err, w.Close()), "write "+p)
	}
	read := func(p string) []byte {
		t.Helper()
		f, err := fsys.Open(ctx, p)
		must(err, "open "+p)
		data, err := io.ReadAll(f)
		must(errors.Join(err, f.Close()), "read "+p)
		return data
	}

	must(fsys.Mkdir(ctx, at("a")), "mkdir a")
	wantErr(fsys.Mkdir(ctx, at("a")), fs.ErrExist, "making a again")
	wantErr(fsys.Mkdir(ctx, at("missing", "b")), fs.ErrNotExist, "making a folder in a missing one")

	write(at("a", "f.txt"), []byte("hello"))
	info, err := fsys.Stat(ctx, at("a", "f.txt"))
	must(err, "stat f.txt")
	if info.Name != "f.txt" || info.IsDir || info.Size != 5 || info.Modified.IsZero() {
		t.Errorf("f.txt states as %+v, want a 5 byte file with a time", info)
	}
	if _, err := fsys.Create(ctx, at("a", "f.txt")); !errors.Is(err, fs.ErrExist) {
		t.Errorf("creating f.txt again: %v, want %v", err, fs.ErrExist)
	}
	if got := read(at("a", "f.txt")); string(got) != "hello" {
		t.Errorf("f.txt reads %q, want hello", got)
	}
	if info, err := fsys.Stat(ctx, at("a")); err != nil || !info.IsDir {
		t.Errorf("a states as %+v, %v, want a folder", info, err)
	}
	_, err = fsys.Stat(ctx, at("nothing"))
	wantErr(err, fs.ErrNotExist, "stating nothing")
	_, err = fsys.Open(ctx, at("nothing"))
	wantErr(err, fs.ErrNotExist, "opening nothing")
	_, err = fsys.ReadDir(ctx, at("nothing"))
	wantErr(err, fs.ErrNotExist, "listing nothing")

	big := make([]byte, bigSize)
	for i := range big {
		big[i] = byte(i * 7)
	}
	write(at("a", "big.bin"), big)
	if got := read(at("a", "big.bin")); !bytes.Equal(got, big) {
		t.Errorf("a %d byte file read back %d bytes, not the same", len(big), len(got))
	}
	f, err := fsys.Open(ctx, at("a", "big.bin"))
	must(err, "open big.bin")
	if ra, ok := f.(io.ReaderAt); ok {
		chunk := make([]byte, 10)
		n, err := ra.ReadAt(chunk, 2<<20)
		if err != nil || n != 10 || !bytes.Equal(chunk, big[2<<20:2<<20+10]) {
			t.Errorf("ReadAt 2 MiB: %d %v, %x want %x", n, err, chunk, big[2<<20:2<<20+10])
		}
	}
	must(f.Close(), "close big.bin")

	must(fsys.Mkdir(ctx, at("a", "sub")), "mkdir a/sub")
	infos, err := fsys.ReadDir(ctx, at("a"))
	must(err, "list a")
	var names []string
	for _, i := range infos {
		names = append(names, i.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"big.bin", "f.txt", "sub"}) {
		t.Errorf("a lists %v, want big.bin f.txt sub", names)
	}

	must(fsys.Rename(ctx, at("a", "f.txt"), at("a", "g.txt")), "rename f.txt")
	_, err = fsys.Stat(ctx, at("a", "f.txt"))
	wantErr(err, fs.ErrNotExist, "stating f.txt renamed")
	wantErr(fsys.Rename(ctx, at("a", "g.txt"), at("a", "big.bin")), fs.ErrExist, "renaming over big.bin")
	if got := read(at("a", "big.bin")); len(got) != bigSize {
		t.Errorf("big.bin holds %d bytes after a refused rename over it, want %d", len(got), bigSize)
	}
	must(fsys.Rename(ctx, at("a", "sub"), at("a", "moved")), "rename a folder")

	if err := fsys.Remove(ctx, at("a")); err == nil {
		t.Error("removing a folder that holds files succeeded")
	}
	for _, name := range []string{"g.txt", "big.bin", "moved"} {
		must(fsys.Remove(ctx, at("a", name)), "remove "+name)
	}
	wantErr(fsys.Remove(ctx, at("a", "g.txt")), fs.ErrNotExist, "removing g.txt again")
	must(fsys.Remove(ctx, at("a")), "remove a")
	_, err = fsys.Stat(ctx, at("a"))
	wantErr(err, fs.ErrNotExist, "stating a removed")
}
