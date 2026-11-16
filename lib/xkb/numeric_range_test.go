package xkb

import (
	"strings"
	"testing"
)

func loggingContext(t *testing.T) (*Context, *[]string) {
	t.Helper()
	c := hostileContext(t)
	var logged []string
	c.SetLogLevel(LogWarning)
	c.SetLogFunc(func(_ LogLevel, message string) { logged = append(logged, message) })
	return c, &logged
}

func TestANumericKeysymLiteralIsBoundedByTheKeysymRange(t *testing.T) {
	for src, want := range map[string]Keysym{"0": Key0, "9": Key9, "10": 10, "0x1fffffff": keysymMax} {
		c, logged := loggingContext(t)
		p, err := newParser(c, nil, "t", src)
		if err != nil {
			t.Fatal(err)
		}
		got, err := p.parseKeysym()
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if got != want || len(*logged) != 0 {
			t.Errorf("%s gave %#x logging %q, want %#x silently", src, got, *logged, want)
		}
	}
	for _, src := range []string{"0x20000000", "0x100000000", "4294967296", "0x7fffffffffffffff"} {
		c, logged := loggingContext(t)
		p, err := newParser(c, nil, "t", src)
		if err != nil {
			t.Fatal(err)
		}
		got, err := p.parseKeysym()
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if got != KeyNoSymbol || len(*logged) != 1 || !strings.Contains((*logged)[0], "unrecognized keysym") {
			t.Errorf("%s gave %#x logging %q, want NoSymbol and one unrecognized keysym warning", src, got, *logged)
		}
	}
}

func TestAUnicodeEscapeIsBoundedByTheLastCodePoint(t *testing.T) {
	c, logged := loggingContext(t)
	tok, err := newLexer(c, "t", `"\u{10FFFF}"`).next()
	if err != nil {
		t.Fatal(err)
	}
	if tok.text != "\U0010FFFF" || len(*logged) != 0 {
		t.Fatalf("lexed %q logging %q, want U+10FFFF silently", tok.text, *logged)
	}
	for _, src := range []string{`"\u{110000}"`, `"\u{7FFFFFFF}"`, `"\u{80000000}"`, `"\u{FFFFFFFF}"`, `"\u{D800}"`} {
		c, logged := loggingContext(t)
		tok, err := newLexer(c, "t", src).next()
		if err != nil {
			t.Fatal(err)
		}
		if tok.text != "" || len(*logged) != 1 || !strings.Contains((*logged)[0], "invalid Unicode escape") {
			t.Errorf("%s lexed %q logging %q, want nothing and one invalid escape warning", src, tok.text, *logged)
		}
	}
}

func TestAUnicodeKeysymNameIsBoundedByTheLastCodePoint(t *testing.T) {
	if got, want := KeysymFromName("U10FFFF", 0), Keysym(keysymUnicode+0x10ffff); got != want {
		t.Fatalf("U10FFFF is %#x, want %#x", got, want)
	}
	for _, name := range []string{"U110000", "U7FFFFFFF", "U80000000", "UFFFFFFFF"} {
		if got := KeysymFromName(name, 0); got != KeyNoSymbol {
			t.Errorf("%s is %#x, want NoSymbol", name, got)
		}
	}
}
