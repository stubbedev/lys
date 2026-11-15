// Command scanner generates Wayland protocol bindings from the XML listed in
// internal/protocol/protocols.json.
package main

//go:generate go run . -dir ../protocol

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"

	xml "github.com/stubbedev/lys/lib/protocols"
	"github.com/stubbedev/lys/lib/srcroot"
)

func main() {
	dir := flag.String("dir", "internal/protocol", "directory holding protocols.json")
	flag.Parse()
	if err := run(*dir); err != nil {
		fmt.Fprintln(os.Stderr, "scanner:", err)
		os.Exit(1)
	}
}

func run(dir string) error {
	m, err := loadManifest(dir)
	if err != nil {
		return err
	}
	_, module, err := srcroot.Component(dir)
	if err != nil {
		return err
	}
	protocols, err := loadProtocols(m)
	if err != nil {
		return err
	}
	for _, s := range sides {
		files := map[string][]byte{}
		for _, p := range protocols {
			src, err := generate(p, s, module)
			if err != nil {
				return err
			}
			files[path.Join(p.pkg, "protocol.go")] = src
		}
		if err := replaceTree(filepath.Join(dir, s.dir()), files); err != nil {
			return err
		}
	}
	return nil
}

func loadProtocols(m *manifest) ([]*protocol, error) {
	var protocols []*protocol
	for _, e := range m.Protocols {
		f, err := xml.FS.Open(e.File)
		if err != nil {
			return nil, fmt.Errorf("open protocol XML: %w", err)
		}
		p, err := parseProtocol(f, path.Join("lys-lib/protocols", e.File), e.Package)
		_ = f.Close()
		if err != nil {
			return nil, err
		}
		protocols = append(protocols, p)
	}
	if err := resolve(protocols); err != nil {
		return nil, err
	}
	return protocols, nil
}

func replaceTree(dst string, files map[string][]byte) error {
	tmp, err := os.MkdirTemp(filepath.Dir(dst), ".scanner-")
	if err != nil {
		return fmt.Errorf("create temporary tree for %s: %w", dst, err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	for name, data := range files {
		p := filepath.Join(tmp, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", p, err)
		}
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return fmt.Errorf("chmod %s: %w", tmp, err)
	}
	if err := os.RemoveAll(dst); err != nil {
		return fmt.Errorf("remove old %s: %w", dst, err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return fmt.Errorf("move generated tree into %s: %w", dst, err)
	}
	return nil
}
