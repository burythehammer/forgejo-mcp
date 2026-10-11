// SPDX-License-Identifier: GPL-3.0-or-later

// Package decode fills a per-tool argument struct from an MCP tool call and
// validates it once, so adapters hand use cases typed values instead of
// reading the argument map field by field.
//
// Struct fields are named by `json:"name"` tags; `required:"true"` marks a
// field that must be present. Unknown arguments are ignored, as the
// hand-written handlers did. Use mcptest.ArgsMatchSchema to keep a struct and
// its tool's input schema in agreement.
package decode

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// Args decodes req's arguments into dst (a pointer to a struct) and reports
// every missing required field and every wrongly typed field in one error.
func Args(req mcp.CallToolRequest, dst any) error {
	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return errors.New("decode: dst must be a pointer to a struct")
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
		b, err := json.Marshal(raw)
		if err == nil {
			err = json.Unmarshal(b, rv.Elem().Field(i).Addr().Interface())
		}
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s has the wrong type", name))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("invalid arguments: %s", strings.Join(problems, "; "))
	}
	return nil
}
