// SPDX-License-Identifier: GPL-3.0-or-later

package outfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type row struct {
	N     int64  `json:"number"`
	Title string `json:"title"`
}

func rowKey(r row) int64 { return r.N }

// useTempDir points os.TempDir at a fresh directory for the test, so the
// module writes under it and nothing lands in the real temp dir.
func useTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	return dir
}

func TestCheckName(t *testing.T) {
	t.Parallel()
	for _, ok := range []string{"issues.json", "a-b_c.txt", "list"} {
		if err := CheckName(ok); err != nil {
			t.Errorf("CheckName(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", ".", "..", "../x", "a/b", "a\\b", "/abs/x", "x/", "/", "..x/y"} {
		if err := CheckName(bad); err == nil {
			t.Errorf("CheckName(%q) = nil, want a refusal", bad)
		}
	}
}

func TestWriteReplyAndContents(t *testing.T) {
	dir := useTempDir(t)
	rows := []row{{N: 3, Title: "c"}, {N: 1, Title: "a"}}
	got, err := Write("list.json", rows, rowKey)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "forgejo-mcp", "list.json")
	if got.Path != want {
		t.Fatalf("path = %q, want %q", got.Path, want)
	}
	if !filepath.IsAbs(got.Path) {
		t.Fatalf("path %q is not absolute", got.Path)
	}
	if got.Count != 2 || !reflect.DeepEqual(got.Keys, []int64{3, 1}) {
		t.Fatalf("reply = %+v, want count 2 keys [3 1]", got)
	}
	b, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	var back []row
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("file is not a JSON array: %v", err)
	}
	if !reflect.DeepEqual(back, rows) {
		t.Fatalf("file rows = %+v, want %+v", back, rows)
	}
}

func TestWriteEmptyListingHasEmptyKeys(t *testing.T) {
	useTempDir(t)
	got, err := Write("empty.json", []row{}, rowKey)
	if err != nil {
		t.Fatal(err)
	}
	if got.Count != 0 || got.Keys == nil || len(got.Keys) != 0 {
		t.Fatalf("empty reply = %+v, want count 0 and keys []", got)
	}
}

func TestWriteOverwritesExistingFile(t *testing.T) {
	useTempDir(t)
	if _, err := Write("same.json", []row{{N: 1}, {N: 2}, {N: 3}}, rowKey); err != nil {
		t.Fatal(err)
	}
	got, err := Write("same.json", []row{{N: 9}}, rowKey)
	if err != nil {
		t.Fatal(err)
	}
	if got.Count != 1 || !reflect.DeepEqual(got.Keys, []int64{9}) {
		t.Fatalf("second reply = %+v", got)
	}
	b, err := os.ReadFile(got.Path)
	if err != nil {
		t.Fatal(err)
	}
	var back []row
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if len(back) != 1 || back[0].N != 9 {
		t.Fatalf("overwritten file = %+v, want one row 9", back)
	}
}

func TestWriteRefusesBadNameAndWritesNothing(t *testing.T) {
	dir := useTempDir(t)
	for _, bad := range []string{"", "..", "../escape.json", "a/b.json", "/abs.json"} {
		if _, err := Write(bad, []row{{N: 1}}, rowKey); err == nil {
			t.Errorf("Write(%q) succeeded, want a refusal", bad)
		}
	}
	entries, err := os.ReadDir(dir)
	if err == nil && len(entries) != 0 {
		t.Fatalf("refused writes left files under %s: %v", dir, entries)
	}
	if _, err := os.Stat(filepath.Join(dir, "escape.json")); err == nil {
		t.Fatal("a refused name escaped the temp dir")
	}
}
