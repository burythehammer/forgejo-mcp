// SPDX-License-Identifier: GPL-3.0-or-later

package decode

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
)

// Arg is one argument as the client sent it. Use it for a field whose legacy
// error text depends on whether the argument was sent at all, or sent as null:
// a plain string or number field cannot tell "absent" from "null" or from the
// wrong type. Give the field a schema tag (`schema:"string"`, `"number"` or
// `"array"`), since its Go type does not name the schema type.
type Arg struct {
	set  bool
	null bool
	raw  json.RawMessage
}

var argType = reflect.TypeOf(Arg{})

// UnmarshalJSON records a value the client sent, of any JSON type.
func (a *Arg) UnmarshalJSON(b []byte) error {
	a.set, a.null = true, false
	a.raw = append(json.RawMessage(nil), b...)
	return nil
}

// Present reports whether the client sent the argument, null included.
func (a Arg) Present() bool { return a.set }

// String returns the argument as a string, as the hand-written handlers read
// it. ok is false when the argument was not sent. It is an error when the
// argument was sent but is not a JSON string, null included; the text is
// "<name> must be a string".
func (a Arg) String(name string) (string, bool, error) {
	if !a.set {
		return "", false, nil
	}
	var s string
	if a.null || json.Unmarshal(a.raw, &s) != nil {
		return "", true, fmt.Errorf("%s must be a string", name)
	}
	return s, true, nil
}

// Int returns the argument as a whole number within [minimum, maximum]. An
// argument not sent, sent as null, or sent as an empty string takes def. A
// number given as a string is read as one. Anything else is an error, worded
// "<name> must be an integer between <minimum> and <maximum>".
func (a Arg) Int(name string, def, minimum, maximum int) (int, error) {
	if !a.set || a.null {
		return def, nil
	}
	var v any
	if err := json.Unmarshal(a.raw, &v); err != nil {
		return 0, intError(name, minimum, maximum)
	}
	var number float64
	switch x := v.(type) {
	case float64:
		number = x
	case string:
		if x == "" {
			return def, nil
		}
		f, err := strconv.ParseFloat(x, 64)
		if err != nil {
			return 0, intError(name, minimum, maximum)
		}
		number = f
	default:
		return 0, intError(name, minimum, maximum)
	}
	if math.Trunc(number) != number || number < float64(minimum) || number > float64(maximum) {
		return 0, intError(name, minimum, maximum)
	}
	return int(number), nil
}

func intError(name string, minimum, maximum int) error {
	return fmt.Errorf("%s must be an integer between %d and %d", name, minimum, maximum)
}

// Map decodes a plain argument map into dst (a pointer to a struct), the way
// Lax decodes a request. It is for an object nested in an argument, such as one
// entry of a batch. Like Lax it never reports a wrongly typed or missing field.
func Map(args map[string]any, dst any) error {
	_, err := fill(args, dst)
	return err
}
