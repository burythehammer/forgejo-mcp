// SPDX-License-Identifier: GPL-3.0-or-later

// Package config resolves the settings every run mode shares into one typed
// Config: explicit flag, then environment variable, then default.
package config

// Mode is how the process runs.
type Mode string

const (
	// ModeServe runs an MCP server on a transport.
	ModeServe Mode = "serve"
	// ModeCLI runs one tool invocation and exits (--cli).
	ModeCLI Mode = "cli"
)

// Build is information about the binary, not an operator setting.
type Build struct {
	Version string
}

// Warning reports a deprecated environment variable that supplied a value.
// The caller decides how to log it.
type Warning struct {
	Deprecated string
	Preferred  string
}

// Config is every operator-supplied setting of a running server.
type Config struct {
	Mode      Mode
	Transport string // stdio, sse or http
	Forgejo   Forgejo
	Listen    Listen
	Auth      Auth
	Debug     bool
	CLI       CLI // set in ModeCLI only

	// suppliedResourceServer names the resource-server-only settings given at
	// all, as a flag or an environment variable. Only validation reads it.
	suppliedResourceServer []string
}

// CLI is the one tool invocation --cli mode performs.
type CLI struct {
	Tool   string // tool name, or "list"; empty when none was given
	Args   string // JSON arguments; empty means read them from stdin
	Output string // json or text
	Help   bool   // show the tool's parameters instead of running it
}

// Forgejo is the instance this server talks to.
type Forgejo struct {
	URL       string
	Token     Secret
	UserAgent string
}

// Listen holds the network transport settings.
type Listen struct {
	SSEPort                    int
	HTTPPort                   int
	Host                       string
	AllowedHosts               []string
	AllowedOrigins             []string
	AllowOperatorTokenFallback bool
}

// Auth selects how the http transport authenticates callers. Every field but
// Mode applies to the resource-server mode only.
type Auth struct {
	Mode                        string // passthrough or resource-server
	AuthorizationServer         string
	Resource                    string
	ResourceAudience            string
	ScopesSupported             []string
	ForgejoAudienceClaim        string
	ForgejoJWTIssuer            string
	ForgejoJWTSigningKeyFile    string
	ForgejoJWTPublishedKeyFiles []string
}

// ResourceServerMode reports whether Auth selects the resource-server mode.
func (c Config) ResourceServerMode() bool { return c.Auth.Mode == AuthModeResourceServer }
