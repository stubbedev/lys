package settings

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/stubbedev/lys/settings/internal/accounts"
)

const pkexecProgram = "pkexec"

func usersHelperPath(exe string) string {
	return filepath.Join(filepath.Dir(filepath.Dir(exe)), "libexec", accounts.HelperName)
}

func changeAccount(ctx context.Context, r accounts.Request) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find %s beside lys-settings: %w", accounts.HelperName, err)
	}
	return runUsersHelper(ctx, []string{pkexecProgram, usersHelperPath(exe)}, r)
}

func runUsersHelper(ctx context.Context, helper []string, r accounts.Request) error {
	body, err := accounts.Encode(r)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, helper[0], append(helper[1:], string(r.Class()))...) //nolint:gosec // pkexec and lys-users-helper, whose class is a constant
	cmd.Stdin = bytes.NewReader(body)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		return accounts.Outcome(exit.ExitCode(), stdout.Bytes(), stderr.Bytes())
	}
	if err != nil {
		return fmt.Errorf("run %s: %w", helper[0], err)
	}
	return nil
}
