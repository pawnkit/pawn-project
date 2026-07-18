// Package pathutil handles platform-independent project paths.
package pathutil

import (
	"errors"
	"fmt"
	"strings"
)

// ErrTraversal is returned when a relative path would escape its base
// directory via ".." segments.
var ErrTraversal = errors.New("pathutil: path escapes root")

// ToSlash normalizes path separators to "/", accepting both "/" and "\" in
// the input regardless of host OS.
func ToSlash(p string) string {
	return strings.ReplaceAll(p, `\`, "/")
}

// VolumeName returns the Windows drive-letter volume prefix of p (e.g.
// "C:"), or "" if p has none.
func VolumeName(p string) string {
	s := ToSlash(p)
	if len(s) >= 2 && isDriveLetter(s[0]) && s[1] == ':' {
		return s[:2]
	}

	return ""
}

func isDriveLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// IsAbs reports whether p is absolute under Unix ("/foo"), Windows drive
// ("C:\foo", "C:/foo"), or UNC (`\\server\share`) conventions.
func IsAbs(p string) bool {
	s := ToSlash(p)

	if strings.HasPrefix(s, "//") {
		return true
	}

	if vol := VolumeName(s); vol != "" {
		return strings.HasPrefix(s[len(vol):], "/")
	}

	return strings.HasPrefix(s, "/")
}

// Clean normalizes separators and simplifies a path without filesystem access.
func Clean(p string) string {
	s := ToSlash(p)
	vol := VolumeName(s)
	rest := s[len(vol):]
	abs := strings.HasPrefix(rest, "/")

	var out []string

	for seg := range strings.SplitSeq(rest, "/") {
		switch seg {
		case "", ".":
			continue
		case "..":
			switch {
			case len(out) > 0 && out[len(out)-1] != "..":
				out = out[:len(out)-1]
			case !abs:
				out = append(out, "..")
			}
		default:
			out = append(out, seg)
		}
	}

	cleaned := strings.Join(out, "/")

	switch {
	case abs:
		cleaned = "/" + cleaned
	case cleaned == "":
		cleaned = "."
	}

	return vol + cleaned
}

// Join joins path elements with "/" and cleans the result.
func Join(elems ...string) string {
	return Clean(strings.Join(elems, "/"))
}

// HasTraversal reports whether rel escapes through a ".." segment.
func HasTraversal(rel string) bool {
	c := Clean(rel)

	return c == ".." || strings.HasPrefix(c, "../")
}

// SafeJoin joins root and rel, rejecting absolute or escaping rel paths.
func SafeJoin(root, rel string) (string, error) {
	if IsAbs(rel) {
		return "", fmt.Errorf("%w: %q is absolute, want relative", ErrTraversal, rel)
	}

	if HasTraversal(rel) {
		return "", fmt.Errorf("%w: %q", ErrTraversal, rel)
	}

	return Join(root, rel), nil
}

// EqualFold compares normalized paths without case sensitivity.
func EqualFold(a, b string) bool {
	return strings.EqualFold(Clean(a), Clean(b))
}

// Dir returns the cleaned parent directory of p.
func Dir(p string) string {
	c := Clean(p)
	vol := VolumeName(c)
	rest := c[len(vol):]

	idx := strings.LastIndex(rest, "/")
	if idx < 0 {
		if vol != "" {
			return vol + "/"
		}

		return "."
	}

	if idx == 0 {
		return vol + "/"
	}

	return vol + rest[:idx]
}

// Base returns the final element of p.
func Base(p string) string {
	c := Clean(p)
	vol := VolumeName(c)
	rest := c[len(vol):]

	idx := strings.LastIndex(rest, "/")
	if idx < 0 {
		return rest
	}

	return rest[idx+1:]
}

// Ext returns the file extension of p, including the leading ".", or "" if
// p's base name has none.
func Ext(p string) string {
	base := Base(p)

	idx := strings.LastIndex(base, ".")
	if idx <= 0 {
		return ""
	}

	return base[idx:]
}
