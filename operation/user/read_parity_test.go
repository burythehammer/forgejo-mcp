// SPDX-License-Identifier: GPL-3.0-or-later

package user

import (
	"context"
	"encoding/json"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
	apporg "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/org"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/orgadapter"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// TestOwnerReadParity_UserAgreesWithTheToolAndOrgWithGetOrg checks the owner
// resource against the paths that answer the same question with a tool: the
// user named by a login is the one get_my_user_info returns, and the org named
// by a login is the one get_org returns (the org tool renders app/org's
// GetOrg result as it is).
func TestOwnerReadParity_UserAgreesWithTheToolAndOrgWithGetOrg(t *testing.T) {
	t.Parallel()
	f := fake.New()
	h := handlers{forge: fake.Factory{Forgejo: f}}
	ctx := context.Background()

	if _, err := f.CreateOrg(ctx, forgejo_sdk.CreateOrgOption{Name: "acme", FullName: "Acme", Description: "widgets", Website: "https://acme.example", Location: "Here"}); err != nil {
		t.Fatal(err)
	}

	// The user.
	toolRes, err := h.GetUserInfoFn(ctx, mcp.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var me model.User
	if err := unmarshalToolResult(toolRes, &me); err != nil {
		t.Fatal(err)
	}
	var user ownerResourcePayload
	readJSON(t, h, me.UserName, &user)
	if user.Kind != "user" || user.Login != me.UserName || user.FullName != me.FullName || user.HTMLURL != me.HTMLURL ||
		user.Description != me.Description || user.Location != me.Location || user.Website != me.Website ||
		user.FollowersCount != me.FollowerCount || user.FollowingCount != me.FollowingCount ||
		user.CreatedAt != me.Created.Format("2006-01-02T15:04:05Z07:00") {
		t.Fatalf("owner resource disagrees with get_my_user_info\nresource: %+v\ntool:     %+v", user, me)
	}

	// The org.
	org, err := apporg.New(func(context.Context) (apporg.Port, error) { return orgadapter.New(f), nil }).GetOrg(ctx, "acme")
	if err != nil {
		t.Fatal(err)
	}
	var got ownerResourcePayload
	readJSON(t, h, "acme", &got)
	if got.Kind != "org" || got.Login != org.UserName || got.FullName != org.FullName ||
		got.Description != org.Description || got.Location != org.Location || got.Website != org.Website {
		t.Fatalf("owner resource disagrees with get_org\nresource: %+v\norg:      %+v", got, org)
	}
}

// readJSON reads the owner resource for login and decodes its JSON body.
func readJSON(t *testing.T, h handlers, login string, v any) {
	t.Helper()
	contents, err := h.ownerResourceHandler(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://owner/" + login}})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(contents[0].(mcp.TextResourceContents).Text), v); err != nil {
		t.Fatal(err)
	}
}

// unmarshalToolResult decodes the Result envelope a tool wraps its output in.
func unmarshalToolResult(res *mcp.CallToolResult, v any) error {
	tc := res.Content[0].(mcp.TextContent)
	var env struct {
		Result json.RawMessage `json:"Result"`
	}
	if err := json.Unmarshal([]byte(tc.Text), &env); err != nil {
		return err
	}
	return json.Unmarshal(env.Result, v)
}
