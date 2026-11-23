package screenshot

import (
	"context"
	"errors"
	"image"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	shelllocales "github.com/stubbedev/lys/desktop/i18n/locales/shell"
	"github.com/stubbedev/lys/lib/portaldialogs"
)

const noticeLocaleEnv = "LYS_SCREENSHOT_TEST_NOTICE_LOCALE"

func TestNoticesFollowTheDesktopLanguage(t *testing.T) {
	if loc := os.Getenv(noticeLocaleEnv); loc != "" {
		noticesIn(t, loc)
		return
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNoticesFollowTheDesktopLanguage$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), noticeLocaleEnv+"=fr", "LANGUAGE=fr")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the notices in fr: %v\n%s", err, out)
	}
}

func noticesIn(t *testing.T, loc string) {
	if langs := i18n.Shell().Languages(); len(langs) == 0 || langs[0].String() != loc {
		t.Fatalf("the shell negotiated %v, want %s first", langs, loc)
	}
	catalogue, err := i18n.Select(i18n.NewAssets(shelllocales.FS), []i18n.LangID{i18n.MustLangID(loc)})
	if err != nil {
		t.Fatal(err)
	}
	english, err := i18n.Select(i18n.NewAssets(shelllocales.FS), []i18n.LangID{i18n.Fallback})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"screenshot-saved", "screenshot-qr-code"} {
		if catalogue.Get(id) == english.Get(id) {
			t.Fatalf("%s reads %q in %s too: the test cannot tell the languages apart", id, english.Get(id), loc)
		}
	}

	const path = "/shots/a.png"
	saved := savedNotice(path, false)
	want := Notice{
		Summary: catalogue.Get("screenshot-saved"),
		Body:    path + "\n" + catalogue.Get("screenshot-not-copied"),
		Icon:    path,
	}
	if plain(saved) != plain(want) {
		t.Errorf("saved notice %q, want %q", plain(saved), plain(want))
	}
	if got := savedNotice(path, true); plain(got) != plain(Notice{Summary: want.Summary, Body: path, Icon: path}) {
		t.Errorf("saved notice without a copy failure %q", plain(got))
	}

	h, _, notified, _ := readHost(t, onWhite(200, 100, qrImage(t, "https://example.org/qr", 96), image.Pt(52, 2)))
	if _, err := h.Read(portaldialogs.Caller{}, ReadQR); err != nil {
		t.Fatal(err)
	}
	if len(*notified) != 1 || (*notified)[0].Summary != catalogue.Get("screenshot-qr-code") {
		t.Fatalf("a decoded code announced %+v, want the summary %q", *notified, catalogue.Get("screenshot-qr-code"))
	}

	h, _, notified, _ = readHost(t, onWhite(200, 100, nil, image.Point{}))
	if _, err := h.Read(portaldialogs.Caller{}, ReadQR); !errors.Is(err, ErrNoCode) {
		t.Fatalf("Read = %v, want no code found", err)
	}
	if len(*notified) != 0 {
		t.Fatalf("no code announced %+v", *notified)
	}
}
