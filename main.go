package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
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

	var dedupeMode bool
	flag.BoolVar(&dedupeMode, "dedupe", false, "only print the first of any lines that differ only by parameter values (e.g. ?id=1 and ?id=2)")

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

	filename := filepath.Join(patDir, patName+".json")
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
		cmd.Stderr = os.Stderr

		if !dedupeMode {
			cmd.Stdout = os.Stdout
			cmd.Run()
			return
		}

		out, err := cmd.StdoutPipe()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to read %s output: %s\n", operator, err)
			return
		}
		err = cmd.Start()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to run %s: %s\n", operator, err)
			return
		}
		dedupe(out, os.Stdout)
		cmd.Wait()
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

// paramValueRe matches the value part of key=value pairs in query strings
// and similar parameter lists (e.g. ?id=123&ref=abc or ;jsessionid=xyz)
var paramValueRe = regexp.MustCompile(`([?&;][^=&;?#\s]*)=[^&;?#\s"'<>]*`)

// dedupeKey returns a normalised version of a line where parameter
// values have been removed, so that lines differing only by those
// values produce the same key
func dedupeKey(line string) string {
	return paramValueRe.ReplaceAllString(line, "$1=")
}

// dedupe copies lines from r to w, skipping any line whose dedupeKey
// has already been seen
func dedupe(r io.Reader, w io.Writer) {
	seen := make(map[string]bool)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	for sc.Scan() {
		line := sc.Text()
		key := dedupeKey(line)
		if seen[key] {
			continue
		}
		seen[key] = true
		fmt.Fprintln(bw, line)
	}
}

func stdinIsPipe() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}
