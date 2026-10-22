// Command grepbugs converts the GrepBugs rules file
// (https://github.com/foospidy/GrepBugsRules/blob/master/rules.json)
// into gf pattern files, one per language.
//
//	curl -s https://raw.githubusercontent.com/foospidy/GrepBugsRules/master/rules.json |
//	    go run contrib/grepbugs/main.go -o ~/.gf
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type rule struct {
	ID          string `json:"id"`
	Tags        string `json:"tags"`
	Language    string `json:"language"`
	Extension   string `json:"extension"`
	Regex       string `json:"regex"`
	Description string `json:"description"`
}

type pattern struct {
	Flags    string   `json:"flags,omitempty"`
	Pattern  string   `json:"pattern,omitempty"`
	Patterns []string `json:"patterns,omitempty"`
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// slug turns a language name like "Objective C" or "C++" into
// something suitable for a pattern name, e.g. "objective-c", "cpp"
func slug(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "++", "pp")
	s = strings.ReplaceAll(s, "#", "sharp")
	s = strings.ReplaceAll(s, ".", "")
	return strings.Trim(nonAlnum.ReplaceAllString(s, "-"), "-")
}

func main() {
	var outDir string
	flag.StringVar(&outDir, "o", ".", "directory to write the pattern files to")

	var prefix string
	flag.StringVar(&prefix, "prefix", "grepbugs-", "prefix for the generated pattern names")

	var flags string
	flag.StringVar(&flags, "flags", "-HnrP", "grep flags for the generated patterns (the rules use PCRE syntax)")

	var force bool
	flag.BoolVar(&force, "f", false, "overwrite existing pattern files")

	flag.Parse()

	var in io.Reader = os.Stdin
	if flag.Arg(0) != "" && flag.Arg(0) != "-" {
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open rules file: %s\n", err)
			os.Exit(1)
		}
		defer f.Close()
		in = f
	}

	var rules []rule
	err := json.NewDecoder(in).Decode(&rules)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse rules: %s\n", err)
		os.Exit(1)
	}

	byLang := make(map[string][]string)
	seen := make(map[string]bool)
	for _, r := range rules {
		name := slug(r.Language)
		if name == "" || r.Regex == "" {
			continue
		}

		key := name + "\x00" + r.Regex
		if seen[key] {
			continue
		}
		seen[key] = true

		byLang[name] = append(byLang[name], r.Regex)
	}

	names := make([]string, 0, len(byLang))
	for name := range byLang {
		names = append(names, name)
	}
	sort.Strings(names)

	openFlags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if force {
		openFlags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}

	failed := false
	for _, name := range names {
		path := filepath.Join(outDir, prefix+name+".json")

		f, err := os.OpenFile(path, openFlags, 0666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create pattern file: %s\n", err)
			failed = true
			continue
		}

		enc := json.NewEncoder(f)
		enc.SetIndent("", "    ")
		enc.SetEscapeHTML(false)

		err = enc.Encode(pattern{Flags: flags, Patterns: byLang[name]})
		f.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to write pattern file: %s\n", err)
			failed = true
			continue
		}

		fmt.Printf("%s (%d rules)\n", path, len(byLang[name]))
	}

	if failed {
		os.Exit(1)
	}
}
