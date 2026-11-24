package intern

import (
	"hash/maphash"
	"runtime"
	"testing"

	"github.com/stubbedev/lys/lib/testbuild"
)

func TestImportingCostsNothing(t *testing.T) { testbuild.RequireNoInit(t) }

type flat struct {
	a, b int64
	name string
}

func rawHasher() func(*flat) uint64 {
	seed := maphash.MakeSeed()
	return func(v *flat) uint64 {
		var h maphash.Hash
		h.SetSeed(seed)
		Raw(&h, v)
		return h.Sum64()
	}
}

func sameFlat(a, b *flat) bool { return *a == *b }

func TestEqualValuesShareOnePointerAndUnequalOnesDoNot(t *testing.T) {
	tab := NewTable(rawHasher(), sameFlat)
	name := "same"
	v := flat{a: 1, b: 2, name: name}
	p := tab.Intern(&v)
	if *p != v {
		t.Fatalf("interned %+v, want %+v", *p, v)
	}
	if p == &v {
		t.Fatal("the table kept the caller's value instead of its own copy")
	}
	v.a = 9
	if p.a != 1 {
		t.Fatal("writing the caller's value changed the interned one")
	}
	again := flat{a: 1, b: 2, name: name}
	if q := tab.Intern(&again); q != p {
		t.Error("an equal value got a pointer of its own")
	}
	other := flat{a: 1, b: 3, name: name}
	if q := tab.Intern(&other); q == p || *q != other {
		t.Errorf("an unequal value shared the pointer of %+v, got %+v", *p, *q)
	}
	runtime.KeepAlive(p)
}

func TestValuesWhoseHashesCollideStayApart(t *testing.T) {
	tab := NewTable(func(*flat) uint64 { return 7 }, sameFlat)
	a, b := flat{a: 1}, flat{a: 2}
	pa, pb := tab.Intern(&a), tab.Intern(&b)
	if pa == pb || *pa != a || *pb != b {
		t.Fatalf("colliding values %+v and %+v interned as %+v and %+v", a, b, *pa, *pb)
	}
	if tab.Intern(&b) != pb || tab.Intern(&a) != pa {
		t.Error("a colliding value did not find its own pointer again")
	}
	runtime.KeepAlive(pa)
	runtime.KeepAlive(pb)
}

func TestAValueNoOneHoldsIsForgotten(t *testing.T) {
	tab := NewTable(rawHasher(), sameFlat)
	kept := tab.Intern(&flat{a: 1})
	tab.Intern(&flat{a: 2})
	for range 20 {
		runtime.GC()
		if tab.Len() == 1 {
			break
		}
	}
	if n := tab.Len(); n != 1 {
		t.Fatalf("the table holds %d values after twenty collections, want only the one still referenced", n)
	}
	if tab.Intern(&flat{a: 1}) != kept {
		t.Error("the referenced value lost its pointer")
	}
	runtime.KeepAlive(kept)
}

func TestRawHashesAValueByItsMemory(t *testing.T) {
	rawHash := rawHasher()
	name := "x"
	a, b := flat{a: 1, b: 2, name: name}, flat{a: 1, b: 2, name: name}
	if rawHash(&a) != rawHash(&b) {
		t.Error("equal values hashed apart")
	}
	b.b = 3
	if rawHash(&a) == rawHash(&b) {
		t.Error("values one field apart hashed alike")
	}
}
