// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"fmt"
	"math"
	"strconv"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/numarg"
)

// Each tool decodes its arguments with decode.Lax into a struct beside its
// handler. Strings decode as strings. Numbers are typed any, not float64: the
// tools' error texts depend on how a number was given. An absent or empty
// value is "is required" (or the default), a wrongly typed one is "must be a
// positive integer" (or an error for a bounded number), and a string holding a
// number is read as that number. Decoding into float64 would collapse those
// cases, so the helpers below read them, and mcptest.ArgsMatchSchema is told
// the schema type with a schema tag.

// number reads a numeric argument: a JSON number, or a string holding one, as
// some MCP clients send numbers as strings. ok is false for anything else.
func number(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	}
	return 0, false
}

// absent reports whether a numeric argument counts as not given.
func absent(v any) bool { return v == nil || v == "" }

// required returns the error for a string argument that is missing or empty.
func required(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

// requiredRepo checks owner, then repo, the order the tools always used.
func requiredRepo(owner, repo string) error {
	if err := required("owner", owner); err != nil {
		return err
	}
	return required("repo", repo)
}

// positiveIntegerArg reads a positive whole number through the numeric-argument
// module. An absent value is an error only when required.
func positiveIntegerArg(v any, name string, isRequired bool) (int64, error) {
	if isRequired {
		return numarg.RequiredPositiveInt(name, v)
	}
	n, _, err := numarg.PositiveInt(name, v)
	return n, err
}

// optionalNonNegativeIntegerArg reads a non-negative whole number and reports
// whether one was given.
func optionalNonNegativeIntegerArg(v any, name string) (int64, bool, error) {
	if absent(v) {
		return 0, false, nil
	}
	n, ok := number(v)
	if !ok || n < 0 || math.Trunc(n) != n || n >= float64(math.MaxInt64) {
		return 0, false, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return int64(n), true, nil
}

// boundedIntArg reads a whole number within [minimum, maximum]. An absent value
// takes defaultValue.
func boundedIntArg(v any, name string, defaultValue, minimum, maximum int) (int, error) {
	if absent(v) {
		return defaultValue, nil
	}
	n, ok := number(v)
	if !ok || math.Trunc(n) != n || n < float64(minimum) || n > float64(maximum) {
		return 0, fmt.Errorf("%s must be an integer between %d and %d", name, minimum, maximum)
	}
	return int(n), nil
}
