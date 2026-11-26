package main

import (
	"flag"
	"log"
	"os"

	"github.com/stubbedev/lys/lib/atomicfile"

	"github.com/stubbedev/lys/settings/internal/land"
)

func main() {
	src := flag.String("src", "", "path of Natural Earth's ne_110m_land.geojson")
	out := flag.String("out", "", "file to write")
	flag.Parse()
	if *src == "" || *out == "" {
		log.Fatal("genland: -src and -out are required")
	}
	data, err := os.ReadFile(*src)
	if err != nil {
		log.Fatalf("genland: %v", err)
	}
	asset, err := land.Generate(data)
	if err != nil {
		log.Fatalf("genland: %s: %v", *src, err)
	}
	if err := atomicfile.WriteFile(*out, asset, 0o644); err != nil {
		log.Fatalf("genland: %v", err)
	}
}
