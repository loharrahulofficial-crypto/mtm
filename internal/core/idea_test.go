package core

import "testing"

func TestIdeaValidate(t *testing.T) {
	cases := []struct {
		name    string
		i       Idea
		wantErr bool
	}{
		{"ok", Idea{Title: "t", Status: "seed"}, false},
		{"case-insensitive status", Idea{Title: "t", Status: "Exploring"}, false},
		{"empty title", Idea{Title: "  ", Status: "seed"}, true},
		{"missing status", Idea{Title: "t"}, true},
		{"bad status", Idea{Title: "t", Status: "wild"}, true},
		{"rating zero is unrated", Idea{Title: "t", Status: "seed", Rating: 0}, false},
		{"rating five", Idea{Title: "t", Status: "seed", Rating: 5}, false},
		{"rating out of range", Idea{Title: "t", Status: "seed", Rating: 6}, true},
		{"negative rating", Idea{Title: "t", Status: "seed", Rating: -1}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.i.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestIdeaStatusCyclesThroughActive(t *testing.T) {
	want := map[string]string{
		"seed":      "exploring",
		"exploring": "parked",
		"parked":    "seed",
		"promoted":  "seed", // terminal statuses cycle back into the lifecycle
		"dropped":   "seed",
	}
	for from, to := range want {
		if got := NextIdeaStatus(from); got != to {
			t.Errorf("NextIdeaStatus(%q) = %q, want %q", from, got, to)
		}
	}
}

func TestParseRating(t *testing.T) {
	for _, ok := range []struct {
		in   string
		want int
	}{{"1", 1}, {"5", 5}, {" 3 ", 3}} {
		got, err := ParseRating(ok.in)
		if err != nil || got != ok.want {
			t.Errorf("ParseRating(%q) = %d, %v; want %d", ok.in, got, err, ok.want)
		}
	}
	for _, bad := range []string{"0", "6", "x", "", "-1"} {
		if _, err := ParseRating(bad); err == nil {
			t.Errorf("ParseRating(%q) accepted; want error", bad)
		}
	}
}
