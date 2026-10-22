package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func setupPatternDir(t *testing.T) string {
	dir, err := os.MkdirTemp("", "gf-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	files := []string{
		"top.json",
		"web/xss.json",
		"web/inner/urls.json",
		"secrets/aws-keys.json",
		"web/notes.txt",
	}
	for _, f := range files {
		p := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(`{"pattern": "x"}`), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestWalkPatternsIncludesSubdirectories(t *testing.T) {
	dir := setupPatternDir(t)

	var names []string
	err := walkPatterns(dir, func(name, path string) bool {
		names = append(names, name)
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(names)

	want := []string{"secrets/aws-keys", "top", "web/inner/urls", "web/xss"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("got %v, want %v", names, want)
	}
}

func TestWalkPatternsMissingDir(t *testing.T) {
	err := walkPatterns(filepath.Join(os.TempDir(), "gf-does-not-exist"), func(string, string) bool {
		t.Error("callback should not be called")
		return true
	})
	if err != nil {
		t.Errorf("expected no error for missing dir, got %s", err)
	}
}

func TestFindPatternFile(t *testing.T) {
	dir := setupPatternDir(t)

	cases := map[string]string{
		"top":              "top.json",
		"web/xss":          "web/xss.json",
		"xss":              "web/xss.json",
		"urls":             "web/inner/urls.json",
		"inner/urls":       "web/inner/urls.json",
		"web/inner/urls":   "web/inner/urls.json",
		"aws-keys":         "secrets/aws-keys.json",
		"secrets/aws-keys": "secrets/aws-keys.json",
	}
	for name, want := range cases {
		got, err := findPatternFile(dir, name)
		if err != nil {
			t.Errorf("%s: unexpected error: %s", name, err)
			continue
		}
		if want := filepath.Join(dir, filepath.FromSlash(want)); got != want {
			t.Errorf("%s: got %s, want %s", name, got, want)
		}
	}

	for _, name := range []string{"", "nope", "notes", "nner/urls", "web", "ss"} {
		if got, err := findPatternFile(dir, name); err == nil {
			t.Errorf("%s: expected error, got %s", name, got)
		}
	}
}
