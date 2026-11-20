// Package shacrypt is SHA-512 crypt(3), the "$6$" scheme of Ulrich
// Drepper's "Unix crypt using SHA-256 and SHA-512", as glibc implements it
// and as /etc/shadow and AccountsService's SetPassword take it.
package shacrypt

import (
	"crypto/rand"
	"crypto/sha512"
	"crypto/subtle"
	"errors"
	"fmt"
	"hash"
	"strconv"
	"strings"
)

const (
	prefix       = "$6$"
	roundsPrefix = "rounds="
	alphabet     = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	// DefaultRounds is the rounds a setting without rounds= takes.
	DefaultRounds = 5000
	// MinRounds is the fewest rounds; a smaller rounds= is raised to it.
	MinRounds = 1000
	// MaxRounds is the most rounds; a larger rounds= is lowered to it.
	MaxRounds = 999_999_999
	// MaxSalt is the longest salt; a longer one is cut.
	MaxSalt = 16
)

// The defined refusals of a setting.
var (
	ErrPrefix = errors.New("a SHA-512 crypt setting starts with $6$")
	ErrRounds = errors.New("rounds= is a decimal count followed by $")
	ErrSalt   = errors.New("a salt is made of ./0-9A-Za-z")
)

// Setting is the salt and rounds a hash is made with.
type Setting struct {
	salt   string
	rounds int
	custom bool
}

// NewSetting is a random salt of MaxSalt characters, read from
// crypto/rand, and the default rounds.
func NewSetting() Setting {
	b := make([]byte, MaxSalt)
	_, _ = rand.Read(b)
	for i, v := range b {
		b[i] = alphabet[v&63]
	}
	return Setting{salt: string(b), rounds: DefaultRounds}
}

// ParseSetting reads the setting at the start of s: "$6$", an optional
// "rounds=N$", and the salt up to the next "$" or the end. A hash is a
// setting too, so a stored hash gives the setting it was made with.
func ParseSetting(s string) (Setting, error) {
	rest, ok := strings.CutPrefix(s, prefix)
	if !ok {
		return Setting{}, fmt.Errorf("the setting %q: %w", s, ErrPrefix)
	}
	st := Setting{rounds: DefaultRounds}
	if after, ok := strings.CutPrefix(rest, roundsPrefix); ok {
		count, tail, found := strings.Cut(after, "$")
		n, err := strconv.ParseUint(count, 10, 64)
		if !found || err != nil {
			return Setting{}, fmt.Errorf("the setting %q: %w", s, ErrRounds)
		}
		st.rounds, st.custom, rest = clampRounds(n), true, tail
	}
	salt, _, _ := strings.Cut(rest, "$")
	salt = salt[:min(len(salt), MaxSalt)]
	if strings.Trim(salt, alphabet) != "" {
		return Setting{}, fmt.Errorf("the salt %q: %w", salt, ErrSalt)
	}
	st.salt = salt
	return st, nil
}

func clampRounds(n uint64) int {
	switch {
	case n < MinRounds:
		return MinRounds
	case n > MaxRounds:
		return MaxRounds
	}
	return int(n)
}

// Salt is the setting's salt.
func (st Setting) Salt() string { return st.salt }

// Rounds is the setting's rounds.
func (st Setting) Rounds() int { return st.rounds }

func (st Setting) String() string {
	if st.custom {
		return prefix + roundsPrefix + strconv.Itoa(st.rounds) + "$" + st.salt
	}
	return prefix + st.salt
}

// Hash is password hashed with a new random setting.
func Hash(password []byte) string { return NewSetting().Hash(password) }

// Verify reports whether password is the one hash was made from.
func Verify(password []byte, hash string) (bool, error) {
	st, err := ParseSetting(hash)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare([]byte(st.Hash(password)), []byte(hash)) == 1, nil
}

// Hash is password hashed with st: the setting, "$" and the digest.
func (st Setting) Hash(password []byte) string {
	sum := st.digest(password)
	var b strings.Builder
	b.Grow(len(prefix) + len(roundsPrefix) + 10 + len(st.salt) + 1 + 86)
	b.WriteString(st.String())
	b.WriteByte('$')
	for _, g := range groups {
		encode(&b, uint(sum[g[0]])<<16|uint(sum[g[1]])<<8|uint(sum[g[2]]), 4)
	}
	encode(&b, uint(sum[63]), 2)
	return b.String()
}

var groups = [21][3]int{
	{0, 21, 42},
	{22, 43, 1},
	{44, 2, 23},
	{3, 24, 45},
	{25, 46, 4},
	{47, 5, 26},
	{6, 27, 48},
	{28, 49, 7},
	{50, 8, 29},
	{9, 30, 51},
	{31, 52, 10},
	{53, 11, 32},
	{12, 33, 54},
	{34, 55, 13},
	{56, 14, 35},
	{15, 36, 57},
	{37, 58, 16},
	{59, 17, 38},
	{18, 39, 60},
	{40, 61, 19},
	{62, 20, 41},
}

func encode(b *strings.Builder, w uint, n int) {
	for range n {
		b.WriteByte(alphabet[w&63])
		w >>= 6
	}
}

func (st Setting) digest(password []byte) []byte {
	salt := []byte(st.salt)
	h := sha512.New()
	h.Write(password)
	h.Write(salt)
	h.Write(password)
	alt := h.Sum(nil)

	h.Reset()
	h.Write(password)
	h.Write(salt)
	writeRepeated(h, alt, len(password))
	for n := len(password); n > 0; n >>= 1 {
		if n&1 != 0 {
			h.Write(alt)
		} else {
			h.Write(password)
		}
	}
	sum := h.Sum(nil)

	h.Reset()
	for range password {
		h.Write(password)
	}
	p := repeated(h.Sum(nil), len(password))

	h.Reset()
	for range 16 + int(sum[0]) {
		h.Write(salt)
	}
	s := repeated(h.Sum(nil), len(salt))

	for i := range st.rounds {
		h.Reset()
		if i&1 != 0 {
			h.Write(p)
		} else {
			h.Write(sum)
		}
		if i%3 != 0 {
			h.Write(s)
		}
		if i%7 != 0 {
			h.Write(p)
		}
		if i&1 != 0 {
			h.Write(sum)
		} else {
			h.Write(p)
		}
		sum = h.Sum(sum[:0])
	}
	return sum
}

func writeRepeated(h hash.Hash, block []byte, n int) {
	for ; n > len(block); n -= len(block) {
		h.Write(block)
	}
	h.Write(block[:n])
}

func repeated(block []byte, n int) []byte {
	out := make([]byte, 0, n)
	for len(out) < n {
		out = append(out, block[:min(len(block), n-len(out))]...)
	}
	return out
}
