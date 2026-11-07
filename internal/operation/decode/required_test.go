// SPDX-License-Identifier: GPL-3.0-or-later

package decode

import "testing"

func TestRequired(t *testing.T) {
	t.Parallel()
	if err := Required("org", ""); err == nil || err.Error() != "org is required" {
		t.Errorf("empty org: got %v, want \"org is required\"", err)
	}
	if err := Required("org", "acme"); err != nil {
		t.Errorf("present org: got %v, want nil", err)
	}
}

func TestRequiredRepo(t *testing.T) {
	t.Parallel()
	cases := []struct {
		owner, repo, want string
	}{
		{"", "", "owner is required"},
		{"", "r", "owner is required"},
		{"o", "", "repo is required"},
		{"o", "r", ""},
	}
	for _, c := range cases {
		err := RequiredRepo(c.owner, c.repo)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("RequiredRepo(%q, %q) = %v, want nil", c.owner, c.repo, err)
		case c.want != "" && (err == nil || err.Error() != c.want):
			t.Errorf("RequiredRepo(%q, %q) = %v, want %q", c.owner, c.repo, err, c.want)
		}
	}
}
