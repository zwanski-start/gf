package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type pattern struct {
	Flags    string   `json:"flags,omitempty"`
	Pattern  string   `json:"pattern,omitempty"`
	Patterns []string `json:"patterns,omitempty"`
	Engine   string   `json:"engine,omitempty"`
}

func main() {
	var saveMode bool
	flag.BoolVar(&saveMode, "save", false, "save a pattern (e.g: gf -save pat-name -Hnri 'search-pattern')")

	var listMode bool
	flag.BoolVar(&listMode, "list", false, "list available patterns")

	var dumpMode bool
	flag.BoolVar(&dumpMode, "dump", false, "prints the grep command rather than executing it")

	flag.Parse()

	if listMode {
		pats, err := getPatterns()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return
		}

		if len(pats) == 0 {
			// only print the hint when a person is looking; the shell
			// completion scripts call gf -list and expect empty output
			if stdoutIsTerminal() {
				fmt.Fprintln(os.Stderr, noPatternsHint())
			}
			return
		}

		fmt.Println(strings.Join(pats, "\n"))
		return
	}

	if saveMode {
		name := flag.Arg(0)
		flags := flag.Arg(1)
		pattern := flag.Arg(2)

		err := savePattern(name, flags, pattern)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
		}
		return
	}

	patName := flag.Arg(0)
	files := flag.Arg(1)
	if files == "" {
		files = "."
	}

	if patName == "" {
		fmt.Fprintln(os.Stderr, "usage: gf [-list|-dump|-save] <pattern-name> [files]")
		return
	}

	filename, err := findPatternFile(patName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}

	f, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to open pattern file '%s': %s\n", filename, err)
		return
	}
	defer f.Close()

	pat := pattern{}
	dec := json.NewDecoder(f)
	err = dec.Decode(&pat)

	if err != nil {
		fmt.Fprintf(os.Stderr, "pattern file '%s' is malformed: %s\n", filename, err)
		return
	}

	if pat.Pattern == "" {
		// check for multiple patterns
		if len(pat.Patterns) == 0 {
			fmt.Fprintf(os.Stderr, "pattern file '%s' contains no pattern(s)\n", filename)
			return
		}

		pat.Pattern = "(" + strings.Join(pat.Patterns, "|") + ")"
	}

	if dumpMode {
		fmt.Printf("grep %v %q %v\n", pat.Flags, pat.Pattern, files)

	} else {
		var cmd *exec.Cmd
		operator := "grep"
		if pat.Engine != "" {
			operator = pat.Engine
		}

		if stdinIsPipe() {
			cmd = exec.Command(operator, pat.Flags, pat.Pattern)
		} else {
			cmd = exec.Command(operator, pat.Flags, pat.Pattern, files)
		}
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	}

}

// getPatternDirs returns every directory that may hold pattern files,
// in order of precedence. The directories are not required to exist.
func getPatternDirs() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return []string{
		filepath.Join(home, ".config/gf"),
		filepath.Join(home, ".gf"),
	}, nil
}

// getPatternDir returns the directory new patterns are saved to:
// ~/.config/gf if it exists, ~/.gf otherwise
func getPatternDir() (string, error) {
	dirs, err := getPatternDirs()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(dirs[0]); !os.IsNotExist(err) {
		// .config/gf exists
		return dirs[0], nil
	}
	return dirs[1], nil
}

// findPatternFile looks for the named pattern in each of the
// pattern directories and returns the path to the first match
func findPatternFile(name string) (string, error) {
	dirs, err := getPatternDirs()
	if err != nil {
		return "", fmt.Errorf("failed to determine pattern directory: %s", err)
	}

	for _, dir := range dirs {
		path := filepath.Join(dir, name+".json")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	pats, _ := getPatterns()
	if len(pats) == 0 {
		return "", fmt.Errorf("no such pattern '%s'\n%s", name, noPatternsHint())
	}
	return "", fmt.Errorf("no such pattern '%s' (searched %s); run gf -list to see the available patterns", name, strings.Join(dirs, " and "))
}

// noPatternsHint explains where gf looks for pattern files
func noPatternsHint() string {
	dirs, err := getPatternDirs()
	if err != nil {
		return "no pattern files found"
	}
	return fmt.Sprintf(
		"no pattern files found in %s\n"+
			"copy some pattern files there to get started, e.g. the examples from the gf repository:\n"+
			"  git clone https://github.com/tomnomnom/gf && mkdir -p %s && cp gf/examples/*.json %s/",
		strings.Join(dirs, " or "), dirs[1], dirs[1],
	)
}

func savePattern(name, flags, pat string) error {
	if name == "" {
		return errors.New("name cannot be empty")
	}

	if pat == "" {
		return errors.New("pattern cannot be empty")
	}

	p := &pattern{
		Flags:   flags,
		Pattern: pat,
	}

	patDir, err := getPatternDir()
	if err != nil {
		return fmt.Errorf("failed to determine pattern directory: %s", err)
	}

	path := filepath.Join(patDir, name+".json")
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return fmt.Errorf("failed to create pattern file: %s", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")

	err = enc.Encode(p)
	if err != nil {
		return fmt.Errorf("failed to write pattern file: %s", err)
	}

	return nil
}

func getPatterns() ([]string, error) {
	out := []string{}

	dirs, err := getPatternDirs()
	if err != nil {
		return out, fmt.Errorf("failed to determine pattern directory: %s", err)
	}

	seen := make(map[string]bool)
	for _, dir := range dirs {
		files, err := filepath.Glob(filepath.Join(dir, "*.json"))
		if err != nil {
			return out, err
		}

		for _, f := range files {
			name := strings.TrimSuffix(filepath.Base(f), ".json")
			if seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name)
		}
	}

	sort.Strings(out)
	return out, nil
}

func stdinIsPipe() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}

func stdoutIsTerminal() bool {
	stat, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}
