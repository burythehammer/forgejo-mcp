// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"errors"
	"fmt"
	"math"

	app "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/actions"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/actionsadapter"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"
)

// useCases returns the Actions use cases over the request's Forgejo port. It
// is called after the arguments are checked, as the handlers always did.
func useCases(ctx context.Context, h handlers) (*app.Service, error) {
	port, err := h.forge.For(ctx)
	if err != nil {
		return nil, err
	}
	return app.New(actionsadapter.New(port)), nil
}

// The helpers below read raw MCP arguments. They are the adapter's share of
// the work: the messages are the tools' own, and a missing or mistyped value
// is an error here, before the use case runs.

func requiredRepo(args map[string]any) (string, string, error) {
	owner, ok := args["owner"].(string)
	if !ok || owner == "" {
		return "", "", errors.New("owner is required")
	}
	repo, ok := args["repo"].(string)
	if !ok || repo == "" {
		return "", "", errors.New("repo is required")
	}
	return owner, repo, nil
}

func requiredOrg(args map[string]any) (string, error) {
	org, ok := args["org"].(string)
	if !ok || org == "" {
		return "", errors.New("org is required")
	}
	return org, nil
}

func requiredSecretName(args map[string]any) (string, error) {
	name, ok := args["secret_name"].(string)
	if !ok || name == "" {
		return "", errors.New("secret_name is required")
	}
	return name, nil
}

func requiredSecretData(args map[string]any) (string, error) {
	data, ok := args["data"].(string)
	if !ok || data == "" {
		return "", errors.New("data is required")
	}
	return data, nil
}

func positiveIntegerArg(args map[string]any, name string, required bool) (int64, error) {
	value, exists := args[name]
	if !exists || value == nil || value == "" {
		if required {
			return 0, fmt.Errorf("%s is required", name)
		}
		return 0, nil
	}
	number, err := to.Float64(value)
	if err != nil || number <= 0 || math.Trunc(number) != number || number >= float64(math.MaxInt64) {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return int64(number), nil
}

func optionalNonNegativeIntegerArg(args map[string]any, name string) (int64, bool, error) {
	value, exists := args[name]
	if !exists || value == nil || value == "" {
		return 0, false, nil
	}
	number, err := to.Float64(value)
	if err != nil || number < 0 || math.Trunc(number) != number || number >= float64(math.MaxInt64) {
		return 0, false, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return int64(number), true, nil
}
