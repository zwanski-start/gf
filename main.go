package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
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

	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: gf [options] <pattern-name> [<pattern-name>...] [<files>]")
		return
	}

	patDir, err := getPatternDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "unable to open user's pattern directory")
		return
	}

	// The first argument is always a pattern name. Any following arguments
	// that are also pattern names are treated as extra patterns, e.g:
	//   cat urls | gf xss sqli ssrf
	// The first argument that isn't a pattern name, and everything after it,
	// are the files to search. When stdin isn't a pipe, an argument that
	// exists on disk is always a file so that `gf name file` keeps working
	// even if 'file' happens to also be the name of a pattern.
	pipe := stdinIsPipe()
	patNames := []string{flag.Arg(0)}
	args := flag.Args()[1:]
	for len(args) > 0 && patternExists(patDir, args[0]) && (pipe || !pathExists(args[0])) {
		patNames = append(patNames, args[0])
		args = args[1:]
	}

	files := "."
	if len(args) > 0 {
		files = args[0]
	}

	pats := make([]pattern, 0, len(patNames))
	for _, patName := range patNames {
		pat, err := loadPattern(patDir, patName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return
		}
		pats = append(pats, pat)
	}

	if dumpMode {
		for _, pat := range pats {
			fmt.Printf("%s %v %q %v\n", pat.operator(), pat.Flags, pat.Pattern, files)
		}
		return
	}

	// stdin can only be read once, so if there's more than one pattern
	// to run against it we need to keep a copy of it for each command
	var input []byte
	if pipe && len(pats) > 1 {
		input, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read stdin: %s\n", err)
			return
		}
	}

	for _, pat := range pats {
		var cmd *exec.Cmd
		if pipe {
			cmd = exec.Command(pat.operator(), pat.Flags, pat.Pattern)
		} else {
			cmd = exec.Command(pat.operator(), pat.Flags, pat.Pattern, files)
		}

		if input != nil {
			cmd.Stdin = bytes.NewReader(input)
		} else {
			cmd.Stdin = os.Stdin
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	}

}

// operator returns the command used to run the pattern
func (p pattern) operator() string {
	if p.Engine != "" {
		return p.Engine
	}
	return "grep"
}

// patternExists reports whether a pattern file with the given name exists
func patternExists(patDir, name string) bool {
	if name == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(patDir, name+".json"))
	return err == nil && !info.IsDir()
}

// pathExists reports whether a file or directory exists at path
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// loadPattern reads and validates the named pattern file
func loadPattern(patDir, name string) (pattern, error) {
	pat := pattern{}

	filename := filepath.Join(patDir, name+".json")
	f, err := os.Open(filename)
	if err != nil {
		return pat, fmt.Errorf("no such pattern: %s", name)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	err = dec.Decode(&pat)
	if err != nil {
		return pat, fmt.Errorf("pattern file '%s' is malformed: %s", filename, err)
	}

	if pat.Pattern == "" {
		// check for multiple patterns
		if len(pat.Patterns) == 0 {
			return pat, fmt.Errorf("pattern file '%s' contains no pattern(s)", filename)
		}

		pat.Pattern = "(" + strings.Join(pat.Patterns, "|") + ")"
	}

	return pat, nil
}

func getPatternDir() (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	path := filepath.Join(usr.HomeDir, ".config/gf")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		// .config/gf exists
		return path, nil
	}
	return filepath.Join(usr.HomeDir, ".gf"), nil
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

	patDir, err := getPatternDir()
	if err != nil {
		return out, fmt.Errorf("failed to determine pattern directory: %s", err)
	}
	_ = patDir

	files, err := filepath.Glob(patDir + "/*.json")
	if err != nil {
		return out, err
	}

	for _, f := range files {
		f = f[len(patDir)+1 : len(f)-5]
		out = append(out, f)
	}

	return out, nil
}

func stdinIsPipe() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}
