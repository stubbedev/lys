// Package fileuri is the percent-encoding and file:// URI handling
// every lys component shares: one rule for which URIs name a local
// file and how their paths decode.
package fileuri

import (
	"fmt"
	"strings"
)

// Decode resolves %XX escapes to bytes. A % that does not begin a
// valid escape is kept as written rather than dropped: it is a literal
// percent sign in a filename or a cookie, and losing it silently
// corrupts the value.
func Decode(s string) []byte {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]) {
			out = append(out, unhex(s[i+1])<<4|unhex(s[i+2]))
			i += 2
			continue
		}
		out = append(out, s[i])
	}
	return out
}

// DecodeString is Decode with each invalid UTF-8 sequence replaced by
// U+FFFD.
func DecodeString(s string) string { return strings.ToValidUTF8(string(Decode(s)), "�") }

// Encode keeps RFC 3986's unreserved bytes and those in keep, and
// escapes the rest.
func Encode(s, keep string) string {
	var b strings.Builder
	for i := range len(s) {
		c := s[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.IndexByte("-_.~", c) >= 0 || strings.IndexByte(keep, c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// Path decodes a local file URI's path: file:/p, file:///p or
// file://localhost/p, its query and fragment dropped. A remote host
// (file://host/p), another scheme, a path that is not absolute and one
// that decodes to a NUL byte are not local paths.
func Path(uri string) (string, bool) {
	rest, ok := strings.CutPrefix(uri, "file:")
	if !ok {
		return "", false
	}
	if authority, ok := strings.CutPrefix(rest, "//"); ok {
		slash := strings.IndexByte(authority, '/')
		if slash < 0 {
			return "", false
		}
		if host := authority[:slash]; host != "" && !strings.EqualFold(host, "localhost") {
			return "", false
		}
		rest = authority[slash:]
	}
	if end := strings.IndexAny(rest, "?#"); end >= 0 {
		rest = rest[:end]
	}
	if !strings.HasPrefix(rest, "/") {
		return "", false
	}
	path := DecodeString(rest)
	if strings.IndexByte(path, 0) >= 0 {
		return "", false
	}
	return path, true
}

// PathOf is the local path s names: a file URI decodes by [Path], and
// anything else is s itself, unless empty.
func PathOf(s string) (string, bool) {
	if strings.HasPrefix(s, "file:") {
		return Path(s)
	}
	return s, s != ""
}

// FromPath is the file:// URI of an absolute path, as
// g_filename_to_uri (and so gio and the thumbnail spec) writes it:
// glib's path-safe sub-delimiters stay literal.
func FromPath(path string) string { return "file://" + Encode(path, "!$&'()*+,/:=@") }

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

func unhex(c byte) byte {
	switch {
	case c >= 'a':
		return c - 'a' + 10
	case c >= 'A':
		return c - 'A' + 10
	}
	return c - '0'
}
