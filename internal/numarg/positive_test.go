// SPDX-License-Identifier: GPL-3.0-or-later

package numarg

import "testing"

func TestPositiveIntAcceptsOnlyPositiveWholeNumbers(t *testing.T) {
	t.Parallel()
	const msg = "run_id must be a positive integer"

	cases := []struct {
		name      string
		json      string
		want      int64
		wantGiven bool
		wantErr   string
	}{
		{name: "one", json: `1`, want: 1, wantGiven: true},
		{name: "large whole number", json: `9e18`, want: 9000000000000000000, wantGiven: true},
		{name: "numeric string", json: `"12"`, want: 12, wantGiven: true},
		{name: "zero", json: `0`, wantErr: msg},
		{name: "negative", json: `-3`, wantErr: msg},
		{name: "fraction", json: `1.5`, wantErr: msg},
		{name: "overflow", json: `1e19`, wantErr: msg},
		{name: "two to the 63", json: `9223372036854775808`, wantErr: msg},
		{name: "nan string", json: `"NaN"`, wantErr: msg},
		{name: "text string", json: `"abc"`, wantErr: msg},
		{name: "boolean", json: `true`, wantErr: msg},
		{name: "object", json: `{}`, wantErr: msg},
		{name: "omitted", json: `<omitted>`, wantGiven: false},
		{name: "null", json: `null`, wantGiven: false},
		{name: "empty string", json: `""`, wantGiven: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, given, err := PositiveInt("run_id", raw(t, c.json))
			if c.wantErr != "" {
				if err == nil || err.Error() != c.wantErr {
					t.Fatalf("PositiveInt(%s) error = %v, want %q", c.json, err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("PositiveInt(%s) unexpected error: %v", c.json, err)
			}
			if given != c.wantGiven || got != c.want {
				t.Fatalf("PositiveInt(%s) = (%d, %v), want (%d, %v)", c.json, got, given, c.want, c.wantGiven)
			}
		})
	}
}

func TestRequiredPositiveIntNamesAnAbsentArgument(t *testing.T) {
	t.Parallel()

	if _, err := RequiredPositiveInt("attachment_id", nil); err == nil || err.Error() != "attachment_id is required" {
		t.Fatalf("absent: error = %v, want %q", err, "attachment_id is required")
	}
	if _, err := RequiredPositiveInt("attachment_id", ""); err == nil || err.Error() != "attachment_id is required" {
		t.Fatalf("empty: error = %v, want %q", err, "attachment_id is required")
	}
	if _, err := RequiredPositiveInt("attachment_id", float64(0)); err == nil || err.Error() != "attachment_id must be a positive integer" {
		t.Fatalf("zero: error = %v, want %q", err, "attachment_id must be a positive integer")
	}
	got, err := RequiredPositiveInt("attachment_id", float64(7))
	if err != nil || got != 7 {
		t.Fatalf("seven = (%d, %v), want (7, nil)", got, err)
	}
}
