package text

import "testing"

func TestPlural(t *testing.T) {
	for _, tc := range []struct {
		n        int
		singular string
		plural   string
		want     string
	}{
		{1, "day", "days", "day"},
		{0, "day", "days", "days"},
		{2, "day", "days", "days"},
		{-1, "day", "days", "days"},
		{1, "item", "items", "item"},
	} {
		if got := Plural(tc.n, tc.singular, tc.plural); got != tc.want {
			t.Errorf("Plural(%d, %q, %q) = %q, want %q", tc.n, tc.singular, tc.plural, got, tc.want)
		}
	}
}

func TestCount(t *testing.T) {
	for _, tc := range []struct {
		n    int
		want string
	}{
		{0, "0 days"},
		{1, "1 day"},
		{3, "3 days"},
	} {
		if got := Count(tc.n, "day", "days"); got != tc.want {
			t.Errorf("Count(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}
