// SPDX-License-Identifier: GPL-3.0-or-later

//go:build unix

package outfile

import (
	"fmt"
	"os"
	"syscall"
)

// checkOwnership refuses a directory open to group or other users, or owned by
// a uid other than uid. The directory is 0700 when this server creates it.
func checkOwnership(fi os.FileInfo, uid int) error {
	if fi.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("is open to group or other users (mode %04o)", fi.Mode().Perm())
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && int(st.Uid) != uid {
		return fmt.Errorf("is owned by uid %d, not the current uid %d", st.Uid, uid)
	}
	return nil
}
