package main

import (
	"reflect"
	"testing"
)

func TestContextArgs(t *testing.T) {
	cases := []struct {
		name                   string
		pat                    pattern
		after, before, context int
		want                   []string
	}{
		{"none", pattern{}, 0, 0, 0, []string{}},
		{"cli after", pattern{}, 2, 0, 0, []string{"-A", "2"}},
		{"cli before and after", pattern{}, 1, 3, 0, []string{"-A", "1", "-B", "3"}},
		{"cli context", pattern{}, 0, 0, 4, []string{"-C", "4"}},
		{"pattern file", pattern{After: 2, Before: 1}, 0, 0, 0, []string{"-A", "2", "-B", "1"}},
		{"cli overrides pattern file", pattern{After: 2, Context: 5}, 7, 0, 0, []string{"-A", "7", "-C", "5"}},
	}

	for _, c := range cases {
		got := contextArgs(c.pat, c.after, c.before, c.context)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: want %v, got %v", c.name, c.want, got)
		}
	}
}
