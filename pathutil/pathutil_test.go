package pathutil

import "testing"

func TestIsAbs(t *testing.T) {
	cases := map[string]bool{
		"/a/b":           true,
		"a/b":            false,
		`C:\a\b`:         true,
		`C:/a/b`:         true,
		`c:\a`:           true,
		`\\server\share`: true,
		`.\a\b`:          false,
		"":               false,
		"relative/path":  false,
	}

	for input, want := range cases {
		if got := IsAbs(input); got != want {
			t.Errorf("IsAbs(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestClean(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/a/b/../c", "/a/c"},
		{"a/b/../c", "a/c"},
		{`a\b\..\c`, "a/c"},
		{"./a/./b", "a/b"},
		{"../a", "../a"},
		{"a/../../b", "../b"},
		{"/a/../../b", "/b"},
		{`C:\a\b\..\c`, "C:/a/c"},
		{`C:\..\a`, "C:/a"},
		{"", "."},
		{"a//b", "a/b"},
		{"/", "/"},
	}

	for _, c := range cases {
		if got := Clean(c.in); got != c.want {
			t.Errorf("Clean(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestJoin(t *testing.T) {
	if got := Join("/root", "a", "b"); got != "/root/a/b" {
		t.Errorf("Join = %q", got)
	}

	if got := Join(`C:\root`, `a\b`); got != "C:/root/a/b" {
		t.Errorf("Join = %q", got)
	}
}

func TestHasTraversal(t *testing.T) {
	cases := map[string]bool{
		"../a":      true,
		"a/../../b": true,
		"a/b":       false,
		"a/../b":    false,
		"..":        true,
		`..\a`:      true,
		"./a":       false,
	}

	for input, want := range cases {
		if got := HasTraversal(input); got != want {
			t.Errorf("HasTraversal(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestSafeJoin(t *testing.T) {
	if _, err := SafeJoin("/root", "../etc/passwd"); err == nil {
		t.Fatal("expected traversal error")
	}

	if _, err := SafeJoin("/root", "/etc/passwd"); err == nil {
		t.Fatal("expected error for absolute rel")
	}

	got, err := SafeJoin("/root", "includes/foo.inc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := "/root/includes/foo.inc"; got != want {
		t.Errorf("SafeJoin = %q, want %q", got, want)
	}
}

func TestEqualFold(t *testing.T) {
	if !EqualFold(`C:\Foo\Bar.inc`, "c:/foo/bar.inc") {
		t.Error("expected case-insensitive equality")
	}

	if EqualFold("/a/b", "/a/c") {
		t.Error("expected inequality")
	}
}

func TestDirBaseExt(t *testing.T) {
	if got := Dir("/a/b/c.pwn"); got != "/a/b" {
		t.Errorf("Dir = %q", got)
	}

	if got := Base("/a/b/c.pwn"); got != "c.pwn" {
		t.Errorf("Base = %q", got)
	}

	if got := Ext("/a/b/c.pwn"); got != ".pwn" {
		t.Errorf("Ext = %q", got)
	}

	if got := Ext("/a/b/.gitignore"); got != "" {
		t.Errorf("Ext(.gitignore) = %q, want empty", got)
	}

	if got := Dir(`C:\a\b.inc`); got != "C:/a" {
		t.Errorf("Dir(windows) = %q", got)
	}
}
