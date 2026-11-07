// SPDX-License-Identifier: GPL-3.0-or-later

// Package pipeline is the request pipeline of the MCP adapter: the one path a
// tool call takes from its raw arguments to its rendered result. A tool
// supplies its argument struct, a Binder that yields its bound use case, and
// the use case's run function. The pipeline decodes the arguments, runs the
// optional Validate method, refuses a bad request before the Binder looks up a
// credential, logs the call, and shapes the error. See GLOSSARY.md.
//
// The pipeline imports no domain adapter; a domain supplies its Binder.
package pipeline

import (
	"context"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Binder turns the request context into the bound use case value S, for
// example the use case built on the caller's Forgejo port. It is where a
// credential is looked up, so it runs only after the request is valid.
type Binder[S any] func(ctx context.Context) (S, error)

// Validator is the optional method an argument struct may have. It runs after
// decoding and before the Binder, for every request. Its error is a bad
// request: a plain tool error, not a Forgejo or credential error.
//
// It is exported because a method in an unexported name cannot be satisfied
// from another package, and the argument structs live in their domains.
type Validator interface {
	Validate() error
}

// defaultStrict is the decoding mode a tool gets when it does not choose one.
// It is lax, which preserves the behaviour of the handlers this replaces. A
// strict default is the switch that #314 flips.
var defaultStrict bool

// Option configures one tool's pipeline.
type Option func(*options)

type options struct {
	strict *bool
	raw    bool
}

// Lax decodes forgivingly for this tool: a missing or wrongly typed argument
// leaves its field at the zero value, whatever the package default is.
func Lax() Option { return func(o *options) { o.strict = boolPtr(false) } }

// Strict decodes strictly for this tool: a missing required argument or a
// wrongly typed one is a bad request, whatever the package default is.
func Strict() Option { return func(o *options) { o.strict = boolPtr(true) } }

// WithRaw gives the tool the raw request, read with Raw, for the few tools
// that must tell an absent argument from a present null. The Binder sees it
// too, so a check on the raw request can refuse before the credential lookup.
func WithRaw() Option { return func(o *options) { o.raw = true } }

func boolPtr(b bool) *bool { return &b }

type rawKey struct{}

// Raw returns the raw request of a tool that asked for it with WithRaw. The
// boolean is false for any other tool.
func Raw(ctx context.Context) (mcp.CallToolRequest, bool) {
	req, ok := ctx.Value(rawKey{}).(mcp.CallToolRequest)
	return req, ok
}

// Handle returns the MCP handler for one tool: decode, validate, bind, run.
// A bad request (decoding or Validate) is a tool error, and the Binder is not
// called. A Binder error is returned as a Go error, as the handlers always did.
// Whatever run returns is returned unchanged.
func Handle[S, A any](bind Binder[S], run func(context.Context, S, A) (*mcp.CallToolResult, error), opts ...Option) server.ToolHandlerFunc {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	strict := defaultStrict
	if o.strict != nil {
		strict = *o.strict
	}
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := req.Params.Name
		start := time.Now()
		log.Debugf("%s: start", name)

		var a A
		if err := decodeInto(req, &a, strict); err != nil {
			return refuse(err)
		}
		if v, ok := any(&a).(Validator); ok {
			if err := v.Validate(); err != nil {
				return refuse(err)
			}
		}
		if o.raw {
			ctx = context.WithValue(ctx, rawKey{}, req)
		}
		s, err := bind(ctx)
		if err != nil {
			return mcpresult.Fail(err)
		}
		res, err := run(ctx, s, a)
		log.Debugf("%s: finish in %s", name, time.Since(start))
		return res, err
	}
}

func decodeInto(req mcp.CallToolRequest, dst any, strict bool) error {
	if strict {
		return decode.Args(req, dst)
	}
	return decode.Lax(req, dst)
}

// refuse sends a bad request to the client as a plain tool error.
func refuse(err error) (*mcp.CallToolResult, error) {
	log.Debugf("%s", err)
	return mcp.NewToolResultError(err.Error()), nil
}
