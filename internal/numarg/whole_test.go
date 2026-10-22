// SPDX-License-Identifier: GPL-3.0-or-later

package numarg

import (
	"math"
	"testing"
)

// TestWholeTruncatesFractionsAndRefusesTheRest pins the seconds rule: a
// fraction truncates toward zero, and the truncated value must still be in
// range, so 0.5 is refused with a positive minimum.
func TestWholeTruncatesFractionsAndRefusesTheRest(t *testing.T) {
	t.Parallel()
	seconds := Bounds{Min: 1, Max: math.MaxInt64}
	const msg = "seconds must be an integer between 1 and 9223372036854775807"

	cases := []struct {
		name    string
		json    string
		want    int64
		given   bool
		wantErr string
	}{
		{"integer", "90", 90, true, ""},
		{"fraction truncates", "2.7", 2, true, ""},
		{"one and a half truncates to one", "1.5", 1, true, ""},
		{"numeric string fraction truncates", `"2.7"`, 2, true, ""},
		{"half a second refused", "0.5", 0, false, msg},
		{"just under one refused", "0.999", 0, false, msg},
		{"zero refused", "0", 0, false, msg},
		{"negative refused", "-5", 0, false, msg},
		{"non-finite string refused", `"NaN"`, 0, false, msg},
		{"overflow refused", "1e30", 0, false, msg},
		{"boolean refused", "true", 0, false, msg},
		{"malformed string refused", `"soon"`, 0, false, msg},
		{"null is absent", "null", 0, false, ""},
		{"empty string is absent", `""`, 0, false, ""},
		{"omitted is absent", "<omitted>", 0, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, given, err := Whole("seconds", raw(t, tc.json), seconds)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want || given != tc.given {
				t.Fatalf("Whole(%s) = (%d, %t), want (%d, %t)", tc.json, got, given, tc.want, tc.given)
			}
		})
	}
}
