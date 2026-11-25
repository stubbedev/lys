package udisks

import (
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	shelllocales "github.com/stubbedev/lys/desktop/i18n/locales/shell"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

func loaderFor(t *testing.T, locale string) *i18n.Loader {
	t.Helper()
	l, err := i18n.Select(i18n.NewAssets(shelllocales.FS), []i18n.LangID{i18n.MustLangID(locale)})
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// unisolated drops the isolation marks Fluent wraps placeables in.
var unisolated = strings.NewReplacer("\u2068", "", "\u2069", "")

func TestAnUnlabelledVolumeIsNamedBySizeInTheLocalesLanguageAndUnits(t *testing.T) {
	plain := Volume{Size: 16_000_000_000}
	locked := Volume{Size: 500_107_862_016, Locked: true}
	for _, c := range []struct {
		locale        string
		plain, locked string
	}{
		{"en-US", "16.0 GB Volume", "500.1 GB Encrypted Volume"},
		{"de", "Datenträger mit 16,0 GB", "Verschlüsselter Datenträger mit 500,1 GB"},
		{"fr", "Volume de 16,0 Go", "Volume chiffré de 500,1 Go"},
		{"nl", "Volume van 16,0 GB", "Versleuteld volume van 500,1 GB"},
	} {
		l := loaderFor(t, c.locale)
		if got := unisolated.Replace(sizeLabel(l, plain)); got != c.plain {
			t.Errorf("%s plain = %q, want %q", c.locale, got, c.plain)
		}
		if got := unisolated.Replace(sizeLabel(l, locked)); got != c.locked {
			t.Errorf("%s locked = %q, want %q", c.locale, got, c.locked)
		}
	}
}

func TestAVolumeOfUnknownSizeIsNamedWithoutOne(t *testing.T) {
	for locale, want := range map[string][2]string{
		"en-US": {"Volume", "Encrypted Volume"},
		"de":    {"Datenträger", "Verschlüsselter Datenträger"},
	} {
		l := loaderFor(t, locale)
		if got := sizeLabel(l, Volume{}); got != want[0] {
			t.Errorf("%s plain = %q, want %q", locale, got, want[0])
		}
		if got := sizeLabel(l, Volume{Locked: true}); got != want[1] {
			t.Errorf("%s locked = %q, want %q", locale, got, want[1])
		}
	}
}

func TestAVolumeIsSizedInDecimalNotBinaryUnits(t *testing.T) {
	got := unisolated.Replace(sizeLabel(loaderFor(t, "en-US"), Volume{Size: 1_000_000_000}))
	if got != "1.0 GB Volume" || strings.Contains(got, "GiB") {
		t.Errorf("a billion bytes = %q, want 1.0 GB, never GiB", got)
	}
}

func TestALabelledVolumeKeepsItsOwnName(t *testing.T) {
	objs := objects{
		"/org/freedesktop/UDisks2/block_devices/sdb1": {
			BlockIface:      {"IdLabel": dbus.MakeVariant("STICK"), "Size": dbus.MakeVariant(uint64(16_000_000_000))},
			FilesystemIface: {},
		},
	}
	vols := volumesOf(loaderFor(t, "de"), objs)
	if len(vols) != 1 || vols[0].Label != "STICK" {
		t.Errorf("volumes = %+v, want the stick under its own label in any language", vols)
	}
}
