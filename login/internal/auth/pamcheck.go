package auth

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

var pamServiceDirs = []string{"/etc/pam.d", "/usr/lib/pam.d"}

var errPromptedInCheck = errors.New("the account check asked for input")

// CheckService reports whether the lock can authenticate user through
// service, without a password: the service is one of the system's, and
// its account stack accepts user. Nothing is authenticated, so no failed
// attempt is counted against user.
func CheckService(service, user string) error {
	return checkService(systemPAM{}, pamServiceDirs, service, user)
}

func checkService(lib pamLib, dirs []string, service, user string) error {
	if service == "" || service == "." || service == ".." || strings.ContainsRune(service, '/') {
		return fmt.Errorf("PAM service %q: not a service name", service)
	}
	if user == "" {
		return fmt.Errorf("PAM service %s: no user to check it for", service)
	}
	if err := findService(dirs, service); err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var prompted bool
	txn, err := lib.start(service, user, func(msgs []pamMessage) ([]pamReply, int32) {
		if slices.ContainsFunc(msgs, func(m pamMessage) bool { return m.style == pamPromptEchoOff || m.style == pamPromptEchoOn }) {
			prompted = true
			return nil, pamConvErr
		}
		return make([]pamReply, len(msgs)), pamSuccess
	})
	if err != nil {
		return fmt.Errorf("PAM service %s: start for %s: %w", service, user, err)
	}
	rc := txn.acctMgmt()
	defer txn.end(rc)
	switch {
	case prompted:
		return fmt.Errorf("PAM service %s: account of %s: %w", service, user, errPromptedInCheck)
	case rc != pamSuccess:
		return fmt.Errorf("PAM service %s: account of %s: %s", service, user, txn.strerror(rc))
	}
	return nil
}

func findService(dirs []string, service string) error {
	for _, dir := range dirs {
		path := filepath.Join(dir, service)
		info, err := os.Stat(path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			return fmt.Errorf("PAM service %s: %w", service, err)
		case info.IsDir():
			return fmt.Errorf("PAM service %s: %s is a directory", service, path)
		}
		return nil
	}
	return fmt.Errorf("PAM service %s: no such file in %s, so PAM would fall back to its \"other\" service", service, strings.Join(dirs, " or "))
}
