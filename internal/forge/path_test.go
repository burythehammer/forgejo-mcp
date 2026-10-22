// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"errors"
	"strings"
	"testing"
)

func TestAPIPath_EscapesSegments(t *testing.T) {
	tests := []struct {
		name     string
		segments []any
		want     string
	}{
		{
			name:     "plain segments are untouched",
			segments: []any{"repos", "goern", "forgejo-mcp", "issues", int64(7), "assets"},
			want:     "/repos/goern/forgejo-mcp/issues/7/assets",
		},
		{
			name:     "slashes cannot break out of their segment",
			segments: []any{"repos", "o/x/y/../admin", "r", "issues"},
			want:     "/repos/o%2Fx%2Fy%2F..%2Fadmin/r/issues",
		},
		{
			name:     "a question mark cannot start a query string",
			segments: []any{"repos", "o", "r?state=all", "issues"},
			want:     "/repos/o/r%3Fstate=all/issues",
		},
		{
			name:     "int and int64 ids format naturally",
			segments: []any{"orgs", "acme", "labels", 12},
			want:     "/orgs/acme/labels/12",
		},
		{
			name:     "spaces and unicode are escaped",
			segments: []any{"repos", "o", "my repo", "wiki", "Seite Ü"},
			want:     "/repos/o/my%20repo/wiki/Seite%20%C3%9C",
		},
		{
			name:     "dot-prefixed names are kept",
			segments: []any{"repos", "o", "r", "contents", ".github"},
			want:     "/repos/o/r/contents/.github",
		},
		{
			name:     "names with dots inside are kept",
			segments: []any{"repos", "o", "r", "contents", "a.b.go"},
			want:     "/repos/o/r/contents/a.b.go",
		},
		{
			name:     "three dots are an ordinary name",
			segments: []any{"repos", "o", "r", "contents", "..."},
			want:     "/repos/o/r/contents/...",
		},
		{
			name:     "an escaped slash inside a dotted owner is kept",
			segments: []any{"repos", "o/x/../../admin", "r", "issues"},
			want:     "/repos/o%2Fx%2F..%2F..%2Fadmin/r/issues",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := APIPath(tt.segments...)
			if err != nil {
				t.Fatalf("APIPath(%v) error = %v, want %q", tt.segments, err, tt.want)
			}
			if got != tt.want {
				t.Fatalf("APIPath(%v) = %q, want %q", tt.segments, got, tt.want)
			}
		})
	}
}

func TestAPIPath_RejectsDotSegments(t *testing.T) {
	tests := []struct {
		name     string
		segments []any
		bad      string
	}{
		{name: "parent segment as owner", segments: []any{"repos", "..", "r", "issues"}, bad: ".."},
		{name: "current segment as owner", segments: []any{"repos", ".", "r", "issues"}, bad: "."},
		{name: "parent segment as repo", segments: []any{"repos", "o", "..", "issues"}, bad: ".."},
		{name: "parent segment inside a file path", segments: []any{"repos", "o", "r", "contents", "a", "..", "b"}, bad: ".."},
		{name: "parent segment as a branch", segments: []any{"repos", "o", "r", "branches", ".."}, bad: ".."},
		{name: "current segment as a file", segments: []any{"repos", "o", "r", "contents", "."}, bad: "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := APIPath(tt.segments...)
			if !errors.Is(err, ErrDotSegment) {
				t.Fatalf("APIPath(%v) = %q, %v; want ErrDotSegment", tt.segments, got, err)
			}
			if got != "" {
				t.Fatalf("APIPath(%v) returned %q with an error; want empty", tt.segments, got)
			}
			if !strings.Contains(err.Error(), `"`+tt.bad+`"`) {
				t.Fatalf("error %q does not name the segment %q", err, tt.bad)
			}
		})
	}
}
