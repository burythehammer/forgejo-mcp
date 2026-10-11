// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"testing"
)

func TestAPIPath_EscapesSegments(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{
			name: "plain segments are untouched",
			got:  APIPath("repos", "goern", "forgejo-mcp", "issues", int64(7), "assets"),
			want: "/repos/goern/forgejo-mcp/issues/7/assets",
		},
		{
			name: "slashes cannot break out of their segment",
			got:  APIPath("repos", "o/x/../../admin", "r", "issues"),
			want: "/repos/o%2Fx%2F..%2F..%2Fadmin/r/issues",
		},
		{
			name: "a question mark cannot start a query string",
			got:  APIPath("repos", "o", "r?state=all", "issues"),
			want: "/repos/o/r%3Fstate=all/issues",
		},
		{
			name: "int and int64 ids format naturally",
			got:  APIPath("orgs", "acme", "labels", 12) + APIPath("x", int64(34)),
			want: "/orgs/acme/labels/12/x/34",
		},
		{
			name: "spaces and unicode are escaped",
			got:  APIPath("repos", "o", "my repo", "wiki", "Seite Ü"),
			want: "/repos/o/my%20repo/wiki/Seite%20%C3%9C",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("APIPath = %q, want %q", tt.got, tt.want)
			}
		})
	}
}
