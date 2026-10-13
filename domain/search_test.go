package domain

import "testing"

func TestLineMatches(t *testing.T) {
	cases := []struct {
		line, query string
		sensitive   bool
		want        bool
	}{
		{"WELCOME TO TENNIS", "tennis", false, true},
		{"WELCOME TO TENNIS", "tennis", true, false},
		{"WELCOME TO TENNIS", "TENNIS", true, true},
		{"abc", "", false, false},
		{"abc", "", true, false},
		{"Tennis Modulator", "tennis mod", false, true},
		{"こんにちは", "にち", false, true},
	}
	for _, c := range cases {
		if got := LineMatches(c.line, c.query, c.sensitive); got != c.want {
			t.Errorf("LineMatches(%q,%q,%v) = %v, want %v",
				c.line, c.query, c.sensitive, got, c.want)
		}
	}
}
