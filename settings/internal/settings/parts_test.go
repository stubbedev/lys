package settings

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/identity"
)

func pageIDs(installed parts) []string {
	var out []string
	for _, s := range layout(config.Defaults(), installed) {
		for _, p := range s.pages {
			out = append(out, p.id)
		}
	}
	return out
}

func rowPaths(installed parts, page string) []string {
	p, ok := pageByID(layout(config.Defaults(), installed), page)
	if !ok {
		return nil
	}
	var out []string
	for _, s := range p.sections {
		for _, r := range s.flatRows() {
			out = append(out, settingPath(p, r))
		}
	}
	return out
}

func TestEachPartShowsOnlyItsOwnPagesAndRows(t *testing.T) {
	shellOnly, wmOnly, neither := parts{shell: true}, parts{wm: true}, parts{}
	for _, tc := range []struct {
		name          string
		installed     parts
		pages, absent []string
	}{
		{"both", bothParts, []string{"bar-general", "theme", "wm-windows", displaysID, "users"}, nil},
		{"lys-shell alone", shellOnly, []string{"bar-general", "theme", "notifications", "users", soundID}, []string{"wm-windows", "wm-input", displaysID}},
		{"lys-wm alone", wmOnly, []string{"wm-windows", "wm-input", displaysID, "users", soundID, loginItemsID}, []string{"bar-general", "theme", "notifications", "lock"}},
		{"neither", neither, []string{"users", soundID, dateTimeID}, []string{"bar-general", "wm-windows", "lock"}},
	} {
		ids := pageIDs(tc.installed)
		for _, id := range tc.pages {
			if !slices.Contains(ids, id) {
				t.Errorf("%s: page %s is missing", tc.name, id)
			}
		}
		for _, id := range tc.absent {
			if slices.Contains(ids, id) {
				t.Errorf("%s: page %s is shown", tc.name, id)
			}
		}
	}
}

func TestAMixedPageKeepsTheRowsOfThePartsInstalled(t *testing.T) {
	if rows := rowPaths(parts{shell: true}, loginItemsID); slices.Contains(rows, "wm:autostart") {
		t.Errorf("lys-shell alone: login items list lys-wm's autostart: %v", rows)
	}
	if rows := rowPaths(parts{wm: true}, loginItemsID); !slices.Contains(rows, "wm:autostart") {
		t.Errorf("lys-wm alone: login items lack lys-wm's autostart: %v", rows)
	}
	if rows := rowPaths(parts{wm: true}, soundID); len(rows) != 0 {
		t.Errorf("lys-wm alone: the sound page shows lys-shell's alert rows %v", rows)
	}
	if rows := rowPaths(parts{shell: true}, soundID); !slices.Contains(rows, "modules.notifications.sounds") {
		t.Errorf("lys-shell alone: the sound page lacks its alert rows: %v", rows)
	}
}

func TestSearchAndSettingLinksFollowTheInstalledParts(t *testing.T) {
	if err := checkSetting(parts{shell: true}, "", "wm:gaps.inner"); err == nil {
		t.Error("lys-shell alone: a lys-wm setting link resolved")
	}
	if err := checkSetting(parts{wm: true}, "", "bar.location"); err == nil {
		t.Error("lys-wm alone: a lys-shell setting link resolved")
	}
	if err := checkSetting(bothParts, "", "wm:gaps.inner"); err != nil {
		t.Errorf("both: %v", err)
	}
	listed := 0
	for _, p := range pages(config.Defaults(), parts{wm: true}) {
		for _, s := range p.Settings {
			listed++
			if !strings.HasPrefix(s.Path, wmSettingPrefix) {
				t.Errorf("lys-wm alone lists lys-shell's %s on %s", s.Path, p.ID)
			}
		}
	}
	if listed == 0 {
		t.Error("lys-wm alone lists no setting")
	}
}

func TestTheInstalledPartsAreWhatThePathHolds(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	if got := installedParts(); got != (parts{}) {
		t.Fatalf("an empty PATH: %+v", got)
	}
	if err := os.WriteFile(filepath.Join(dir, identity.Name), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := installedParts(); got != (parts{shell: true}) {
		t.Fatalf("lys-shell on the PATH: %+v", got)
	}
}
