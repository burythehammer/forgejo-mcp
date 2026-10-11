// SPDX-License-Identifier: GPL-3.0-or-later

// Package decode fills a per-tool argument struct from an MCP tool call and
// validates it once, so adapters hand use cases typed values instead of
// reading the argument map field by field.
//
// Struct fields are named by `json:"name"` tags; `required:"true"` marks a
// field that must be present. Unknown arguments are ignored, as the
// hand-written handlers did. A number given as a string ("7") is accepted, as
// some MCP clients send them that way. Use mcptest.ArgsMatchSchema to keep a struct and
// its tool's input schema in agreement.
package decode

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// takesStringNumber reports whether a string given for f is read as a number.
// A plain number field always does, as the search and pull tools always have.
// An optional number (a pointer) does only when tagged `numeric:"string"`: the
// original handlers read some optional numbers with a type assertion, which
// refuses a string, and others with to.Float64, which accepts one, so the tag
// keeps each field to the one it had.
func takesStringNumber(f reflect.StructField) bool {
	if f.Type.Kind() == reflect.Pointer {
		return f.Tag.Get("numeric") == "string" && isNumber(f.Type.Elem().Kind())
	}
	return isNumber(f.Type.Kind())
}

func isNumber(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

// Args decodes req's arguments into dst (a pointer to a struct) and reports
// every missing required field and every wrongly typed field in one error.
func Args(req mcp.CallToolRequest, dst any) error {
	problems, err := fill(req, dst)
	if err != nil {
		return err
	}
	if len(problems) > 0 {
		return fmt.Errorf("invalid arguments: %s", strings.Join(problems, "; "))
	}
	return nil
}

// Lax is Args for tools that must keep the forgiving behaviour of the
// hand-written handlers they replace: a missing or wrongly typed argument is
// not an error but leaves its field at the zero value, so the use case sees
// what the old code saw. Declare numbers as float64 to keep their truncation
// (int64(5.5) is 5; an integer field would reject 5.5).
func Lax(req mcp.CallToolRequest, dst any) error {
	_, err := fill(req, dst)
	return err
}

func fill(req mcp.CallToolRequest, dst any) ([]string, error) {
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return nil, errors.New("decode: dst must be a pointer to a struct")
	}
	args := req.GetArguments()

	var problems []string
	rt := rv.Elem().Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		raw, present := args[name]
		if !present || raw == nil {
			if f.Tag.Get("required") == "true" {
				problems = append(problems, fmt.Sprintf("%s is required", name))
			}
			continue
		}
		if s, isString := raw.(string); isString && takesStringNumber(f) {
			// Some MCP clients send number parameters as strings.
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			if _, perr := strconv.ParseFloat(s, 64); perr != nil {
				problems = append(problems, fmt.Sprintf("%s has the wrong type", name))
				continue
			}
			raw = json.Number(s)
		}
		b, err := json.Marshal(raw)
		if err == nil {
			err = json.Unmarshal(b, rv.Elem().Field(i).Addr().Interface())
		}
		if err != nil {
			// A pointer field can be left allocated by a failed decode, which
			// would read as "given"; a wrongly typed argument must read as
			// absent, as the old handlers' type assertions saw it.
			rv.Elem().Field(i).Set(reflect.Zero(f.Type))
			problems = append(problems, fmt.Sprintf("%s has the wrong type", name))
		}
	}
	return problems, nil
}
