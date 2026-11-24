package usershelper

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/useraccount"
	"github.com/stubbedev/lys/lib/atomicfile"

	"github.com/stubbedev/lys/settings/internal/accounts"
)

var (
	sbinDir = "/usr/sbin"
	binDir  = "/usr/bin"
)

const (
	runTimeout = 10 * time.Minute
	maxRequest = 2*accounts.MaxPictureBytes + 64<<10
)

type system struct {
	sbin, bin, avatars string
}

func (s system) path(t tool) string {
	if t == gpasswd {
		return filepath.Join(s.bin, string(t))
	}
	return filepath.Join(s.sbin, string(t))
}

func (s system) apply(ctx context.Context, st step) error {
	switch st := st.(type) {
	case command:
		return s.run(ctx, st)
	case writeAvatar:
		if err := os.MkdirAll(s.avatars, 0o755); err != nil { //nolint:gosec // the greeter reads the avatars
			return fmt.Errorf("create %s: %w", s.avatars, err)
		}
		path := filepath.Join(s.avatars, st.login)
		if err := atomicfile.WriteFile(path, st.picture, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		return nil
	case removeAvatar:
		path := filepath.Join(s.avatars, st.login)
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
		return nil
	}
	return fmt.Errorf("no way to apply a %T", st)
}

func (s system) run(ctx context.Context, c command) error {
	path := s.path(c.tool)
	cmd := exec.CommandContext(ctx, path, c.args...) //nolint:gosec // a shadow tool with argv the plan built from validated values
	cmd.Env = []string{"PATH=" + s.sbin + ":" + s.bin, "LC_ALL=C"}
	cmd.Stdin = bytes.NewReader(c.stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", path, strings.Join(c.args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s system) carryOut(ctx context.Context, p plan) *accounts.Error {
	for i, st := range p.steps {
		err := s.apply(ctx, st)
		if err == nil {
			continue
		}
		failed := accounts.Errorf(accounts.ErrFailed, "%v", err)
		if i == 0 || len(p.rollback) == 0 {
			return failed
		}
		for _, undo := range p.rollback {
			if uerr := s.apply(ctx, undo); uerr != nil {
				failed.Detail += "; undoing it: " + uerr.Error()
			}
		}
		return failed
	}
	return nil
}

// Main is lys-users-helper with args after the program name, the
// request on stdin, an accounts.Error as JSON on stdout when it refused
// or failed, and pkexecUID and euid as pkexec left them. It exits 0
// once the change is made, 1 when it is refused or failed, and 2 on a
// malformed command line.
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer, pkexecUID string, euid int) int {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "usage: %s self|others < REQUEST\n", accounts.HelperName)
		return 2
	}
	class, err := accounts.ParseClass(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", accounts.HelperName, err)
		return 2
	}
	sys := system{sbin: sbinDir, bin: binDir, avatars: useraccount.AvatarsDir}
	if aerr := serve(class, stdin, pkexecUID, euid, accounts.ReadDatabase, sys); aerr != nil {
		fmt.Fprintf(stderr, "%s: %v\n", accounts.HelperName, aerr)
		if err := json.NewEncoder(stdout).Encode(aerr); err != nil {
			fmt.Fprintf(stderr, "%s: write the answer: %v\n", accounts.HelperName, err)
		}
		return 1
	}
	return 0
}

func serve(class accounts.Class, stdin io.Reader, pkexecUID string, euid int, read func() (accounts.Database, error), sys system) *accounts.Error {
	if euid != 0 {
		return accounts.Errorf(accounts.ErrInvalid, "runs as root, through pkexec, not as uid %d", euid)
	}
	uid, err := strconv.ParseUint(pkexecUID, 10, 32)
	if err != nil {
		return accounts.Errorf(accounts.ErrInvalid, "PKEXEC_UID %q names no caller: run it through pkexec", pkexecUID)
	}
	data, err := io.ReadAll(io.LimitReader(stdin, maxRequest+1))
	if err != nil {
		return accounts.Errorf(accounts.ErrInvalid, "read the request: %v", err)
	}
	if len(data) > maxRequest {
		return accounts.Errorf(accounts.ErrInvalid, "the request is over %d bytes", maxRequest)
	}
	r, err := accounts.Decode(class, data)
	if err != nil {
		return accounts.Errorf(accounts.ErrInvalid, "%v", err)
	}
	db, err := read()
	if err != nil {
		return accounts.Errorf(accounts.ErrFailed, "%v", err)
	}
	caller, ok := db.ByUID(uint32(uid))
	if !ok {
		return accounts.Errorf(accounts.ErrUnknownUser, "the caller, uid %d, is in no passwd entry", uid)
	}
	p, aerr := makePlan(db, caller, r)
	if aerr != nil {
		return aerr
	}
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()
	return sys.carryOut(ctx, p)
}
