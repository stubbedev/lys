package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/stubbedev/lys/lib/atomicfile"
	"github.com/stubbedev/lys/lib/srcroot"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "gopackages:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("gopackages", flag.ContinueOnError)
	check := flags.Bool("check", false, "fail when the file differs from what the tree gives, instead of writing it")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: gopackages [-check] <file>")
	}
	target := flags.Arg(0)
	root, module, err := srcroot.Module(".")
	if err != nil {
		return err
	}
	tracked, err := trackedFiles(root)
	if err != nil {
		return err
	}
	packages, err := scan(os.DirFS(root), module, tracked)
	if err != nil {
		return err
	}
	want, err := encode(packages)
	if err != nil {
		return err
	}
	if !*check {
		return atomicfile.WriteFile(target, want, 0o644)
	}
	have, err := os.ReadFile(target) //nolint:gosec // the file the recipe names
	if err != nil {
		return fmt.Errorf("read %s: %w", target, err)
	}
	if !bytes.Equal(have, want) {
		return fmt.Errorf("%s does not match the Go packages of the tree: run just go-packages", target)
	}
	return nil
}

func trackedFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files in %s: %w", root, err)
	}
	return strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00"), nil
}

func encode(packages map[string]Package) ([]byte, error) {
	data, err := json.MarshalIndent(packages, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode the Go packages: %w", err)
	}
	return append(data, '\n'), nil
}
