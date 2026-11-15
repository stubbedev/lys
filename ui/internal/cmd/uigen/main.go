// Command uigen writes package ui's generated builders: go generate in
// ui runs it.
package main

import (
	"log"
	"os"

	"github.com/stubbedev/lys/lib/atomicfile"

	"github.com/stubbedev/lys/ui/internal/uigen"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatal("usage: uigen <widget dir> <output file>")
	}
	src, err := uigen.Generate(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	if err := atomicfile.WriteFile(os.Args[2], src, 0o644); err != nil {
		log.Fatal(err)
	}
}
