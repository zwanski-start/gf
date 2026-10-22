package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestFlagListUnmarshal(t *testing.T) {
	cases := []struct {
		in   string
		want flagList
	}{
		{`{"flags": "-HnrE"}`, flagList{"-HnrE"}},
		{`{"flags": "-HnroE --include=*.php"}`, flagList{"-HnroE", "--include=*.php"}},
		{`{"flags": ["-HnroE", "--include=*.php", "--exclude-dir=vendor"]}`, flagList{"-HnroE", "--include=*.php", "--exclude-dir=vendor"}},
		{`{"flags": ""}`, flagList{}},
		{`{}`, nil},
	}

	for _, c := range cases {
		p := pattern{}
		if err := json.Unmarshal([]byte(c.in), &p); err != nil {
			t.Fatalf("unmarshal %s: %s", c.in, err)
		}
		if len(p.Flags) != len(c.want) || (len(c.want) > 0 && !reflect.DeepEqual(p.Flags, c.want)) {
			t.Errorf("unmarshal %s: want %q, have %q", c.in, c.want, p.Flags)
		}
	}
}

func TestFlagListUnmarshalInvalid(t *testing.T) {
	p := pattern{}
	if err := json.Unmarshal([]byte(`{"flags": 42}`), &p); err == nil {
		t.Error("expected error for non-string flags")
	}
}

func TestFlagListMarshal(t *testing.T) {
	b, err := json.Marshal(pattern{Flags: flagList{"-HnrE", "--include=*.php"}, Pattern: "x"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"flags":"-HnrE --include=*.php","pattern":"x"}`
	if string(b) != want {
		t.Errorf("want %s, have %s", want, b)
	}
}
