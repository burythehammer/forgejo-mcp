// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/config"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/toolset"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RunCLI is the entry point for --cli mode. cfg.CLI holds the parsed command.
func RunCLI(cfg config.Config, version string) {
	c := cfg.CLI
	if c.Tool == "" {
		fmt.Fprintln(os.Stderr, "Usage: forgejo-mcp --cli <command> [options]")
		fmt.Fprintln(os.Stderr, "Commands: list, <tool-name>")
		fmt.Fprintln(os.Stderr, "Options: --args '{json}', --output=json|text, --help")
		os.Exit(1)
	}

	// The same composition root as the server, so the CLI offers exactly the
	// tools the operator selected.
	built, err := operation.Build(cfg, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	mcpSrv := built.Server

	switch c.Tool {
	case "list":
		if err := cliList(mcpSrv, c.Output); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		if c.Help {
			if err := cliHelp(mcpSrv, c.Tool); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}

		argsJSON, err := resolveArgs(c.Args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading arguments: %v\n", err)
			os.Exit(1)
		}

		if err := cliExec(mcpSrv, c.Tool, argsJSON, c.Output); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}

// resolveArgs returns JSON args from --args flag or stdin pipe.
// --args takes precedence. If neither provided, returns "{}".
func resolveArgs(argsFlag string) (string, error) {
	if argsFlag != "" {
		return argsFlag, nil
	}

	// Check if stdin is a pipe (not a terminal).
	stat, err := os.Stdin.Stat()
	if err != nil {
		return "{}", nil
	}
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		if len(data) > 0 {
			return string(data), nil
		}
	}

	return "{}", nil
}

// cliList prints all registered tools.
func cliList(s *server.MCPServer, outputMode string) error {
	tools := s.ListTools()
	if tools == nil {
		fmt.Println("No tools registered.")
		return nil
	}

	type toolInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Domain      string `json:"domain"`
	}

	// Build sorted list.
	var infos []toolInfo
	for name, st := range tools {
		domain := toolset.Of(name)
		if domain == "" {
			domain = "other"
		}
		infos = append(infos, toolInfo{
			Name:        name,
			Description: st.Tool.Description,
			Domain:      domain,
		})
	}
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].Domain != infos[j].Domain {
			return infos[i].Domain < infos[j].Domain
		}
		return infos[i].Name < infos[j].Name
	})

	if outputMode == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(infos)
	}

	// Text mode: grouped by domain.
	grouped := map[string][]toolInfo{}
	domainOrder := []string{}
	for _, info := range infos {
		if _, exists := grouped[info.Domain]; !exists {
			domainOrder = append(domainOrder, info.Domain)
		}
		grouped[info.Domain] = append(grouped[info.Domain], info)
	}

	for _, domain := range domainOrder {
		fmt.Printf("\n%s:\n", strings.ToUpper(domain))
		for _, info := range grouped[domain] {
			fmt.Printf("  %-40s %s\n", info.Name, info.Description)
		}
	}
	fmt.Println()

	return nil
}

// cliHelp prints the parameter schema for a tool.
func cliHelp(s *server.MCPServer, toolName string) error {
	st := s.GetTool(toolName)
	if st == nil {
		return fmt.Errorf("unknown tool: %s", toolName)
	}

	fmt.Printf("Tool: %s\n", st.Tool.Name)
	if st.Tool.Description != "" {
		fmt.Printf("Description: %s\n", st.Tool.Description)
	}
	fmt.Println()

	props := st.Tool.InputSchema.Properties
	required := st.Tool.InputSchema.Required

	if len(props) == 0 {
		fmt.Println("No parameters.")
		return nil
	}

	fmt.Println("Parameters:")
	// Sort parameter names for consistent output.
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	sort.Strings(names)

	requiredSet := map[string]bool{}
	for _, r := range required {
		requiredSet[r] = true
	}

	for _, name := range names {
		prop := props[name]
		reqStr := "optional"
		if requiredSet[name] {
			reqStr = "required"
		}

		// Property is stored as map[string]any.
		propMap, ok := prop.(map[string]any)
		if !ok {
			fmt.Printf("  %-20s (%s)\n", name, reqStr)
			continue
		}

		typStr, _ := propMap["type"].(string)
		desc, _ := propMap["description"].(string)

		fmt.Printf("  %-20s %-10s %-10s %s\n", name, typStr, reqStr, desc)
	}

	return nil
}

// cliExec invokes a tool handler and prints the result.
func cliExec(s *server.MCPServer, toolName, argsJSON, outputMode string) error {
	st := s.GetTool(toolName)
	if st == nil {
		return fmt.Errorf("unknown tool: %s\nRun 'forgejo-mcp --cli list' to see available tools", toolName)
	}

	// Parse JSON arguments.
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Errorf("invalid JSON arguments: %w", err)
	}

	// Construct CallToolRequest.
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: args,
		},
	}

	// Call the handler. The CLI runs in-process for the operator, like stdio, so
	// the request is marked for the operator's own credential.
	result, err := st.Handler(forge.WithOperatorCredential(context.Background()), req)
	if err != nil {
		return fmt.Errorf("tool execution failed: %w", err)
	}

	// Check IsError flag.
	if result.IsError {
		if outputMode == "json" {
			enc := json.NewEncoder(os.Stderr)
			enc.SetIndent("", "  ")
			_ = enc.Encode(result.Content)
		} else {
			for _, c := range result.Content {
				if tc, ok := c.(mcp.TextContent); ok {
					fmt.Fprintln(os.Stderr, tc.Text)
				}
			}
		}
		os.Exit(1)
	}

	// Output result.
	if outputMode == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result.Content)
	}

	// Text mode: print text content line by line.
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			fmt.Println(tc.Text)
		}
	}

	return nil
}
