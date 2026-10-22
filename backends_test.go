package main

import (
	"errors"
	"strings"
	"testing"
)

func fakeLookPath(available ...string) func(string) (string, error) {
	return func(name string) (string, error) {
		for _, a := range available {
			if a == name {
				return "/usr/bin/" + name, nil
			}
		}
		return "", errors.New("not found")
	}
}

func TestParseLegacyPatternFile(t *testing.T) {
	pats, err := parsePatternFile(strings.NewReader(`{"flags": "-HnrE", "pattern": "foo"}`))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(pats) != 1 {
		t.Fatalf("want 1 pattern, have %d", len(pats))
	}
	if pats[0].Flags != "-HnrE" || pats[0].Pattern != "foo" || pats[0].engine() != "grep" {
		t.Errorf("unexpected pattern: %+v", pats[0])
	}
}

func TestParseLegacyPatternFileWithEngine(t *testing.T) {
	pats, err := parsePatternFile(strings.NewReader(`{"engine": "ag", "flags": "-Hanr", "pattern": "foo"}`))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(pats) != 1 || pats[0].engine() != "ag" {
		t.Errorf("unexpected patterns: %+v", pats)
	}
}

func TestParseMultiEnginePatternFile(t *testing.T) {
	in := `{
		"rg": {"flags": "-olIN --binary", "pattern": "a"},
		"ag": {"flags": "-o", "patterns": ["b", "c"]},
		"grep": {"flags": "-oriahE", "pattern": "d"}
	}`
	pats, err := parsePatternFile(strings.NewReader(in))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	want := []string{"rg", "ag", "grep"}
	if len(pats) != len(want) {
		t.Fatalf("want %d patterns, have %d", len(want), len(pats))
	}
	for i, w := range want {
		if pats[i].engine() != w {
			t.Errorf("pattern %d: want engine %s, have %s", i, w, pats[i].engine())
		}
	}
	if pats[0].Flags != "-olIN --binary" || pats[0].Pattern != "a" {
		t.Errorf("unexpected rg pattern: %+v", pats[0])
	}
	if len(pats[1].Patterns) != 2 {
		t.Errorf("unexpected ag pattern: %+v", pats[1])
	}
}

func TestParseMalformedPatternFile(t *testing.T) {
	cases := []string{
		``,
		`[]`,
		`{"flags": "-H", "pattern": }`,
		`{"rg": "not an object"}`,
	}
	for _, c := range cases {
		if _, err := parsePatternFile(strings.NewReader(c)); err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func TestSelectBackend(t *testing.T) {
	pats := []pattern{
		{Engine: "rg", Pattern: "a"},
		{Engine: "ag", Pattern: "b"},
		{Engine: "grep", Pattern: "c"},
	}

	cases := []struct {
		available []string
		want      string
		ok        bool
	}{
		{[]string{"rg", "ag", "grep"}, "rg", true},
		{[]string{"ag", "grep"}, "ag", true},
		{[]string{"grep"}, "grep", true},
		{nil, "", false},
	}

	for _, c := range cases {
		p, ok := selectBackend(pats, fakeLookPath(c.available...))
		if ok != c.ok || p.Engine != c.want {
			t.Errorf("available %v: want (%s, %t), have (%s, %t)", c.available, c.want, c.ok, p.Engine, ok)
		}
	}
}

func TestSelectBackendSinglePattern(t *testing.T) {
	// a single pattern is always used, even if its engine can't be found
	p, ok := selectBackend([]pattern{{Pattern: "a"}}, fakeLookPath())
	if !ok || p.engine() != "grep" {
		t.Errorf("unexpected result: %+v, %t", p, ok)
	}
}
