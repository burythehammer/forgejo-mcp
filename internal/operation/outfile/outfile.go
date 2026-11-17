// SPDX-License-Identifier: GPL-3.0-or-later

// Package outfile writes a tool's whole result to a file, so a list the client
// processes by script never passes through the model's context. Every tool that
// takes outFile uses this module, so the name rules, the location and the reply
// are the same everywhere. See GLOSSARY.md, "file output".
package outfile

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// dirName is the directory, under the system temp dir, that holds output files.
const dirName = "forgejo-mcp"

// Result is the reply of a tool that wrote its result to a file. Path is
// absolute, Count is the number of rows written and Keys are their issue
// numbers in file order.
type Result struct {
	Path  string  `json:"path"`
	Count int     `json:"count"`
	Keys  []int64 `json:"keys"`
}

// CheckName refuses a name that is not a bare file name: an empty name, a path
// separator, or "..". A caller cannot write outside the output directory.
func CheckName(name string) error {
	switch {
	case name == "":
		return errors.New("outFile is empty: give a bare file name")
	case strings.ContainsAny(name, "/\\\x00") || strings.Contains(name, ".."):
		return errors.New("outFile must be a bare file name (no path separator, no \"..\")")
	case name == ".":
		return errors.New("outFile must name a file, not the directory")
	}
	return nil
}

// Write checks name, then writes rows as a JSON array into the output
// directory, creating it on demand and overwriting a file of the same name. key
// gives each row's issue number. A refused name writes nothing.
func Write[T any](name string, rows []T, key func(T) int64) (Result, error) {
	if err := CheckName(name); err != nil {
		return Result{}, err
	}
	if rows == nil {
		rows = []T{}
	}
	dir, err := filepath.Abs(filepath.Join(os.TempDir(), dirName))
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Result{}, err
	}
	body, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return Result{}, err
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, append(body, '\n'), 0o600); err != nil {
		return Result{}, err
	}
	keys := make([]int64, 0, len(rows))
	for _, r := range rows {
		keys = append(keys, key(r))
	}
	return Result{Path: path, Count: len(rows), Keys: keys}, nil
}
