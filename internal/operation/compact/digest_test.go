// SPDX-License-Identifier: GPL-3.0-or-later

package compact

import "testing"

func TestBodySHA256_KnownVectors(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"abc": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
	}
	for in, want := range cases {
		if got := BodySHA256(in); got != want {
			t.Fatalf("BodySHA256(%q) = %s, want %s", in, got, want)
		}
	}
}
