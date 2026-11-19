// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/fake"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// domainRegistrars registers one domain's tools on a server, so each tool's
// domain is the registration it came from. A new domain is added here.
var domainRegistrars = map[string]func(*server.MCPServer, forge.Factory){
	"actions":          RegisterActionsTool,
	"attachment":       RegisterAttachmentTool,
	"branchprotection": RegisterBranchProtectionTool,
	"hook":             RegisterHookTool,
	"issue":            RegisterIssueTool,
	"org":              RegisterOrgTool,
	"packages":         RegisterPackagesTool,
	"pull": func(s *server.MCPServer, f forge.Factory) {
		RegisterPullTool(s, f)
		RegisterPullReviewTool(s, f)
	},
	"release":  RegisterReleaseTool,
	"repo":     RegisterRepoTool,
	"search":   RegisterSearchTool,
	"tracking": RegisterTrackingTool,
	"user":     RegisterUserTool,
	"version":  func(s *server.MCPServer, f forge.Factory) { RegisterVersionTool(s, "test", f) },
	"wiki":     RegisterWikiTool,
}

// noCredential lists the domains whose tools take no Forgejo credential, so
// the refusal-ordering check does not apply to them.
var noCredential = map[string]bool{
	"version": true,
}

func sortedDomains() []string {
	names := make([]string, 0, len(domainRegistrars))
	for name := range domainRegistrars {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func registerDomain(name string, f forge.Factory) *server.MCPServer {
	s := server.NewMCPServer("test", "0")
	domainRegistrars[name](s, f)
	return s
}

func TestRegistryDomainsCoverEveryTool(t *testing.T) {
	t.Parallel()
	all := newRegisteredServer(t).ListTools()
	fromDomains := map[string]bool{}
	for _, name := range sortedDomains() {
		for tool := range registerDomain(name, fake.Factory{Forgejo: fake.New()}).ListTools() {
			fromDomains[tool] = true
		}
	}
	for tool := range all {
		if !fromDomains[tool] {
			t.Errorf("tool %q is registered but belongs to no domain in domainRegistrars", tool)
		}
	}
	for tool := range fromDomains {
		if _, ok := all[tool]; !ok {
			t.Errorf("domain registrar names %q, which the full registry does not hold", tool)
		}
	}
}

// TestRegistryRefusesMissingRequiredArgumentBeforeCredential checks that a bad
// request (a call that omits a required argument) is refused before the
// credential: every tool of every domain is called that way, against a factory
// that refuses every credential. The refusal must be a tool error with text,
// and the credential refusal must never appear. Refusing an unknown argument is
// #314's concern, not this test's: lax decoding ignores unknown arguments.
func TestRegistryRefusesMissingRequiredArgumentBeforeCredential(t *testing.T) {
	t.Parallel()
	refuse := errors.New("no credential for this request")
	for _, domain := range sortedDomains() {
		if noCredential[domain] {
			continue
		}
		s := registerDomain(domain, fake.Factory{Err: refuse})
		tools := s.ListTools()
		names := make([]string, 0, len(tools))
		for name := range tools {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			// A tool with no required argument has no bad request to refuse:
			// decoding is lax by default, so an unknown argument is ignored. An
			// argument with a schema default is not required in effect (a call that
			// omits it is valid and takes the default), so it does not count.
			if !hasRequiredWithoutDefault(tools[name].Tool.InputSchema) {
				continue
			}
			req := mcp.CallToolRequest{Params: mcp.CallToolParams{
				Name:      name,
				Arguments: map[string]any{"not_a_real_argument": "x"},
			}}
			res, err := s.GetTool(name).Handler(context.Background(), req)
			if err != nil {
				t.Errorf("%s (%s): bad request returned Go error %q; want a tool error", name, domain, err)
				continue
			}
			if res == nil || !res.IsError {
				t.Errorf("%s (%s): bad request was not refused as a tool error", name, domain)
				continue
			}
			if len(res.Content) == 0 {
				t.Errorf("%s (%s): refusal has no content", name, domain)
				continue
			}
			content, ok := res.Content[0].(mcp.TextContent)
			if !ok {
				t.Errorf("%s (%s): refusal content is not text: %T", name, domain, res.Content[0])
				continue
			}
			text := content.Text
			if strings.Contains(text, refuse.Error()) {
				t.Errorf("%s (%s): reached the credential lookup: %q", name, domain, text)
			}
			if strings.TrimSpace(text) == "" {
				t.Errorf("%s (%s): refusal has no text", name, domain)
			}
		}
	}
}

// hasRequiredWithoutDefault reports whether a tool's schema has a required
// argument that takes no default. A required argument with a default is not
// required in effect: a call that omits it is valid, and its golden pins that.
func hasRequiredWithoutDefault(schema mcp.ToolInputSchema) bool {
	for _, name := range schema.Required {
		prop, ok := schema.Properties[name].(map[string]any)
		if !ok {
			return true
		}
		if _, hasDefault := prop["default"]; !hasDefault {
			return true
		}
	}
	return false
}
