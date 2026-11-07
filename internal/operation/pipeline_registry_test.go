// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"

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
	"version":  func(s *server.MCPServer, _ forge.Factory) { RegisterVersionTool(s, "test") },
	"wiki":     RegisterWikiTool,
}

// noCredential lists the domains whose tools take no Forgejo credential, so
// the refusal-ordering check does not apply to them.
var noCredential = map[string]bool{
	"version": true,
}

// unmigrated is the allowlist of domains whose tools have not yet moved to the
// request pipeline. A migration ticket deletes its domain's line. Keep it
// sorted, one domain per line; it may only shrink.
var unmigrated = map[string]bool{
	"actions":          true,
	"attachment":       true,
	"branchprotection": true,
	"hook":             true,
	"org":              true,
	"release":          true,
	"repo":             true,
	"search":           true,
	"tracking":         true,
	"wiki":             true,
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

func TestRegistryAllowlistNamesOnlyRealDomains(t *testing.T) {
	t.Parallel()
	for name := range unmigrated {
		if _, ok := domainRegistrars[name]; !ok {
			t.Errorf("unmigrated lists %q, which is not a domain; remove the stale line", name)
		}
	}
}

// TestRegistryPipelineDomainsRefuseBadRequestBeforeCredential calls every tool
// of every migrated domain with no valid arguments, plus an unknown one, and a
// factory that refuses every credential. The refusal must be a tool error
// naming the bad request, and the credential refusal must never appear.
func TestRegistryPipelineDomainsRefuseBadRequestBeforeCredential(t *testing.T) {
	t.Parallel()
	refuse := errors.New("no credential for this request")
	for _, domain := range sortedDomains() {
		if unmigrated[domain] || noCredential[domain] {
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
			// decoding is lax by default, so an unknown argument is ignored.
			if len(tools[name].Tool.InputSchema.Required) == 0 {
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
			text := res.Content[0].(mcp.TextContent).Text
			if strings.Contains(text, refuse.Error()) {
				t.Errorf("%s (%s): reached the credential lookup: %q", name, domain, text)
			}
			if !strings.Contains(text, "required") {
				t.Errorf("%s (%s): refusal %q does not name the missing argument", name, domain, text)
			}
		}
	}
}
