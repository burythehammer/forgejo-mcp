// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"encoding/base64"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestCoreResourceDispatchRoutesWikiTemplate(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo("o", "r")
	if _, err := f.CreateWikiPage(context.Background(), "o", "r", "Home", base64.StdEncoding.EncodeToString([]byte("# Dispatched\n")), "seed"); err != nil {
		t.Fatal(err)
	}

	s := server.NewMCPServer("forgejo-mcp", "test")
	RegisterCoreResources(s, fake.Factory{Forgejo: f})
	response := s.HandleMessage(context.Background(), []byte(`{
		"jsonrpc":"2.0",
		"id":1,
		"method":"resources/read",
		"params":{"uri":"forgejo://repo/o/r/wiki/Home"}
	}`))
	rpc, ok := response.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("wiki resource was not dispatched: %#v", response)
	}
	result, ok := rpc.Result.(mcp.ReadResourceResult)
	if !ok || len(result.Contents) != 2 {
		t.Fatalf("unexpected wiki resource response: %#v", rpc.Result)
	}
	markdown, ok := result.Contents[1].(mcp.TextResourceContents)
	if !ok || markdown.Text != "# Dispatched\n" {
		t.Fatalf("wiki handler did not produce markdown sidecar: %#v", result.Contents)
	}
}
