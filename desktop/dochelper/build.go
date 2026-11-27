package dochelper

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
)

// goEnv is the environment Program is built in past CGO_ENABLED=0, as
// a JSON object of variables, which its Nix package reads too.
//
//go:embed goenv.json
var goEnv []byte

// BuildEnv is the environment Program is built in past CGO_ENABLED=0,
// as KEY=value entries in the order of their keys: its Nix package's,
// for the tests that build it to build it the same way. It leaves out
// the Green Tea collector, which dies of a segmentation fault where
// the memory limit Program holds itself to should end it as out of
// memory.
func BuildEnv() ([]string, error) {
	var vars map[string]string
	if err := json.Unmarshal(goEnv, &vars); err != nil {
		return nil, fmt.Errorf("%s: reading its build environment: %w", Program, err)
	}
	env := make([]string, 0, len(vars))
	for _, k := range slices.Sorted(maps.Keys(vars)) {
		env = append(env, k+"="+vars[k])
	}
	return env, nil
}
