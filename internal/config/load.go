// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/toolset"
)

const defaultAudienceClaim = "forgejo_aud"

// UsageError is a flag parsing failure with the usage text to show for it.
// Unwrapping it yields flag.ErrHelp for -h.
type UsageError struct {
	Err   error
	Usage string
}

func (e *UsageError) Error() string { return e.Err.Error() }
func (e *UsageError) Unwrap() error { return e.Err }

// ValidationError is Validate's refusal as Load returns it.
type ValidationError struct {
	Err error
}

func (e *ValidationError) Error() string { return e.Err.Error() }
func (e *ValidationError) Unwrap() error { return e.Err }

// IsVersionRequest reports whether args ask for the version, which must be
// answered before Load so that no URL is required.
func IsVersionRequest(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "version", "--version", "-version":
		return true
	}
	return false
}

type resourceSetting struct {
	flagName string
	envVar   string
	help     string
}

var resourceSettings = []resourceSetting{
	{"authorization-server", "FORGEJO_MCP_AUTHORIZATION_SERVER", "resource-server mode: issuer URL of the identity provider callers obtain tokens from"},
	{"authorization-server-tls-ca", "FORGEJO_MCP_AUTHORIZATION_SERVER_TLS_CA", "resource-server mode: PEM file of CA certificates trusted for the identity provider's discovery and key lookups, added to the system roots"},
	{"resource", "FORGEJO_MCP_RESOURCE", "resource-server mode: canonical URI of this server's MCP endpoint, e.g. https://mcp.example.org/mcp"},
	{"resource-audience", "FORGEJO_MCP_RESOURCE_AUDIENCE", "resource-server mode: value an inbound token's aud must contain (default: the value of -resource)"},
	{"scopes-supported", "FORGEJO_MCP_SCOPES_SUPPORTED", "resource-server mode: space-separated scopes advertised to clients"},
	{"forgejo-audience-claim", "FORGEJO_MCP_FORGEJO_AUDIENCE_CLAIM", "resource-server mode: inbound claim holding the caller's Forgejo integration audience (default: " + defaultAudienceClaim + ")"},
	{"forgejo-jwt-issuer", "FORGEJO_MCP_FORGEJO_JWT_ISSUER", "resource-server mode: https issuer URL this server presents to Forgejo, e.g. https://mcp.example.org/issuer"},
	{"forgejo-jwt-signing-key-file", "FORGEJO_MCP_FORGEJO_JWT_SIGNING_KEY_FILE", "resource-server mode: PEM private key (EC P-256/P-384, Ed25519 or RSA >= 2048 bits) that signs Forgejo tokens"},
	{"forgejo-jwt-published-key-files", "FORGEJO_MCP_FORGEJO_JWT_PUBLISHED_KEY_FILES", "resource-server mode: comma-separated PEM keys published without signing with them, for key rotation"},
}

// Load resolves args and the environment into a Config, flag over environment
// over default. It reads no process state: getenv stands in for os.Getenv and
// args excludes the program name. With --cli among args, the options before it
// are server options and parse as in serve mode; the arguments after it belong
// to the CLI.
func Load(args []string, getenv func(string) string, build Build) (Config, []Warning, error) {
	var (
		cfg   Config
		warns []Warning

		transport, urlFlag, token, userAgent string
		requestTimeout                       string
		host, allowedHosts, allowedOrigins   string
		shutdownGrace                        string
		ssePort, httpPort                    int
		allowFallback, debug                 bool
		maxRequestBody                       int64
		authMode                             string
		tlsCA                                string
		tlsSkipVerify                        bool
		toolsets, tools                      string
		preset                               string
		lean                                 bool
		laxArgs                              bool
	)
	resource := make(map[string]*string, len(resourceSettings))

	fs := flag.NewFlagSet("forgejo-mcp", flag.ContinueOnError)
	var out bytes.Buffer
	fs.SetOutput(&out)

	fs.StringVar(&transport, "t", "stdio", "Transport type (stdio, sse, or http)")
	fs.StringVar(&transport, "transport", "stdio", "Transport type (stdio, sse, or http)")
	fs.StringVar(&urlFlag, "url", "", "Forgejo instance URL (required, must start with http:// or https://)")
	fs.IntVar(&ssePort, "sse-port", 8080, "Port for SSE transport mode")
	fs.IntVar(&httpPort, "http-port", 8080, "Port for streamable HTTP transport mode")
	fs.StringVar(&token, "token", "", "Your personal access token")
	fs.StringVar(&userAgent, "user-agent", "", "User agent for HTTP requests (default: forgejo-mcp/<version>)")
	fs.StringVar(&requestTimeout, "request-timeout", "",
		"Longest wait for one Forgejo request, as a Go duration such as 30s "+
			"(default: 60s; env: FORGEJO_REQUEST_TIMEOUT)")
	fs.StringVar(&tlsCA, "tls-ca", "",
		"PEM file of CA certificates trusted for the Forgejo instance, added to the system roots "+
			"(env: FORGEJO_TLS_CA)")
	fs.BoolVar(&tlsSkipVerify, "tls-skip-verify", false,
		"Skip verification of the Forgejo instance's TLS certificate. Off by default; logs a warning "+
			"(env: FORGEJO_TLS_SKIP_VERIFY)")
	fs.StringVar(&host, "host", "localhost",
		"Address the sse and http transports bind to. The default reaches this machine only, "+
			"binding both 127.0.0.1 and ::1; pass one of those addresses to bind that family alone; "+
			"set 0.0.0.0 to accept connections from the network, which also requires -allowed-hosts")
	fs.StringVar(&allowedHosts, "allowed-hosts", "",
		"Comma-separated Host names this server answers to, required when -host is not loopback "+
			"(for example: mcp.example.org,mcp.example.org:8080)")
	fs.StringVar(&allowedOrigins, "allowed-origins", "",
		"Comma-separated web origins allowed to send an Origin header, as full origins "+
			"(for example: https://console.example.org). Empty means no browser origin is accepted; "+
			"requests with no Origin header are unaffected")
	fs.StringVar(&shutdownGrace, "shutdown-grace", DefaultShutdownGrace.String(),
		"How long in-flight HTTP requests may run after SIGINT or SIGTERM before open connections, "+
			"including event streams, are closed; a positive Go duration such as 8s or 1m "+
			"(env: FORGEJO_MCP_SHUTDOWN_GRACE)")
	fs.BoolVar(&allowFallback, "allow-operator-token-fallback", false,
		"On the sse and http transports, serve a request that carries no Authorization header using "+
			"this server's own token. Off by default: it hands this server's forge credential to any "+
			"caller that can reach the port")
	fs.Int64Var(&maxRequestBody, "max-request-body-bytes", DefaultMaxRequestBodyBytes,
		"Largest request body the sse and http transports read, in bytes; a larger request gets 413. "+
			"Must be positive. The default clears the largest attachment upload (env: FORGEJO_MCP_MAX_REQUEST_BODY_BYTES)")
	fs.BoolVar(&debug, "d", false, "debug mode")
	fs.BoolVar(&debug, "debug", false, "debug mode")
	fs.StringVar(&authMode, "auth-mode", "passthrough",
		"How the http transport authenticates callers: passthrough forwards the caller's Authorization header to "+
			"Forgejo; resource-server validates an identity provider's JWT access token and signs a Forgejo "+
			"Authorized Integration token (Forgejo 16 or newer)")
	fs.StringVar(&toolsets, "toolsets", "", "(env: FORGEJO_MCP_TOOLSETS) "+toolset.FlagHelp())
	fs.StringVar(&tools, "tools", "",
		"Comma-separated names of individual tools to load, in addition to any -toolsets "+
			"(env: FORGEJO_MCP_TOOLS)")
	fs.StringVar(&preset, "preset", "", "(env: FORGEJO_MCP_PRESET) "+toolset.PresetFlagHelp())
	fs.BoolVar(&lean, "lean", false,
		"Serve terse tool text: each description cut to its first sentence, most property descriptions removed, "+
			"annotations empty; the same tool and argument names (env: FORGEJO_MCP_LEAN)")
	fs.BoolVar(&laxArgs, "lax-args", false,
		"Ignore an argument a tool does not declare, as every tool did before, instead of refusing the call. "+
			"Only search_issues refuses such an argument by default (env: FORGEJO_MCP_LAX_ARGS)")
	for _, s := range resourceSettings {
		p := new(string)
		resource[s.flagName] = p
		fs.StringVar(p, s.flagName, "", s.help)
	}

	cfg.Mode = ModeServe
	// Server options go before --cli and are parsed like in serve mode; the
	// arguments after it belong to the CLI.
	var cliArgs []string
	serverArgs := args
	for i, a := range args {
		if a == "--cli" {
			cfg.Mode = ModeCLI
			serverArgs = args[:i]
			cliArgs = args[i+1:]
			break
		}
	}
	if err := fs.Parse(serverArgs); err != nil {
		var usage bytes.Buffer
		fs.SetOutput(&usage)
		fs.PrintDefaults()
		return Config{}, nil, &UsageError{Err: err, Usage: usage.String()}
	}
	if cfg.Mode == ModeCLI {
		c, err := parseCLI(cliArgs)
		if err != nil {
			return Config{}, nil, err
		}
		cfg.CLI = c
	}

	passed := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { passed[f.Name] = true })
	resolve := func(value, envVar, name string) string {
		if passed[name] {
			return value
		}
		if v := getenv(envVar); v != "" {
			return v
		}
		return value
	}

	cfg.Transport = transport
	cfg.Listen.SSEPort, cfg.Listen.HTTPPort = ssePort, httpPort

	cfg.Forgejo.URL = urlFlag
	if cfg.Forgejo.URL == "" {
		cfg.Forgejo.URL = getenv("FORGEJO_URL")
	}
	if cfg.Forgejo.URL == "" {
		if v := getenv("GITEA_HOST"); v != "" {
			warns = append(warns, Warning{Deprecated: "GITEA_HOST", Preferred: "FORGEJO_URL"})
			cfg.Forgejo.URL = v
		}
	}
	if cfg.Forgejo.URL == "" {
		return Config{}, warns, errors.New("missing required configuration: url (provide -url or FORGEJO_URL)")
	}
	if err := validateURL(cfg.Forgejo.URL); err != nil {
		return Config{}, warns, err
	}

	tok := token
	if tok == "" {
		tok = getenv("FORGEJO_ACCESS_TOKEN")
	}
	if tok == "" {
		if v := getenv("GITEA_ACCESS_TOKEN"); v != "" {
			warns = append(warns, Warning{Deprecated: "GITEA_ACCESS_TOKEN", Preferred: "FORGEJO_ACCESS_TOKEN"})
			tok = v
		}
	}
	cfg.Forgejo.Token = NewSecret(tok)

	timeout, err := parseRequestTimeout(resolve(requestTimeout, "FORGEJO_REQUEST_TIMEOUT", "request-timeout"))
	if err != nil {
		return Config{}, warns, err
	}
	cfg.Forgejo.RequestTimeout = timeout

	cfg.Forgejo.UserAgent = userAgent
	if cfg.Forgejo.UserAgent == "" {
		cfg.Forgejo.UserAgent = getenv("FORGEJO_USER_AGENT")
	}
	if cfg.Forgejo.UserAgent == "" && build.Version != "" {
		// An unknown version leaves the default unset rather than naming a
		// release that does not exist.
		cfg.Forgejo.UserAgent = "forgejo-mcp/" + build.Version
	}

	skipVerify := tlsSkipVerify
	if !passed["tls-skip-verify"] {
		if v := getenv("FORGEJO_TLS_SKIP_VERIFY"); v != "" {
			skipVerify = isTruthy(v)
		}
	}
	tlsCfg, err := loadTLS(resolve(tlsCA, "FORGEJO_TLS_CA", "tls-ca"), skipVerify)
	if err != nil {
		return Config{}, warns, err
	}
	cfg.Forgejo.TLS = tlsCfg

	cfg.Toolsets = splitList(resolve(toolsets, "FORGEJO_MCP_TOOLSETS", "toolsets"))
	cfg.Tools = splitList(resolve(tools, "FORGEJO_MCP_TOOLS", "tools"))
	cfg.Preset = strings.TrimSpace(resolve(preset, "FORGEJO_MCP_PRESET", "preset"))
	cfg.Lean = lean
	if !passed["lean"] {
		if v := getenv("FORGEJO_MCP_LEAN"); v != "" {
			cfg.Lean = isTruthy(v)
		}
	}
	cfg.LaxArgs = laxArgs
	if !passed["lax-args"] {
		if v := getenv("FORGEJO_MCP_LAX_ARGS"); v != "" {
			cfg.LaxArgs = isTruthy(v)
		}
	}
	if _, err := toolset.ParseWithPreset(cfg.Preset, cfg.Toolsets, cfg.Tools); err != nil {
		return Config{}, warns, fmt.Errorf("refusing to start: %w", err)
	}

	cfg.Listen.Host = resolve(host, "FORGEJO_MCP_HOST", "host")
	cfg.Listen.AllowedHosts = splitList(resolve(allowedHosts, "FORGEJO_MCP_ALLOWED_HOSTS", "allowed-hosts"))
	cfg.Listen.AllowedOrigins = splitList(resolve(allowedOrigins, "FORGEJO_MCP_ALLOWED_ORIGINS", "allowed-origins"))
	grace, err := time.ParseDuration(resolve(shutdownGrace, "FORGEJO_MCP_SHUTDOWN_GRACE", "shutdown-grace"))
	if err != nil {
		return Config{}, warns, fmt.Errorf("invalid shutdown grace period: %w (use a positive duration such as 8s)", err)
	}
	if grace <= 0 {
		return Config{}, warns, fmt.Errorf("invalid shutdown grace period %v: must be positive", grace)
	}
	cfg.Listen.ShutdownGrace = grace
	cfg.Listen.AllowOperatorTokenFallback = allowFallback
	if !passed["allow-operator-token-fallback"] {
		if v := getenv("FORGEJO_MCP_ALLOW_OPERATOR_TOKEN_FALLBACK"); v != "" {
			cfg.Listen.AllowOperatorTokenFallback = isTruthy(v)
		}
	}
	cfg.Listen.MaxRequestBodyBytes = maxRequestBody
	bodyLimitSource := "-max-request-body-bytes"
	if !passed["max-request-body-bytes"] {
		if v := getenv("FORGEJO_MCP_MAX_REQUEST_BODY_BYTES"); v != "" {
			n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			if err != nil {
				return Config{}, warns, fmt.Errorf("refusing to start: FORGEJO_MCP_MAX_REQUEST_BODY_BYTES must be a whole number of bytes, got %q", v)
			}
			cfg.Listen.MaxRequestBodyBytes = n
			bodyLimitSource = "FORGEJO_MCP_MAX_REQUEST_BODY_BYTES"
		}
	}
	// Checked here, where the value was given, rather than in Validate: an
	// in-process Config that never set the field is zero too, and takes the default.
	if cfg.Listen.MaxRequestBodyBytes <= 0 {
		return Config{}, warns, fmt.Errorf("refusing to start: the request body limit must be a positive number of bytes, got %d from %s; "+
			"set it to a positive byte count or leave it unset for the default %d",
			cfg.Listen.MaxRequestBodyBytes, bodyLimitSource, DefaultMaxRequestBodyBytes)
	}

	switch {
	case debug:
		cfg.Debug = true
	case getenv("FORGEJO_DEBUG") == "true":
		cfg.Debug = true
	case getenv("GITEA_DEBUG") == "true":
		warns = append(warns, Warning{Deprecated: "GITEA_DEBUG", Preferred: "FORGEJO_DEBUG"})
		cfg.Debug = true
	}

	cfg.Auth.Mode = resolve(authMode, "FORGEJO_MCP_AUTH_MODE", "auth-mode")
	if cfg.Auth.Mode == "" {
		cfg.Auth.Mode = "passthrough"
	}
	values := make(map[string]string, len(resourceSettings))
	for _, s := range resourceSettings {
		values[s.flagName] = resolve(*resource[s.flagName], s.envVar, s.flagName)
		switch {
		case passed[s.flagName]:
			cfg.suppliedResourceServer = append(cfg.suppliedResourceServer, "-"+s.flagName)
		case getenv(s.envVar) != "":
			cfg.suppliedResourceServer = append(cfg.suppliedResourceServer, s.envVar)
		}
	}
	cfg.Auth.AuthorizationServer = values["authorization-server"]
	cfg.Auth.Resource = values["resource"]
	cfg.Auth.ResourceAudience = values["resource-audience"]
	cfg.Auth.ScopesSupported = strings.Fields(values["scopes-supported"])
	cfg.Auth.ForgejoAudienceClaim = values["forgejo-audience-claim"]
	if cfg.Auth.ForgejoAudienceClaim == "" {
		cfg.Auth.ForgejoAudienceClaim = defaultAudienceClaim
	}
	cfg.Auth.ForgejoJWTIssuer = values["forgejo-jwt-issuer"]
	cfg.Auth.ForgejoJWTSigningKeyFile = values["forgejo-jwt-signing-key-file"]
	cfg.Auth.ForgejoJWTPublishedKeyFiles = splitList(values["forgejo-jwt-published-key-files"])
	if caFile := values["authorization-server-tls-ca"]; caFile != "" {
		pool, err := loadCAPool("-authorization-server-tls-ca", caFile)
		if err != nil {
			return Config{}, warns, err
		}
		cfg.Auth.AuthorizationServerCAs = pool
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, warns, &ValidationError{Err: err}
	}
	return cfg, warns, nil
}

// parseCLI reads the arguments after --cli: a command, then its options.
func parseCLI(args []string) (CLI, error) {
	if len(args) == 0 {
		return CLI{}, nil
	}
	fs := flag.NewFlagSet("cli", flag.ContinueOnError)
	var out bytes.Buffer
	fs.SetOutput(&out)
	c := CLI{Tool: args[0]}
	fs.StringVar(&c.Args, "args", "", "JSON arguments for tool invocation")
	fs.StringVar(&c.Output, "output", "", "Output format: json or text")
	fs.BoolVar(&c.Help, "help", false, "Show tool parameter help")
	if err := fs.Parse(args[1:]); err != nil {
		var usage bytes.Buffer
		fs.SetOutput(&usage)
		fs.PrintDefaults()
		if !errors.Is(err, flag.ErrHelp) {
			err = fmt.Errorf("%w (server options such as --toolsets, --tools, --preset and --lean go before --cli, for example: forgejo-mcp --toolsets issues --cli list)", err)
		}
		return CLI{}, &UsageError{Err: err, Usage: usage.String()}
	}
	if c.Output == "" {
		c.Output = "json"
		if c.Tool == "list" {
			c.Output = "text"
		}
	}
	return c, nil
}

// parseRequestTimeout reads --request-timeout. An empty value means the
// default; a value that is not a positive duration is refused at startup, so a
// typo cannot quietly turn every request into an instant timeout.
func parseRequestTimeout(s string) (time.Duration, error) {
	if s == "" {
		return DefaultRequestTimeout, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid request timeout %q: %w", s, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("request timeout must be positive, got %s", s)
	}
	return d, nil
}

func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL must start with http:// or https://, got: %s", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("URL must include a host")
	}
	return nil
}

// splitList turns a comma-separated value into a trimmed, non-empty list.
func splitList(raw string) []string {
	var out []string
	for _, v := range strings.Split(raw, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// isTruthy matches the spelling accepted by this project's boolean variables.
func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
