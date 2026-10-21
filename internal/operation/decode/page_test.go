// SPDX-License-Identifier: GPL-3.0-or-later

package decode

import "testing"

func TestPageOrDefaultsOnlyAnAbsentOrZeroNumber(t *testing.T) {
	cases := []struct {
		v    float64
		want int
	}{
		{0, 7},   // absent: the default
		{0.5, 0}, // fractional zero is not defaulted; it truncates and is passed on
		{1, 1},
		{3.9, 3},
	}
	for _, c := range cases {
		if got := PageOr(c.v, 7); got != c.want {
			t.Errorf("PageOr(%v, 7) = %d, want %d", c.v, got, c.want)
		}
	}
}

func TestIntOrDefaultsTheTruncatedNumber(t *testing.T) {
	cases := []struct {
		v    float64
		want int
	}{
		{0, 7},
		{0.5, 7}, // truncates to 0, so the default applies
		{2, 2},
		{3.9, 3},
	}
	for _, c := range cases {
		if got := IntOr(c.v, 7); got != c.want {
			t.Errorf("IntOr(%v, 7) = %d, want %d", c.v, got, c.want)
		}
	}
}
