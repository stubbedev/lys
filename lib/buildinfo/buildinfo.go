// Package buildinfo is the lys release a binary was built from. Every
// lys component is released under one version; a build stamps it with
// -ldflags "-X github.com/stubbedev/lys/lib/buildinfo.version=X.Y.Z",
// and `go install github.com/stubbedev/lys/...@vX.Y.Z` records it in the
// binary without any flag.
package buildinfo

import (
	"runtime/debug"
	"strings"
)

var version string

// Dev is the version of a binary built from a checkout without a stamp.
const Dev = "dev"

// Version is the release, without a leading v: the stamped one, else
// the module version the Go toolchain recorded, else Dev.
func Version() string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Dev
	}
	return fromModule(info.Main.Version)
}

func fromModule(v string) string {
	if v == "" || v == "(devel)" {
		return Dev
	}
	return strings.TrimPrefix(v, "v")
}
