package dependency

import "testing"

func TestSelectTag(t *testing.T) {
	tags := []string{"not-a-version", "v1.2.0", "1.2.4", "v1.3.0", "v2.0.0"}
	for name, test := range map[string]struct {
		constraint string
		want       string
	}{
		"exact":    {constraint: "v1.2.0", want: "v1.2.0"},
		"caret":    {constraint: "^1.2.0", want: "v1.3.0"},
		"tilde":    {constraint: "~1.2", want: "1.2.4"},
		"wildcard": {constraint: "1.2.x", want: "1.2.4"},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := SelectTag(tags, test.constraint)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("tag = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSelectTagRejectsInvalidOrUnmatchedConstraint(t *testing.T) {
	for _, constraint := range []string{"", "not valid!", "^9.0.0"} {
		if _, err := SelectTag([]string{"v1.0.0"}, constraint); err == nil {
			t.Fatalf("SelectTag(%q) succeeded", constraint)
		}
	}
}

func TestSelectTagIsIndependentOfInputOrder(t *testing.T) {
	first, err := SelectTag([]string{"v1.2.0", "v1.3.0"}, "^1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := SelectTag([]string{"v1.3.0", "v1.2.0"}, "^1")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("tags differ: %q and %q", first, second)
	}
}
