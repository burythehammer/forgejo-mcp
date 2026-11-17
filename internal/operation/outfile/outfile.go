// SPDX-License-Identifier: GPL-3.0-or-later

// Package outfile writes a tool's whole result to a file, so a list the client
// processes by script never passes through the model's context. Every tool that
// takes outFile uses this module, so the name rules, the location and the reply
// are the same everywhere. See GLOSSARY.md, "file output".
package outfile

import (
	"encoding/json"
	"errors"
	"fmt"
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
//
// The write goes to a temp file in the same directory that is renamed over the
// target only once it is complete, so a failed write leaves an earlier file of
// the same name intact.
func Write[T any](name string, rows []T, key func(T) int64) (Result, error) {
	if err := CheckName(name); err != nil {
		return Result{}, err
	}
	if rows == nil {
		rows = []T{}
	}
	dir, err := outputDir()
	if err != nil {
		return Result{}, err
	}
	body, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return Result{}, err
	}
	path := filepath.Join(dir, name)
	if err := replaceFile(dir, path, append(body, '\n')); err != nil {
		return Result{}, err
	}
	keys := make([]int64, 0, len(rows))
	for _, r := range rows {
		keys = append(keys, key(r))
	}
	return Result{Path: path, Count: len(rows), Keys: keys}, nil
}

// outputDir returns the output directory, creating it at 0700 on demand, and
// refuses one that is not safe to write into: a symbolic link (the file would
// land wherever it points), something other than a directory, a directory open
// to group or other users, or one owned by another uid. A directory left from
// an older run with looser permissions is refused too; remove it to recreate.
func outputDir() (string, error) {
	dir, err := filepath.Abs(filepath.Join(os.TempDir(), dirName))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
		return "", fmt.Errorf("output directory %s is a symbolic link or not a directory; remove it and retry", dir)
	}
	if err := checkOwnership(fi, os.Getuid()); err != nil {
		return "", fmt.Errorf("output directory %s %w; remove it and retry", dir, err)
	}
	return dir, nil
}

// replaceFile writes body to a temp file in dir and renames it to path. The
// temp file is removed if any step fails.
func replaceFile(dir, path string, body []byte) (err error) {
	f, err := os.CreateTemp(dir, ".write-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmp)
		}
	}()
	if _, err = f.Write(body); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
