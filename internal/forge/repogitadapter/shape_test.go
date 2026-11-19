// SPDX-License-Identifier: GPL-3.0-or-later

package repogitadapter

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// The repo-owned types are copies of the SDK's wire shapes, converted through
// JSON. This keeps them honest: if an SDK upgrade adds, renames or retypes a
// field, the shapes differ and this fails, instead of a tool silently dropping
// the field from its output.
func TestModelTypesMirrorTheSDKWireShape(t *testing.T) {
	t.Parallel()
	pairs := []struct{ sdk, own reflect.Type }{
		{reflect.TypeOf(forgejo_sdk.Branch{}), reflect.TypeOf(model.Branch{})},
		{reflect.TypeOf(forgejo_sdk.PayloadCommit{}), reflect.TypeOf(model.PayloadCommit{})},
		{reflect.TypeOf(forgejo_sdk.PayloadUser{}), reflect.TypeOf(model.PayloadUser{})},
		{reflect.TypeOf(forgejo_sdk.PayloadCommitVerification{}), reflect.TypeOf(model.PayloadCommitVerification{})},
		{reflect.TypeOf(forgejo_sdk.CommitMeta{}), reflect.TypeOf(model.CommitMeta{})},
		{reflect.TypeOf(forgejo_sdk.Identity{}), reflect.TypeOf(model.Identity{})},
		{reflect.TypeOf(forgejo_sdk.CommitUser{}), reflect.TypeOf(model.CommitUser{})},
		{reflect.TypeOf(forgejo_sdk.RepoCommit{}), reflect.TypeOf(model.RepoCommit{})},
		{reflect.TypeOf(forgejo_sdk.CommitStats{}), reflect.TypeOf(model.CommitStats{})},
		{reflect.TypeOf(forgejo_sdk.CommitAffectedFiles{}), reflect.TypeOf(model.CommitAffectedFiles{})},
		{reflect.TypeOf(forgejo_sdk.Commit{}), reflect.TypeOf(model.Commit{})},
		{reflect.TypeOf(forgejo_sdk.FileLinksResponse{}), reflect.TypeOf(model.FileLinksResponse{})},
		{reflect.TypeOf(forgejo_sdk.ContentsResponse{}), reflect.TypeOf(model.ContentsResponse{})},
		{reflect.TypeOf(forgejo_sdk.FileCommitResponse{}), reflect.TypeOf(model.FileCommitResponse{})},
		{reflect.TypeOf(forgejo_sdk.FileResponse{}), reflect.TypeOf(model.FileResponse{})},
		{reflect.TypeOf(forgejo_sdk.GitEntry{}), reflect.TypeOf(model.GitEntry{})},
		{reflect.TypeOf(forgejo_sdk.GitTreeResponse{}), reflect.TypeOf(model.GitTreeResponse{})},
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
