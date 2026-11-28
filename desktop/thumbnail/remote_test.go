package thumbnail

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A file on a network location is fetched to be thumbnailed once:
// later its thumbnail comes from the cache under its own URI, with
// nothing fetched, until the file is modified; a fetch that fails is
// no thumbnail and records no failure.
func TestARemoteFileIsFetchedOnlyToMakeItsThumbnail(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	th := New(Config{})
	ctx := context.Background()
	local := photo(t, 40, 20)
	fetched := 0
	r := Remote{URI: "sftp://h/pics/photo.png", Modified: time.Unix(1_700_000_000, 0), Size: 100, Fetch: func(context.Context) (string, error) {
		fetched++
		return local, nil
	}}
	for range 2 {
		got, err := th.SquareRemote(ctx, r, "image/png", 24)
		if err != nil {
			t.Fatal(err)
		}
		if got.Image.Bounds().Dx() != 24 {
			t.Errorf("the thumbnail is %v, want a 24px square", got.Image.Bounds())
		}
		if err := th.Free(got); err != nil {
			t.Fatal(err)
		}
	}
	if fetched != 1 {
		t.Errorf("two thumbnails fetched the file %d times, want once", fetched)
	}
	if err := os.Remove(local); err != nil {
		t.Fatal(err)
	}
	if got, err := th.SquareRemote(ctx, r, "image/png", 24); err != nil || fetched != 1 {
		t.Errorf("with the copy gone the cached thumbnail = %v, %v after %d fetches", got.Image, err, fetched)
	} else if err := th.Free(got); err != nil {
		t.Fatal(err)
	}

	refused := errors.New("not connected")
	r.Modified = r.Modified.Add(time.Hour)
	r.Fetch = func(context.Context) (string, error) { fetched++; return "", refused }
	for range 2 {
		if _, err := th.SquareRemote(ctx, r, "image/png", 24); !errors.Is(err, refused) {
			t.Errorf("a modified file that cannot be fetched: %v, want %v", err, refused)
		}
	}
	if fetched != 3 {
		t.Errorf("a failed fetch was recorded: %d fetches, want both tried", fetched)
	}
	if entries, _ := os.ReadDir(filepath.Join(os.Getenv("XDG_CACHE_HOME"), "thumbnails", "fail", software)); len(entries) != 0 {
		t.Errorf("the failed fetches left %d fail records", len(entries))
	}
	if live := th.Live(); live != 0 {
		t.Errorf("%d thumbnails left allocated", live)
	}
}
