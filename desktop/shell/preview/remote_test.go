package preview

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A file on a network location loads as the copy Sources.Remote
// fetches and a folder there as what its listing found; one that cannot
// be fetched, or with nothing to reach it through, is Unreadable.
func TestARemoteFileLoadsThroughItsCopy(t *testing.T) {
	db := testDB(t)
	local := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(local, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	modified := time.Unix(1_700_000_000, 0)
	refused := errors.New("not connected")
	src := Sources{MIME: db, Pictures: NewPictures(PictureBudget), Remote: func(_ context.Context, loc string) (Fetched, error) {
		switch loc {
		case "sftp://h/notes.txt":
			return Fetched{Local: local}, nil
		case "sftp://h/dir":
			return Fetched{Listed: &Listed{Items: 3, Bytes: 10, Partial: true, Modified: modified}}, nil
		}
		return Fetched{}, refused
	}}
	ctx := context.Background()
	if c := Load(ctx, src, Request{Path: "sftp://h/notes.txt"}); c.Renderer() != Text || c.text != "hello\n" {
		t.Errorf("a remote text file loads as %v %q", c.Renderer(), c.text)
	}
	c := Load(ctx, src, Request{Path: "sftp://h/dir"})
	if c.Renderer() != Folder || c.folder != (folderStats{items: 3, bytes: 10, bytesMore: true}) || !c.info.Dir || !c.info.Modified.Equal(modified) {
		t.Errorf("a remote folder loads as %v %+v %+v", c.Renderer(), c.folder, c.info)
	}
	if c := Load(ctx, src, Request{Path: "sftp://h/gone.txt"}); c.Renderer() != Failed || c.failure != Unreadable || !errors.Is(c.err, refused) {
		t.Errorf("a remote file not fetched loads as %v %v %v", c.Renderer(), c.failure, c.err)
	}
	src.Remote = nil
	if c := Load(ctx, src, Request{Path: "sftp://h/notes.txt"}); c.Renderer() != Failed || !errors.Is(c.err, errNoRemote) {
		t.Errorf("a remote file with nothing to reach it loads as %v %v", c.Renderer(), c.err)
	}
}
