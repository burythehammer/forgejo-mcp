// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import "errors"

// errInvalidParams is the prefix of the commit-SHA validation errors. Its text
// is the one the tool has always returned.
var errInvalidParams = errors.New("invalid params")
