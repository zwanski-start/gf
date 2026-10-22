package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestDedupeKey(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://example.com/invite?id=3454&ref=sedrt", "https://example.com/invite?id=&ref="},
		{"https://example.com/a;jsessionid=abc?x=1#frag", "https://example.com/a;jsessionid=?x=#frag"},
		{"https://example.com/no-params", "https://example.com/no-params"},
		{"https://example.com/?a=1&b=2 https://example.com/?c=3", "https://example.com/?a=&b= https://example.com/?c="},
		{"x = 1", "x = 1"},
	}

	for _, c := range cases {
		got := dedupeKey(c.in)
		if got != c.want {
			t.Errorf("dedupeKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDedupe(t *testing.T) {
	in := strings.Join([]string{
		"https://example.com/invite?id=3454&ref=sedrt",
		"https://example.com/invite?id=686&ref=folfjg",
		"https://example.com/0auth?cat_id=78978&mode=null",
		"https://example.com/0auth?cat_id=89778&mode=false",
		"https://example.com/invite?id=767&ref=shfh",
		"https://example.com/invite?id=767",
		"https://other.example.com/invite?id=1&ref=2",
	}, "\n") + "\n"

	want := strings.Join([]string{
		"https://example.com/invite?id=3454&ref=sedrt",
		"https://example.com/0auth?cat_id=78978&mode=null",
		"https://example.com/invite?id=767",
		"https://other.example.com/invite?id=1&ref=2",
	}, "\n") + "\n"

	var out bytes.Buffer
	dedupe(strings.NewReader(in), &out)

	if out.String() != want {
		t.Errorf("dedupe output:\n%s\nwant:\n%s", out.String(), want)
	}
}
