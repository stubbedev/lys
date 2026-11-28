package procstat

import "testing"

func TestUnescapeMountResolvesEveryOctalEscape(t *testing.T) {
	for in, want := range map[string]string{
		`/mnt/a\040b\134c`:      `/mnt/a b\c`,
		`/mnt/tab\011nl\012`:    "/mnt/tab\tnl\n",
		`/mnt/tag\043one`:       "/mnt/tag#one",
		`/mnt/plain`:            "/mnt/plain",
		`/mnt/odd\08`:           `/mnt/odd\08`,
		`/mnt/short\04`:         `/mnt/short\04`,
		`/mnt/trailing\`:        `/mnt/trailing\`,
		`/mnt/kept\x41`:         `/mnt/kept\x41`,
		`\040\040`:              "  ",
		`/mnt/utf\303\251t\351`: "/mnt/utfét\xe9",
	} {
		if got := UnescapeMount(in); got != want {
			t.Errorf("UnescapeMount(%q) = %q, want %q", in, got, want)
		}
	}
	if allocs := testing.AllocsPerRun(10, func() { UnescapeMount("/mnt/plain") }); allocs != 0 {
		t.Errorf("a field without escapes allocates %.0f times", allocs)
	}
}
