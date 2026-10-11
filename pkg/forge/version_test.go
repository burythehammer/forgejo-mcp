// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import "testing"

func TestMajorVersion(t *testing.T) {
	cases := map[string]int{
		"16.0.3":                               16,
		"16.0.0-dev-741-6f391573+gitea-1.22.0": 16,
		"15.0.3":                               15,
		"v16.1":                                16,
		"16":                                   16,
	}
	for in, want := range cases {
		got, err := MajorVersion(in)
		if err != nil || got != want {
			t.Errorf("MajorVersion(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "dev", "x.1.2"} {
		if _, err := MajorVersion(bad); err == nil {
			t.Errorf("MajorVersion(%q) accepted a version without a numeric major", bad)
		}
	}
}
