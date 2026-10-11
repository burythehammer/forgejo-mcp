// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import "net/http"

// totalCount is the X-Total-Count header, nil when absent or unparsable.
func totalCount(h http.Header) *int { return totalCountPtr(h) }
