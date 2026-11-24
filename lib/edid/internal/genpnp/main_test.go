package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestTheCheckedInTableIsHwdatasRegistry(t *testing.T) {
	ids := os.Getenv("LYS_HWDATA_PNP_IDS")
	if ids == "" {
		t.Fatal("LYS_HWDATA_PNP_IDS is not set: the dev shell points it at hwdata's pnp.ids")
	}
	src, err := os.ReadFile(ids)
	if err != nil {
		t.Fatal(err)
	}
	want, _, err := generate(src)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join("..", "..", "pnp_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("lib/edid/pnp_gen.go is stale: run go generate ./edid")
	}
}

func TestTheTableIsSortedAndLeavesOutWhatNoEDIDEncodes(t *testing.T) {
	text, unencodable, err := generate([]byte("ZZZ\tBoca Research Inc\nAAA\tAvolites Ltd \ninu\tInovatec S.p.A.\nVQ@\tVision Quest\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(unencodable, []string{"inu"}) {
		t.Fatalf("left out %q, want inu", unencodable)
	}
	want := "\t{\"AAA\", \"Avolites Ltd\"},\n\t{\"VQ@\", \"Vision Quest\"},\n\t{\"ZZZ\", \"Boca Research Inc\"},\n}\n"
	if !strings.HasSuffix(string(text), want) || strings.Contains(string(text), "inu") {
		t.Fatalf("generated:\n%s", text)
	}
}

func TestAMalformedRegistryIsRefused(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"AAA Avolites Ltd\n", "line 1 is not a PNP ID"},
		{"AAA\tAvolites Ltd\nAAAA\tLonger\n", "line 2 is not a PNP ID"},
		{"AAA\t \n", "line 1 is not a PNP ID"},
		{"AAA\t\xff\n", "line 1 is not a PNP ID"},
		{"AAA\tAvolites Ltd\nAAA\tAnother\n", "AAA is listed twice"},
	} {
		if _, _, err := generate([]byte(tc.src)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q: %v, want %q", tc.src, err, tc.want)
		}
	}
}
