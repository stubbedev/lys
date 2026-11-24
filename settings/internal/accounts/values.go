package accounts

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/stubbedev/lys/lib/shacrypt"
	"golang.org/x/text/unicode/norm"
)

// MaxLogin is the longest login name useradd takes everywhere.
const MaxLogin = 32

// The defined refusals of a login name.
var (
	ErrLoginEmpty = errors.New("a login name is not empty")
	ErrLoginLong  = fmt.Errorf("a login name has at most %d characters", MaxLogin)
	ErrLoginStart = errors.New("a login name starts with a lower-case letter or _")
	ErrLoginChars = errors.New("a login name holds only lower-case letters, digits, - and _")
)

// Login is a login name every useradd takes: a lower-case letter or _,
// then lower-case letters, digits, - and _, at most MaxLogin of them.
type Login string

// ParseLogin is s as a Login.
func ParseLogin(s string) (Login, error) {
	switch {
	case s == "":
		return "", ErrLoginEmpty
	case len(s) > MaxLogin:
		return "", fmt.Errorf("%q: %w", s, ErrLoginLong)
	case !loginStart(rune(s[0])):
		return "", fmt.Errorf("%q: %w", s, ErrLoginStart)
	case strings.IndexFunc(s, func(r rune) bool { return !loginRune(r) }) >= 0:
		return "", fmt.Errorf("%q: %w", s, ErrLoginChars)
	}
	return Login(s), nil
}

// UnmarshalText reads a Login, refusing text ParseLogin refuses.
func (l *Login) UnmarshalText(text []byte) error {
	v, err := ParseLogin(string(text))
	*l = v
	return err
}

// MaxName is the longest name of an existing account taken.
const MaxName = 256

// ErrName is text that names no account the shadow tools could be given.
var ErrName = fmt.Errorf("an account's name is at most %d bytes, without :, /, a leading - or blank or control characters", MaxName)

// Name is an existing account's login name as the passwd database holds
// it, which may be one useradd would no longer make, but never one that
// reads as a flag, a path or another field.
type Name string

// ParseName is s as a Name.
func ParseName(s string) (Name, error) {
	if s == "" || len(s) > MaxName || s[0] == '-' || strings.ContainsAny(s, ":/") || strings.IndexFunc(s, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 || !utf8.ValidString(s) {
		return "", fmt.Errorf("%q: %w", s, ErrName)
	}
	return Name(s), nil
}

// UnmarshalText reads a Name, refusing text ParseName refuses.
func (n *Name) UnmarshalText(text []byte) error {
	v, err := ParseName(string(text))
	*n = v
	return err
}

func loginStart(r rune) bool { return r == '_' || r >= 'a' && r <= 'z' }

func loginRune(r rune) bool { return loginStart(r) || r == '-' || r >= '0' && r <= '9' }

// DeriveLogin is a login name made from a full name: its letters folded
// to lower case without accents, and its digits, - and _ once a letter
// or _ began it; "" where it has none.
func DeriveLogin(full string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(full)) {
		if loginRune(r) && (b.Len() > 0 || loginStart(r)) {
			b.WriteRune(r)
		}
		if b.Len() == MaxLogin {
			break
		}
	}
	return b.String()
}

// MaxRealName is the longest full name taken, in bytes.
const MaxRealName = 256

// ErrRealName is a full name the passwd database cannot hold.
var ErrRealName = fmt.Errorf("a full name is at most %d bytes of text without : , or control characters", MaxRealName)

// RealName is a full name the GECOS field holds: trimmed, valid UTF-8,
// without the field separators : and , or control characters.
type RealName string

// ParseRealName is s, trimmed, as a RealName.
func ParseRealName(s string) (RealName, error) {
	s = strings.TrimSpace(s)
	if len(s) > MaxRealName || !utf8.ValidString(s) || strings.ContainsAny(s, ":,") || strings.IndexFunc(s, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("%q: %w", s, ErrRealName)
	}
	return RealName(s), nil
}

// UnmarshalText reads a RealName, refusing text ParseRealName refuses or
// would trim.
func (n *RealName) UnmarshalText(text []byte) error {
	v, err := ParseRealName(string(text))
	if err == nil && string(v) != string(text) {
		err = fmt.Errorf("%q: %w", text, ErrRealName)
	}
	*n = v
	return err
}

// Hash is a whole SHA-512 crypt hash, as chpasswd -e takes it.
type Hash string

// HashPassword is password hashed with a new random setting.
func HashPassword(password []byte) Hash { return Hash(shacrypt.Hash(password)) }

// String keeps the hash out of logs and errors.
func (Hash) String() string { return "<hash>" }

// MarshalText writes the hash itself.
func (h Hash) MarshalText() ([]byte, error) { return []byte(h), nil }

// UnmarshalText reads a Hash, refusing anything shacrypt.CheckHash does.
func (h *Hash) UnmarshalText(text []byte) error {
	if err := shacrypt.CheckHash(string(text)); err != nil {
		return fmt.Errorf("accounts: the password hash: %w", err)
	}
	*h = Hash(text)
	return nil
}

// The bounds of a picture.
const (
	MaxPictureBytes = 4 << 20
	MaxPictureSide  = 1024
)

// ErrPicture is a picture that is not a PNG within the bounds.
var ErrPicture = fmt.Errorf("a picture is a PNG of at most %d bytes and %dpx a side", MaxPictureBytes, MaxPictureSide)

// Picture is a PNG within the bounds, encoded afresh from its pixels.
type Picture []byte

// ParsePicture decodes data as a PNG within the bounds and encodes it
// afresh, so nothing but its pixels is kept.
func ParsePicture(data []byte) (Picture, error) {
	if len(data) > MaxPictureBytes {
		return nil, fmt.Errorf("%d bytes: %w", len(data), ErrPicture)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPicture, err)
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > MaxPictureSide || cfg.Height > MaxPictureSide {
		return nil, fmt.Errorf("%dx%d: %w", cfg.Width, cfg.Height, ErrPicture)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPicture, err)
	}
	return EncodePicture(img)
}

// EncodePicture is img as a Picture.
func EncodePicture(img image.Image) (Picture, error) {
	b := img.Bounds()
	if b.Dx() < 1 || b.Dy() < 1 || b.Dx() > MaxPictureSide || b.Dy() > MaxPictureSide {
		return nil, fmt.Errorf("%dx%d: %w", b.Dx(), b.Dy(), ErrPicture)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode the picture: %w", err)
	}
	return buf.Bytes(), nil
}

// MarshalJSON writes the picture's bytes in base64.
func (p Picture) MarshalJSON() ([]byte, error) { return json.Marshal([]byte(p)) }

// UnmarshalJSON reads a picture, refusing what ParsePicture refuses.
func (p *Picture) UnmarshalJSON(data []byte) error {
	var raw []byte
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("accounts: the picture: %w", err)
	}
	v, err := ParsePicture(raw)
	*p = v
	return err
}
