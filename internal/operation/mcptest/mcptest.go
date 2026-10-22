// SPDX-License-Identifier: GPL-3.0-or-later

// Package mcptest holds test helpers shared by every MCP adapter: golden
// output, schema snapshots and the args-struct/schema agreement check.
package mcptest

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/lean"

	"github.com/mark3labs/mcp-go/mcp"
)

var update = flag.Bool("update", false, "rewrite golden and schema snapshot files")

// EnvelopeKeys decodes the {"Result": {...}} envelope that internal/to wraps around a
// tool's output and returns the keys of the result object. It fails the test if
// the result is not an object.
func EnvelopeKeys(t *testing.T, text string) map[string]json.RawMessage {
	t.Helper()
	var wrapped struct{ Result map[string]json.RawMessage }
	if err := json.Unmarshal([]byte(text), &wrapped); err != nil {
		t.Fatalf("not a result envelope: %v\n%s", err, text)
	}
	if wrapped.Result == nil {
		t.Fatalf("result is not an object: %s", text)
	}
	return wrapped.Result
}

// Golden compares got with testdata/<name>.golden byte for byte. Run the
// package's tests with -update to (re)write the file.
func Golden(t *testing.T, name string, got []byte) {
	t.Helper()
	compare(t, filepath.Join("testdata", name+".golden"), got)
}

// SchemaSnapshot pins the JSON of a tool definition (name, description,
// input schema) in testdata/<tool>.schema.json.
func SchemaSnapshot(t *testing.T, tool mcp.Tool) {
	t.Helper()
	// The keep marker that lean reads is not part of the served definition.
	b, err := json.MarshalIndent(lean.Strip(tool), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	compare(t, filepath.Join("testdata", tool.Name+".schema.json"), append(b, '\n'))
}

func compare(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s (run with -update to create): %v", path, err)
	}
	if string(want) != string(got) {
		t.Fatalf("%s differs\nwant: %q\n got: %q", path, want, got)
	}
}

// ArgsMatchSchema asserts that the argument struct a tool decodes into
// agrees with the tool's input schema: same property names, compatible
// types, same required set. Struct fields use `json:"name"` tags and
// `required:"true"` for required ones.
func ArgsMatchSchema(t *testing.T, tool mcp.Tool, args any) {
	t.Helper()
	rt := reflect.TypeOf(args)
	for rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	fields := map[string]string{}
	var required []string
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		// A field kept raw (any) names its schema type with a schema tag.
		typ := schemaType(f.Type)
		if s := f.Tag.Get("schema"); s != "" {
			typ = s
		}
		fields[name] = typ
		if f.Tag.Get("required") == "true" {
			required = append(required, name)
		}
	}
	for name, typ := range fields {
		p, ok := tool.InputSchema.Properties[name].(map[string]any)
		if !ok {
			t.Errorf("struct field %q has no schema property", name)
			continue
		}
		if p["type"] != typ {
			t.Errorf("field %q: struct type %q, schema type %v", name, typ, p["type"])
		}
	}
	for name := range tool.InputSchema.Properties {
		if _, ok := fields[name]; !ok {
			t.Errorf("schema property %q has no struct field", name)
		}
	}
	sort.Strings(required)
	want := append([]string(nil), tool.InputSchema.Required...)
	sort.Strings(want)
	if len(required) != len(want) || (len(want) > 0 && !reflect.DeepEqual(required, want)) {
		t.Errorf("required set: struct %v, schema %v", required, want)
	}
}

func schemaType(t reflect.Type) string {
	// A pointer field is an optional argument whose type is the pointee's.
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Slice, reflect.Array:
		return "array"
	default:
		return "object"
	}
}
