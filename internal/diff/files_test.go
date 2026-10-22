// SPDX-License-Identifier: GPL-3.0-or-later

package diff

import (
	"strings"
	"testing"
)

func TestFilesJoinsBackToTheWholeDiff(t *testing.T) {
	t.Parallel()
	raw := "diff --git a/a.go b/a.go\n+a\ndiff --git a/b.go b/b.go\n+b\n"
	got := Files(raw)
	if len(got) != 2 {
		t.Fatalf("want 2 sections, got %d: %q", len(got), got)
	}
	if got[0] != "diff --git a/a.go b/a.go\n+a" || got[1] != "diff --git a/b.go b/b.go\n+b\n" {
		t.Fatalf("sections: %q", got)
	}
	if joined := strings.Join(got, "\n"); joined != raw {
		t.Fatalf("join %q, want %q", joined, raw)
	}
}

func TestFilesKeepsTextBeforeTheFirstHeader(t *testing.T) {
	t.Parallel()
	raw := "preamble\ndiff --git a/x b/x\n+x"
	got := Files(raw)
	if strings.Join(got, "\n") != raw || len(got) != 2 {
		t.Fatalf("sections %q", got)
	}
}

func TestFilesOfAnEmptyDiffIsEmpty(t *testing.T) {
	t.Parallel()
	if got := Files(""); len(got) != 0 {
		t.Fatalf("got %q", got)
	}
}
