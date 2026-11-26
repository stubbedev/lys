package usershelper

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/useraccount"
	"github.com/stubbedev/lys/lib/intconv"

	"github.com/stubbedev/lys/settings/internal/pwfiles"
)

// defaults is what login.defs and /etc/default/useradd say a new
// account is, where each says nothing being shadow-utils' default.
type defaults struct {
	uids, gids pwfiles.Range
	// userGroups is USERGROUPS_ENAB: a new account gets a group of its
	// own; else its primary group is group, GROUP of default/useradd.
	userGroups bool
	group      string
	homeBase   string
	shell      string
	homeMode   fs.FileMode
	// passMin, passMax and passWarn are the shadow fields, "" for none.
	passMin, passMax, passWarn string
}

// The paths in /etc that hold the defaults, the skeleton of a new home,
// and the file whose presence makes the system NixOS.
const (
	loginDefsName = "login.defs"
	useraddName   = "default/useradd"
	skelName      = "skel"
	nixosName     = "NIXOS"
)

// The defaults where the files say nothing: shadow-utils' compiled
// ones, and the shell an empty passwd field means (passwd(5)).
const (
	defaultHomeBase = "/home"
	defaultShell    = "/bin/sh"
	defaultUmask    = 0o022
)

var defaultGIDs = pwfiles.Range{Min: 1000, Max: 60000}

func keyValues(text string, split func(string) (string, string, bool)) map[string]string {
	out := map[string]string{}
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := split(line); ok {
			if _, seen := out[k]; !seen {
				out[k] = strings.Trim(strings.TrimSpace(v), `"`)
			}
		}
	}
	return out
}

func loginDefsField(line string) (string, string, bool) {
	f := strings.Fields(line)
	if len(f) < 2 {
		return "", "", false
	}
	return f[0], f[1], true
}

func useraddField(line string) (string, string, bool) { return strings.Cut(line, "=") }

func parseDefaults(loginDefs, useradd string) (defaults, error) {
	uids, err := useraccount.ParseUIDRange(loginDefs)
	if err != nil {
		return defaults{}, fmt.Errorf("%s: %w", loginDefsName, err)
	}
	defs := keyValues(loginDefs, loginDefsField)
	ua := keyValues(useradd, useraddField)
	d := defaults{
		uids:       pwfiles.Range{Min: uids.Min, Max: uids.Max},
		gids:       defaultGIDs,
		userGroups: strings.EqualFold(defs["USERGROUPS_ENAB"], "yes"),
		group:      ua["GROUP"],
		homeBase:   defaultHomeBase,
		shell:      defaultShell,
	}
	if v := ua["HOME"]; v != "" {
		d.homeBase = v
	}
	if v := ua["SHELL"]; v != "" {
		d.shell = v
	}
	if !filepath.IsAbs(d.homeBase) {
		return defaults{}, fmt.Errorf("%s: HOME %q is not an absolute path", useraddName, d.homeBase)
	}
	for key, into := range map[string]*uint32{"GID_MIN": &d.gids.Min, "GID_MAX": &d.gids.Max} {
		if v, ok := defs[key]; ok {
			n, err := strconv.ParseUint(v, 0, 32)
			if err != nil {
				return defaults{}, fmt.Errorf("%s: %s %q: %w", loginDefsName, key, v, err)
			}
			*into = uint32(n)
		}
	}
	d.homeMode = 0o777 &^ defaultUmask
	if v, ok := defs["UMASK"]; ok {
		umask, err := permission("UMASK", v)
		if err != nil {
			return defaults{}, err
		}
		d.homeMode = 0o777 &^ umask
	}
	if v, ok := defs["HOME_MODE"]; ok {
		if d.homeMode, err = permission("HOME_MODE", v); err != nil {
			return defaults{}, err
		}
	}
	for key, into := range map[string]*string{"PASS_MIN_DAYS": &d.passMin, "PASS_MAX_DAYS": &d.passMax, "PASS_WARN_AGE": &d.passWarn} {
		v, ok := defs[key]
		if !ok {
			continue
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < -1 {
			return defaults{}, fmt.Errorf("%s: %s %q is no count of days", loginDefsName, key, v)
		}
		if n >= 0 {
			*into = strconv.FormatInt(n, 10)
		}
	}
	return d, nil
}

func permission(key, v string) (fs.FileMode, error) {
	n, err := strconv.ParseUint(v, 8, 32)
	if err != nil || n > 0o777 {
		return 0, fmt.Errorf("%s: %s %q is no octal permission", loginDefsName, key, v)
	}
	return fs.FileMode(intconv.Must[uint32](n)), nil
}

func readOptional(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // a fixed file of the system's /etc
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(data), nil
}

func readDefaults(etc string) (defaults, error) {
	loginDefs, err := readOptional(filepath.Join(etc, loginDefsName))
	if err != nil {
		return defaults{}, err
	}
	useradd, err := readOptional(filepath.Join(etc, useraddName))
	if err != nil {
		return defaults{}, err
	}
	return parseDefaults(loginDefs, useradd)
}
