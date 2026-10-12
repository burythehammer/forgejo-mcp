// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mustReadSource reads a source file in this package, for the two tests that
// assert on the shape of the code rather than on its behaviour. Both guard
// properties no behavioural test can see: that no transport binds its own
// listener, and that the conformance table covers every transport.
func mustReadSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

// mustReadPackageSources returns every non-test Go file in this package, keyed
// by file name. Shape tests scan all of them, so a violation in any file is
// caught, not only one in the file a test happens to name.
func mustReadPackageSources(t *testing.T) map[string]string {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("list package sources: %v", err)
	}
	sources := make(map[string]string, len(paths))
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		sources[p] = mustReadSource(t, p)
	}
	if len(sources) == 0 {
		t.Fatal("found no non-test Go sources in the package")
	}
	return sources
}
