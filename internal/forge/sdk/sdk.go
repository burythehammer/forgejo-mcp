// SPDX-License-Identifier: GPL-3.0-or-later

// Package sdk is the real Forgejo adapter: it satisfies forge.Forgejo with the
// Forgejo SDK and, for the endpoints the SDK lacks, one raw-HTTP client, behind
// one type, so callers cannot tell which transport serves a given method. It is
// also the only package that knows the SDK client, the raw transport and the
// credential policy.
package sdk

import (
	"context"
	"net/http"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Adapter is the real forge.Forgejo, bound to one credential and one instance.
type Adapter struct {
	client   *forgejo_sdk.Client
	raw      *rawClient // the one raw-HTTP path, same instance and credential
	settings *settings
}

var _ forge.Forgejo = (*Adapter)(nil)

// NewAdapter binds an adapter to an already-built SDK client. It is a test seam
// only: its raw-HTTP client is the default one, without the operator's TLS
// settings. The factory builds production adapters with those settings on both
// the SDK and the raw-HTTP client, so production code must go through
// NewFactory. Callers outside a test binary panic rather than run without TLS.
// Tests in other packages need it too, so it stays exported; WithRawBaseURL
// points its raw-HTTP half at a stub server.
//
// Each domain's methods live in their own file in this package (pull.go, ...),
// all on *Adapter.
func NewAdapter(c *forgejo_sdk.Client) *Adapter {
	if !testing.Testing() {
		panic("sdk.NewAdapter is a test seam without TLS settings; build adapters through sdk.NewFactory")
	}
	return &Adapter{client: c, raw: newRawClient("", "", ""), settings: &settings{}}
}

// WithRawBaseURL returns a copy of the adapter whose raw-HTTP methods (the ones
// the SDK has no call for) talk to the Forgejo at url. The factory sets it from
// Config; contract tests point it at the stub.
func (a *Adapter) WithRawBaseURL(url string) *Adapter {
	c := *a
	c.raw = a.raw.withBase(url)
	return &c
}

// statusErr wraps an SDK failure with the HTTP status Forgejo answered with,
// when there was one. The message is the SDK's, unchanged. ctx is not passed
// to the SDK by the adapter methods: the SDK client carries no per-call
// context, and the request deadline is the HTTP client's.
func statusErr(resp *forgejo_sdk.Response, err error) error {
	if err == nil {
		return nil
	}
	if resp == nil || resp.Response == nil {
		return deadlineErr(err)
	}
	return &forge.StatusError{StatusCode: resp.StatusCode, Err: err}
}

// rawJSON performs a JSON request for the endpoints the SDK lacks, against the
// adapter's instance with the adapter's credential. Paths are built with
// forge.APIPath so every segment is escaped. A server failure comes back as
// *forge.StatusError, with the message unchanged.
func (a *Adapter) rawJSON(ctx context.Context, method, path string, body, out any) (http.Header, error) {
	return a.raw.doJSONWithHeader(ctx, method, path, body, out)
}

// rawDo is rawJSON for callers that do not need the response headers.
func (a *Adapter) rawDo(ctx context.Context, method, path string, body, out any) error {
	return a.raw.doJSON(ctx, method, path, body, out)
}
