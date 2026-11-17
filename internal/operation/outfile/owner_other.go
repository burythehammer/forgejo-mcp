// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !unix

package outfile

import "os"

// checkOwnership does nothing off unix: permission bits and uids do not gate
// access to a directory there, so the check is not made.
func checkOwnership(os.FileInfo, int) error { return nil }
