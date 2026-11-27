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
	// subIDs is how a new account's subordinate IDs of each kind are
	// allocated, and subIDModule the NSS module nsswitch.conf names
	// for them, "" where they are the files'.
	subIDs      [2]subIDRule
	subIDModule string
	// mailDir is the folder of the mail spools, "" where mail lives in
	// the home folder (MAIL_FILE).
	mailDir string
}

// subIDRule is SUB_UID_* or SUB_GID_* of login.defs: count IDs in ids,
// at the lowest free start, or deterministic, at the start the new
// account's offset from UID_MIN picks, wrapping around the range where
// wrap is set. storeByUID names the owner by its user ID.
type subIDRule struct {
	ids                             pwfiles.Range
	count                           uint32
	deterministic, wrap, storeByUID bool
}

// The paths in /etc that hold the defaults, the skeleton of a new home,
// the file whose presence makes the system NixOS, and the name service
// switch.
const (
	loginDefsName = "login.defs"
	useraddName   = "default/useradd"
	skelName      = "skel"
	nixosName     = "NIXOS"
	nsswitchName  = "nsswitch.conf"
)

// The defaults where the files say nothing: shadow-utils' compiled
// ones, the shell an empty passwd field means (passwd(5)), and the
// mail spool of paths.h.
const (
	defaultHomeBase = "/home"
	defaultShell    = "/bin/sh"
	defaultUmask    = 0o022
	defaultMailDir  = "/var/mail"
	defaultSubCount = 65536
)

var (
	defaultGIDs = pwfiles.Range{Min: 1000, Max: 60000}
	// The subordinate IDs useradd allocates where login.defs names no
	// range: the lowest free start's, and the deterministic start's.
	defaultSubIDs              = pwfiles.Range{Min: 100000, Max: 600100000}
	defaultDeterministicSubIDs = pwfiles.Range{Min: 65536, Max: 4294967295}
)

// subIDKinds is each kind's word in login.defs' keys.
var subIDKinds = [...]string{pwfiles.SubUIDs: "UID", pwfiles.SubGIDs: "GID"}

func keyValues(text string, split func(string) (string, string, bool)) map[string]string {
	out := map[string]string{}
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := split(line); ok {
			out[k] = strings.Trim(strings.TrimSpace(v), `"`)
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

func enabled(defs map[string]string, key string) bool { return strings.EqualFold(defs[key], "yes") }

func loginDefsID(defs map[string]string, key string, into *uint32) error {
	v, ok := defs[key]
	if !ok {
		return nil
	}
	n, err := strconv.ParseUint(v, 0, 32)
	if err != nil {
		return fmt.Errorf("%s: %s %q: %w", loginDefsName, key, v, err)
	}
	*into = uint32(n)
	return nil
}

func parseSubIDRule(defs map[string]string, kind string) (subIDRule, error) {
	r := subIDRule{
		ids: defaultSubIDs, count: defaultSubCount,
		deterministic: enabled(defs, "SUB_"+kind+"_DETERMINISTIC"),
		wrap:          enabled(defs, "UNSAFE_SUB_"+kind+"_DETERMINISTIC_WRAP"),
		storeByUID:    enabled(defs, "SUB_"+kind+"_STORE_BY_UID"),
	}
	if r.deterministic {
		r.ids = defaultDeterministicSubIDs
	}
	for _, f := range []struct {
		key  string
		into *uint32
	}{{"MIN", &r.ids.Min}, {"MAX", &r.ids.Max}, {"COUNT", &r.count}} {
		if err := loginDefsID(defs, "SUB_"+kind+"_"+f.key, f.into); err != nil {
			return subIDRule{}, err
		}
	}
	return r, nil
}

// parseNSSwitchSubID is the NSS module the subid line of nsswitch.conf
// names, as shadow-utils reads it: the first word of the first such
// line that names one, "" for none or for files.
func parseNSSwitchSubID(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.HasPrefix(line, "#") || !strings.EqualFold(key, "subid") {
			continue
		}
		module, _, _ := strings.Cut(strings.TrimLeft(value, " \t"), " ")
		module, _, _ = strings.Cut(module, "\t")
		if module == "" {
			continue
		}
		if module == "files" {
			return ""
		}
		return module
	}
	return ""
}

func parseMailDir(defs map[string]string) (string, error) {
	dir, ok := defs["MAIL_DIR"]
	if !ok {
		if _, inHome := defs["MAIL_FILE"]; inHome {
			return "", nil
		}
		return defaultMailDir, nil
	}
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("%s: MAIL_DIR %q is not an absolute path", loginDefsName, dir)
	}
	return filepath.Clean(dir), nil
}

func parseDefaults(loginDefs, useradd, nsswitch string) (defaults, error) {
	uids, err := useraccount.ParseUIDRange(loginDefs)
	if err != nil {
		return defaults{}, fmt.Errorf("%s: %w", loginDefsName, err)
	}
	defs := keyValues(loginDefs, loginDefsField)
	ua := keyValues(useradd, useraddField)
	d := defaults{
		uids:       pwfiles.Range{Min: uids.Min, Max: uids.Max},
		gids:       defaultGIDs,
		userGroups: enabled(defs, "USERGROUPS_ENAB"),
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
	if err := loginDefsID(defs, "GID_MIN", &d.gids.Min); err != nil {
		return defaults{}, err
	}
	if err := loginDefsID(defs, "GID_MAX", &d.gids.Max); err != nil {
		return defaults{}, err
	}
	for _, ids := range []pwfiles.SubIDs{pwfiles.SubUIDs, pwfiles.SubGIDs} {
		if d.subIDs[ids], err = parseSubIDRule(defs, subIDKinds[ids]); err != nil {
			return defaults{}, err
		}
	}
	d.subIDModule = parseNSSwitchSubID(nsswitch)
	if d.mailDir, err = parseMailDir(defs); err != nil {
		return defaults{}, err
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
	nsswitch, err := readOptional(filepath.Join(etc, nsswitchName))
	if err != nil {
		return defaults{}, err
	}
	return parseDefaults(loginDefs, useradd, nsswitch)
}
