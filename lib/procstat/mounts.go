package procstat

import "strings"

// UnescapeMount resolves the kernel's octal escapes in a field of
// /proc/mounts or mountinfo, a backslash and three octal digits for one
// byte (\040 a space, \134 a backslash). A backslash that begins no
// such escape is kept as written; a field without one is returned as
// is, unallocated.
func UnescapeMount(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) && isOctal(s[i+1]) && isOctal(s[i+2]) && isOctal(s[i+3]) {
			b.WriteByte((s[i+1]-'0')<<6 | (s[i+2]-'0')<<3 | (s[i+3] - '0'))
			i += 3
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isOctal(c byte) bool { return c >= '0' && c <= '7' }
