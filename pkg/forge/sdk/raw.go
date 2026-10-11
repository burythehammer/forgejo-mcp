// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
)

// rawJSON performs a JSON request for the endpoints the SDK lacks. Paths are
// built with forgejo.APIPath so every segment is escaped. The credential is the
// request's, via ctx; the instance is WithRawBaseURL's when set. A server
// failure comes back as *forge.StatusError wrapping the original
// *forgejo.HTTPError, so the message is unchanged.
func (a *Adapter) rawJSON(ctx context.Context, method, path string, body, out any) (http.Header, error) {
	if a.rawBase != "" {
		path = strings.TrimRight(a.rawBase, "/") + "/api/v1" + path
	}
	header, err := forgejo.DoJSONWithHeader(ctx, method, path, body, out)
	var he *forgejo.HTTPError
	if errors.As(err, &he) {
		err = &forge.StatusError{StatusCode: he.StatusCode, Err: err}
	}
	return header, err
}

// totalCount is the X-Total-Count header, nil when absent or unparsable.
func totalCount(h http.Header) *int { return forgejo.TotalCountPtr(h) }
