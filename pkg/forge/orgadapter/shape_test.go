// SPDX-License-Identifier: GPL-3.0-or-later

package orgadapter

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// The repo-owned types are copies of the SDK's wire shapes, converted through
// JSON. This keeps them honest: if an SDK upgrade adds, renames or retypes a
// field, the shapes differ and this fails, instead of a tool silently
// dropping the field from its output.
func TestModelTypesMirrorTheSDKWireShape(t *testing.T) {
	t.Parallel()
	pairs := []struct{ sdk, own reflect.Type }{
		{reflect.TypeOf(forgejo_sdk.User{}), reflect.TypeOf(model.User{})},
		{reflect.TypeOf(forgejo_sdk.Organization{}), reflect.TypeOf(model.Organization{})},
		{reflect.TypeOf(forgejo_sdk.Team{}), reflect.TypeOf(model.Team{})},
	}
	for _, p := range pairs {
		t.Run(p.own.Name(), func(t *testing.T) {
			if got, want := shape(p.own, nil), shape(p.sdk, nil); got != want {
				t.Fatalf("model shape differs from the SDK's\nmodel: %s\n  sdk: %s", got, want)
			}
		})
	}
}

// shape renders the JSON-visible structure of t: field names in order, with
// kinds (named string types count as string) and pointer/slice nesting.
func shape(t reflect.Type, seen []reflect.Type) string {
	for _, s := range seen {
		if s == t {
			return "<" + t.Name() + ">"
		}
	}
	switch t.Kind() {
	case reflect.Pointer:
		return "*" + shape(t.Elem(), seen)
	case reflect.Slice:
		return "[]" + shape(t.Elem(), seen)
	case reflect.Map:
		return "map[" + shape(t.Key(), seen) + "]" + shape(t.Elem(), seen)
	case reflect.Struct:
		if t.PkgPath() == "time" {
			return "time"
		}
		var b strings.Builder
		b.WriteString("{")
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.Tag.Get("json") == "-" {
				continue
			}
			name := f.Tag.Get("json")
			if name == "" {
				name = f.Name
			}
			fmt.Fprintf(&b, "%s:%s;", name, shape(f.Type, append(seen, t)))
		}
		b.WriteString("}")
		return b.String()
	default:
		return t.Kind().String()
	}
}
