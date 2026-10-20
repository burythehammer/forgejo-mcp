// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"fmt"

	apprepo "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/repogitadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

// gitService returns the branch, commit, contents and file use cases bound to
// this request's credential.
func (h handlers) gitService(ctx context.Context) (*apprepo.GitService, error) {
	f, err := h.forge.For(ctx)
	if err != nil {
		return nil, err
	}
	return apprepo.NewGitService(repogitadapter.New(f)), nil
}

// need is the value of a required argument. It reports the error the tools have
// always given when the argument is missing or not a string.
func need(name string, v *string) (string, error) {
	if v == nil {
		return "", fmt.Errorf("%s is required", name)
	}
	return *v, nil
}

// orDefault is a paging argument, or def when it was left out (zero).
func orDefault(v, def float64) float64 {
	if v == 0 {
		return def
	}
	return v
}

// gitText renders a use case's result, or its error, as the tools' JSON text.
func gitText(v any, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(v)
}
