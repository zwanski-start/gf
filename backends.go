package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
)

// engine returns the command that should be run for the pattern,
// defaulting to grep
func (p pattern) engine() string {
	if p.Engine != "" {
		return p.Engine
	}
	return "grep"
}

// legacyKeys are the top-level keys of a single-engine pattern file.
// If any of them are present the file is treated as a single pattern
// rather than as a set of per-engine patterns.
var legacyKeys = map[string]bool{
	"flags":    true,
	"pattern":  true,
	"patterns": true,
	"engine":   true,
}

// parsePatternFile reads a pattern file and returns the candidate
// patterns in order of preference.
//
// Two formats are supported. The original single-engine format:
//
//	{"flags": "-HnrE", "pattern": "..."}
//
// and a multi-engine format where each top-level key is the command
// to run, tried in the order they appear in the file:
//
//	{
//	    "rg":   {"flags": "-oIN", "pattern": "..."},
//	    "grep": {"flags": "-oriahE", "pattern": "..."}
//	}
func parsePatternFile(r io.Reader) ([]pattern, error) {
	raw, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	keys, values, err := orderedObject(raw)
	if err != nil {
		return nil, err
	}

	multi := len(keys) > 0
	for _, k := range keys {
		if legacyKeys[k] {
			multi = false
			break
		}
	}

	if !multi {
		pat := pattern{}
		if err := json.Unmarshal(raw, &pat); err != nil {
			return nil, err
		}
		return []pattern{pat}, nil
	}

	out := make([]pattern, 0, len(keys))
	for i, k := range keys {
		pat := pattern{}
		if err := json.Unmarshal(values[i], &pat); err != nil {
			return nil, fmt.Errorf("engine '%s': %s", k, err)
		}
		if pat.Engine == "" {
			pat.Engine = k
		}
		out = append(out, pat)
	}
	return out, nil
}

// orderedObject decodes the top level of a JSON object, preserving
// the order of its keys
func orderedObject(raw []byte) ([]string, []json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))

	tok, err := dec.Token()
	if err != nil {
		return nil, nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, nil, errors.New("expected a JSON object")
	}

	var keys []string
	var values []json.RawMessage
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, nil, errors.New("expected an object key")
		}

		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, nil, err
		}

		keys = append(keys, key)
		values = append(values, v)
	}

	if _, err := dec.Token(); err != nil {
		return nil, nil, err
	}

	return keys, values, nil
}

// selectBackend returns the first pattern whose engine is available
// according to lookPath. A file with only one pattern is always used
// as-is so that the error from running a missing engine is preserved.
func selectBackend(pats []pattern, lookPath func(string) (string, error)) (pattern, bool) {
	if len(pats) == 1 {
		return pats[0], true
	}

	for _, p := range pats {
		if _, err := lookPath(p.engine()); err == nil {
			return p, true
		}
	}
	return pattern{}, false
}

// engineNames returns a comma separated list of the engines in pats
func engineNames(pats []pattern) string {
	names := make([]string, len(pats))
	for i, p := range pats {
		names[i] = p.engine()
	}
	return strings.Join(names, ", ")
}
