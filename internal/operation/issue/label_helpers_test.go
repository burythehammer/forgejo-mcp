// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

func makeReq(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

// textOf flattens a CallToolResult into a single string for substring assertions.
func textOf(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// seedRepoLabels creates n repository labels "label-1".."label-n" in
// testOwner/testRepo and returns their IDs in order.
func seedRepoLabels(t *testing.T, f *fake.Forgejo, n int) []int64 {
	t.Helper()
	ids := make([]int64, 0, n)
	for i := 1; i <= n; i++ {
		l, err := f.CreateRepoLabel(context.Background(), testOwner, testRepo, forgejo_sdk.CreateLabelOption{Name: fmt.Sprintf("label-%d", i), Color: "#aabbcc"})
		if err != nil {
			t.Fatalf("seeding label: %v", err)
		}
		ids = append(ids, l.ID)
	}
	return ids
}

// seedOrgLabels creates n organization labels "label-1".."label-n".
func seedOrgLabels(t *testing.T, f *fake.Forgejo, org string, n int) []int64 {
	t.Helper()
	ids := make([]int64, 0, n)
	for i := 1; i <= n; i++ {
		l, err := f.CreateOrgLabel(context.Background(), org, forgejo_sdk.CreateLabelOption{Name: fmt.Sprintf("label-%d", i), Color: "#aabbcc"})
		if err != nil {
			t.Fatalf("seeding org label: %v", err)
		}
		ids = append(ids, l.ID)
	}
	return ids
}

// issueLabelNames is the names of the labels on testOwner/testRepo#index.
func issueLabelNames(t *testing.T, f *fake.Forgejo, index int64) []string {
	t.Helper()
	iss, err := f.GetIssue(context.Background(), testOwner, testRepo, index)
	if err != nil {
		t.Fatalf("reading issue: %v", err)
	}
	names := make([]string, len(iss.Labels))
	for i, l := range iss.Labels {
		names[i] = l.Name
	}
	return names
}

// statusPort is a port whose chosen failures carry an HTTP status, for the
// cases a fake cannot reach on its own (a 401/403 from the org endpoint).
type statusPort struct {
	*fake.Forgejo
	orgLabelsStatus int
	repoLabelReads  *int
}

func (p statusPort) ListOrgLabels(ctx context.Context, org string, page, limit int) ([]*forgejo_sdk.Label, forge.PageInfo, error) {
	if p.orgLabelsStatus != 0 {
		return nil, forge.PageInfo{}, &forge.StatusError{StatusCode: p.orgLabelsStatus, Err: errString("no")}
	}
	return p.Forgejo.ListOrgLabels(ctx, org, page, limit)
}

func (p statusPort) ListRepoLabels(ctx context.Context, owner, repo string, opt forgejo_sdk.ListLabelsOptions) ([]*forgejo_sdk.Label, forge.PageInfo, error) {
	if p.repoLabelReads != nil {
		*p.repoLabelReads++
	}
	return p.Forgejo.ListRepoLabels(ctx, owner, repo, opt)
}
