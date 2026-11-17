// SPDX-License-Identifier: GPL-3.0-or-later

package outfile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestWriteRefusesASymlinkedDir: a symlink planted at the output directory
// would send the file to wherever it points, so the module refuses it.
func TestWriteRefusesASymlinkedDir(t *testing.T) {
	tmp := useTempDir(t)
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(tmp, dirName)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := Write("x.json", []row{{N: 1}}, rowKey); err == nil {
		t.Fatal("Write through a symlinked output dir succeeded, want a refusal")
	}
	entries, err := os.ReadDir(target)
	if err != nil || len(entries) != 0 {
		t.Fatalf("the symlink target received files: %v (err %v)", entries, err)
	}
}

// TestWriteRefusesADirOpenToOtherUsers: a pre-existing output directory with
// group or other access is refused, not written into.
func TestWriteRefusesADirOpenToOtherUsers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits do not gate access on windows")
	}
	tmp := useTempDir(t)
	dir := filepath.Join(tmp, dirName)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Write("x.json", []row{{N: 1}}, rowKey); err == nil {
		t.Fatal("Write into a 0755 output dir succeeded, want a refusal")
	}
}

// TestCheckOwnershipRefusesAnotherUID: the directory must belong to the
// current uid.
func TestCheckOwnershipRefusesAnotherUID(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("ownership is not checked on windows")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkOwnership(fi, os.Getuid()); err != nil {
		t.Fatalf("own 0700 dir refused: %v", err)
	}
	if err := checkOwnership(fi, os.Getuid()+1); err == nil {
		t.Fatal("dir owned by another uid accepted, want a refusal")
	}
}

// TestWriteLeavesOnlyTheNamedFile: the write goes through a temp file that is
// renamed into place, so no temp file survives a successful write.
func TestWriteLeavesOnlyTheNamedFile(t *testing.T) {
	tmp := useTempDir(t)
	if _, err := Write("only.json", []row{{N: 1}}, rowKey); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(tmp, dirName))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "only.json" {
		t.Fatalf("output dir holds %v, want only only.json", entries)
	}
}
