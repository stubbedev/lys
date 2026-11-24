package edid

import (
	"cmp"
	"slices"
)

//go:generate go run ./internal/genpnp -out pnp_gen.go

type vendor struct{ id, name string }

// Vendor is the manufacturer name the UEFI PNP ID registry, as hwdata
// carries it, gives three-letter PNP ID pnp, and whether it lists pnp.
func Vendor(pnp string) (string, bool) {
	i, ok := slices.BinarySearchFunc(vendors[:], pnp, func(v vendor, id string) int { return cmp.Compare(v.id, id) })
	if !ok {
		return "", false
	}
	return vendors[i].name, true
}
