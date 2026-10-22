package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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

	patName := flag.Arg(0)
	files := flag.Arg(1)
	if files == "" {
		files = "."
	}

	patDir, err := getPatternDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "unable to open user's pattern directory")
		return
	}

	filename, err := findPatternFile(patDir, patName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no such pattern")
		return
	}

	f, err := os.Open(filename)
	if err != nil {
		fmt.Fprintln(os.Stderr, "no such pattern")
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

	err = walkPatterns(patDir, func(name, path string) bool {
		out = append(out, name)
		return true
	})

	return out, err
}

// walkPatterns calls fn for every pattern file in patDir, including those
// in sub-directories. The name passed to fn is the pattern's path relative
// to patDir, without the .json extension (e.g. "web/xss"). Walking stops
// when fn returns false.
func walkPatterns(patDir string, fn func(name, path string) bool) error {
	done := errors.New("done")

	err := filepath.Walk(patDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if path == patDir {
				return err
			}
			// skip anything we can't read rather than failing outright
			return nil
		}

		if info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		rel, err := filepath.Rel(patDir, path)
		if err != nil {
			return nil
		}
		name := filepath.ToSlash(strings.TrimSuffix(rel, ".json"))

		if !fn(name, path) {
			return done
		}
		return nil
	})

	if err == done {
		return nil
	}
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// findPatternFile resolves a pattern name to a file. The name can be a path
// relative to the pattern directory (e.g. "web/xss"); if no such file exists
// the sub-directories are searched for a pattern with a matching file name.
func findPatternFile(patDir, name string) (string, error) {
	if name == "" {
		return "", errors.New("no such pattern")
	}

	filename := filepath.Join(patDir, filepath.FromSlash(name)+".json")
	if info, err := os.Stat(filename); err == nil && !info.IsDir() {
		return filename, nil
	}

	found := ""
	err := walkPatterns(patDir, func(n, path string) bool {
		if strings.HasSuffix("/"+n, "/"+name) {
			found = path
			return false
		}
		return true
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", errors.New("no such pattern")
	}

	return found, nil
}

func stdinIsPipe() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}
