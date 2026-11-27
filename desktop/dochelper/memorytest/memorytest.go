// Package memorytest builds hog, a program that holds itself to the
// memory of a lys-docthumb request as lys-docthumb does and allocates,
// for the tests of what a helper past its memory dies of. hog is built
// in lys-docthumb's environment, since the runtime a test binary is
// built with can die of a segmentation fault at the memory limit where
// lys-docthumb's runs out of memory.
package memorytest

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/stubbedev/lys/lib/buildtest"

	"github.com/stubbedev/lys/desktop/dochelper"
)

// Allocate names, in hog's environment, the bytes it allocates, in
// Step steps, keeping each; without it, hog allocates twice the
// request's memory, which one allocation crossing the limit cannot
// survive, since the kernel holds a mapping to the limit only once the
// next one is made.
const Allocate = "LYS_MEMORYTEST_ALLOCATE"

// Step is the size of each allocation hog keeps.
const Step = 4 << 20

// Ballast is the data hog maps before it limits itself, past the
// memory it then holds itself to, as a program's own start-up mappings
// may be.
const Ballast = 1 << 30

const command = "hog"

var build = sync.OnceValues(func() (*buildtest.Dir, error) {
	env, err := dochelper.BuildEnv()
	if err != nil {
		return nil, err
	}
	d, err := buildtest.Hold("lys-memorytest")
	if err != nil {
		return nil, err
	}
	if err := d.BuildWith(env, "./desktop/dochelper/memorytest/"+command); err != nil {
		return nil, fmt.Errorf("memorytest: build %s: %w", command, err)
	}
	return d, nil
})

// Hog is the path of hog, built by the first call of the process into
// a directory it holds until it ends; later calls share that build.
func Hog() (string, error) {
	d, err := build()
	if err != nil {
		return "", err
	}
	return filepath.Join(d.Path(), command), nil
}
