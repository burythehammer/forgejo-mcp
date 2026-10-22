// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrDotSegment is returned by APIPath for a segment that is exactly "." or "..".
// A proxy in front of Forgejo may collapse such a segment and route the request
// to a different endpoint, and the escaping in APIPath does not change dots, so
// the builder refuses them instead. Names that merely contain dots are fine.
var ErrDotSegment = errors.New("path segment is a dot segment")

// APIPath builds a REST API path from segments, escaping every one of them.
//
// It exists because the raw-HTTP helpers in this package take a path string
// and concatenate it onto the base URL verbatim, while the SDK client escapes
// user-supplied segments for us (escapeValidatePathSegments). Building a path
// with a bare fmt.Sprintf therefore lets a tool argument containing "/" or "?"
// escape its segment and retarget the request at a different endpoint — which
// matters most for the DELETE/PATCH call sites.
//
// String segments are url.PathEscape'd; anything else is formatted with %v
// first (so int/int64 IDs read naturally at the call site) and escaped too,
// which is a no-op for digits. Every segment is prefixed with "/", so
//
//	APIPath("repos", owner, repo, "issues", index, "assets")
//
// yields "/repos/{owner}/{repo}/issues/{index}/assets". A segment that is
// exactly "." or ".." is refused with ErrDotSegment and no path is returned;
// escaping cannot neutralise it, so the builder does not try. Query strings are
// not this function's job: append them to the result, escaping values yourself.
func APIPath(segments ...any) (string, error) {
	var b strings.Builder
	for _, seg := range segments {
		s, ok := seg.(string)
		if !ok {
			s = fmt.Sprintf("%v", seg)
		}
		if s == "." || s == ".." {
			return "", fmt.Errorf("%w: %q", ErrDotSegment, s)
		}
		b.WriteByte('/')
		b.WriteString(url.PathEscape(s))
	}
	return b.String(), nil
}
