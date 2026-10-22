// SPDX-License-Identifier: GPL-3.0-or-later

package numarg

import (
	"encoding/json"
	"math"
	"testing"
)

// raw decodes a JSON literal the way the MCP layer hands an argument to us.
// The sentinel "<omitted>" stands for an argument the client did not send,
// which the map lookup yields as nil.
func raw(t *testing.T, text string) any {
	t.Helper()
	if text == "<omitted>" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		t.Fatalf("bad JSON literal %q: %v", text, err)
	}
	return v
}

func TestIntAcceptsOnlyWholeNumbersInRange(t *testing.T) {
	t.Parallel()
	page := Bounds{Min: 1, Max: 50}
	withZero := Bounds{Min: 0, Max: 50}
	id := Bounds{Min: 1, Max: math.MaxInt64}

	const msg = "limit must be an integer between 1 and 50"
	const zeroMsg = "limit must be an integer between 0 and 50"
	const idMsg = "limit must be an integer between 1 and 9223372036854775807"

	cases := []struct {
		name    string
		json    string
		bounds  Bounds
		want    int64
		given   bool
		wantErr string
	}{
		// integers and range edges
		{"integer", "7", page, 7, true, ""},
		{"lower edge", "1", page, 1, true, ""},
		{"upper edge", "50", page, 50, true, ""},
		{"zero allowed by range", "0", withZero, 0, true, ""},
		{"exponent form", "1e2", id, 100, true, ""},
		{"above range", "51", page, 0, false, msg},
		{"zero refused by a positive range", "0", page, 0, false, msg},
		{"negative", "-3", page, 0, false, msg},
		{"negative with zero allowed", "-1", withZero, 0, false, zeroMsg},

		// fractions are refused, never truncated
		{"fraction", "1.5", page, 0, false, msg},
		{"fraction below one with zero allowed", "0.5", withZero, 0, false, zeroMsg},

		// numeric strings read as numbers; other strings are refused
		{"numeric string", `"12"`, page, 12, true, ""},
		{"numeric string fraction", `"1.5"`, page, 0, false, msg},
		{"numeric string out of range", `"51"`, page, 0, false, msg},
		{"numeric string keeps every digit past 2^53", `"9007199254740993"`, id, 9007199254740993, true, ""},
		{"numeric string at the largest int64", `"9223372036854775807"`, id, math.MaxInt64, true, ""},
		{"numeric string one past the largest int64", `"9223372036854775808"`, id, 0, false, idMsg},
		{"numeric string with a zero fraction", `"2.0"`, page, 2, true, ""},
		{"numeric string exponent form", `"1e3"`, id, 1000, true, ""},
		// a JSON number is a float64 before this module sees it, so the
		// exact value above 2^53 is lost at the decode boundary
		{"JSON number above 2^53 is float64", "9007199254740993", id, 9007199254740992, true, ""},
		{"empty string takes the default", `""`, page, 0, false, ""},
		{"non-numeric string", `"abc"`, page, 0, false, msg},
		{"NaN string", `"NaN"`, page, 0, false, msg},
		{"infinity string", `"Inf"`, page, 0, false, msg},
		{"negative infinity string", `"-Inf"`, page, 0, false, msg},
		{"string overflowing float", `"1e400"`, page, 0, false, msg},

		// absent and null mean "not given"
		{"null", "null", page, 0, false, ""},
		{"omitted", "<omitted>", page, 0, false, ""},

		// wrongly typed input
		{"boolean", "true", page, 0, false, msg},
		{"array", "[1]", page, 0, false, msg},
		{"object", "{}", page, 0, false, msg},

		// overflow: never wrapped, never a panic
		{"overflow past int64", "1e19", id, 0, false, idMsg},
		{"overflow far beyond", "1e300", id, 0, false, idMsg},
		{"large but in range", "1e18", id, 1000000000000000000, true, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, given, err := Int("limit", raw(t, c.json), c.bounds)
			if c.wantErr != "" {
				if err == nil {
					t.Fatalf("Int(%s) = %d, nil error; want error %q", c.json, got, c.wantErr)
				}
				if err.Error() != c.wantErr {
					t.Fatalf("Int(%s) error = %q, want %q", c.json, err.Error(), c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Int(%s) error = %v", c.json, err)
			}
			if got != c.want || given != c.given {
				t.Fatalf("Int(%s) = (%d, %v), want (%d, %v)", c.json, got, given, c.want, c.given)
			}
		})
	}
}

func TestRequiredIntReportsAMissingArgument(t *testing.T) {
	t.Parallel()
	page := Bounds{Min: 1, Max: 50}
	for _, text := range []string{"<omitted>", "null", `""`} {
		_, err := RequiredInt("limit", raw(t, text), page)
		if err == nil || err.Error() != "limit is required" {
			t.Errorf("RequiredInt(%s) error = %v, want \"limit is required\"", text, err)
		}
	}
	got, err := RequiredInt("limit", raw(t, "9"), page)
	if err != nil || got != 9 {
		t.Errorf("RequiredInt(9) = (%d, %v), want (9, nil)", got, err)
	}
}
