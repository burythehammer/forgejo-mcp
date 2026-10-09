package cmd

import (
	"context"
	"errors"
	stdflag "flag"
	"fmt"
	"os"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/config"
	flagPkg "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
)

var (
	transport      string
	urlFlag        string
	ssePort        int
	httpPort       int
	token          string
	userAgent      string
	host           string
	allowedHosts   string
	allowedOrigins string
	allowFallback  bool

	// flagSet is retained so initConfig can ask whether a flag was actually
	// passed, rather than comparing it to its default value — which cannot
	// tell "unset" from "explicitly set to the default".
	flagSet *stdflag.FlagSet

	debug bool
)

// initFlags registers and parses CLI flags using a dedicated FlagSet to avoid
// polluting the global flag.CommandLine (which breaks `go test`).
func initFlags() {
	fs := stdflag.NewFlagSet("forgejo-mcp", stdflag.ExitOnError)
	flagSet = fs

	fs.StringVar(
		&transport,
		"t",
		"stdio",
		"Transport type (stdio, sse, or http)",
	)
	fs.StringVar(
		&transport,
		"transport",
		"stdio",
		"Transport type (stdio, sse, or http)",
	)
	fs.StringVar(
		&urlFlag,
		"url",
		"",
		"Forgejo instance URL (required, must start with http:// or https://)",
	)
	fs.IntVar(
		&ssePort,
		"sse-port",
		8080,
		"Port for SSE transport mode",
	)
	fs.IntVar(
		&httpPort,
		"http-port",
		8080,
		"Port for streamable HTTP transport mode",
	)
	fs.StringVar(
		&token,
		"token",
		"",
		"Your personal access token",
	)
	fs.StringVar(
		&userAgent,
		"user-agent",
		"",
		"User agent for HTTP requests (default: forgejo-mcp/<version>)",
	)
	fs.StringVar(
		&host,
		"host",
		"localhost",
		"Address the sse and http transports bind to. The default reaches this machine only, "+
			"binding both 127.0.0.1 and ::1; pass one of those addresses to bind that family alone; "+
			"set 0.0.0.0 to accept connections from the network, which also requires -allowed-hosts",
	)
	fs.StringVar(
		&allowedHosts,
		"allowed-hosts",
		"",
		"Comma-separated Host names this server answers to, required when -host is not loopback "+
			"(for example: mcp.example.org,mcp.example.org:8080)",
	)
	fs.StringVar(
		&allowedOrigins,
		"allowed-origins",
		"",
		"Comma-separated web origins allowed to send an Origin header, as full origins "+
			"(for example: https://console.example.org). Empty means no browser origin is accepted; "+
			"requests with no Origin header are unaffected",
	)
	fs.BoolVar(
		&allowFallback,
		"allow-operator-token-fallback",
		false,
		"On the sse and http transports, serve a request that carries no Authorization header using "+
			"this server's own token. Off by default: it hands this server's forge credential to any "+
			"caller that can reach the port",
	)
	fs.BoolVar(
		&debug,
		"d",
		false,
		"debug mode",
	)
	fs.BoolVar(
		&debug,
		"debug",
		false,
		"debug mode",
	)

	registerAuthFlags(fs)

	// ExitOnError: Parse exits the process on error, so the return is moot.
	_ = fs.Parse(os.Args[1:])

	initConfig()
}

// version is the build version handed to Execute, kept for initConfig.
var version string

// loadErr is the refusal config.Load returned, reported by Execute.
// TEMPORARY with the shim: see config.ValidationError.
var loadErr error

// initConfig resolves every shared setting through config.Load and copies the
// result into the legacy globals.
func initConfig() {
	cfg, warns, err := config.Load(os.Args[1:], os.Getenv, config.Build{Version: version})
	var invalid *config.ValidationError
	if errors.As(err, &invalid) {
		// Refused, but still copied below so the globals reflect what was
		// asked for; Execute stops on loadErr before anything runs.
		loadErr, cfg, err = err, invalid.Rejected, nil
	}
	if err != nil {
		var usage *config.UsageError
		if errors.As(err, &usage) {
			fmt.Fprint(os.Stderr, usage.Usage)
			if errors.Is(err, stdflag.ErrHelp) {
				os.Exit(0)
			}
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		log.Fatal("Invalid configuration", log.ErrorField(err))
	}
	log.Configure(cfg.Debug)
	for _, w := range warns {
		log.Warn("Deprecated environment variable used",
			log.StringField("deprecated_var", w.Deprecated),
			log.StringField("preferred_var", w.Preferred),
			log.StringField("migration_help", "Please update your configuration to use "+w.Preferred),
		)
	}
	applyToFlagPkg(cfg)
	transport = cfg.Transport
}

// applyToFlagPkg copies cfg into the pkg/flag globals every existing reader
// still uses.
//
// TEMPORARY: this shim exists only while consumers migrate to passing Config
// explicitly. #43 deletes it together with pkg/flag.
func applyToFlagPkg(cfg config.Config) {
	flagPkg.URL = cfg.Forgejo.URL
	flagPkg.Token = cfg.Forgejo.Token.Reveal()
	flagPkg.UserAgent = cfg.Forgejo.UserAgent
	flagPkg.SSEPort = cfg.Listen.SSEPort
	flagPkg.HTTPPort = cfg.Listen.HTTPPort
	flagPkg.Host = cfg.Listen.Host
	flagPkg.AllowedHosts = cfg.Listen.AllowedHosts
	flagPkg.AllowedOrigins = cfg.Listen.AllowedOrigins
	flagPkg.AllowOperatorTokenFallback = cfg.Listen.AllowOperatorTokenFallback
	flagPkg.Debug = cfg.Debug
	flagPkg.AuthMode = cfg.Auth.Mode
	flagPkg.AuthorizationServer = cfg.Auth.AuthorizationServer
	flagPkg.Resource = cfg.Auth.Resource
	flagPkg.ResourceAudience = cfg.Auth.ResourceAudience
	flagPkg.ScopesSupported = cfg.Auth.ScopesSupported
	flagPkg.ForgejoAudienceClaim = cfg.Auth.ForgejoAudienceClaim
	flagPkg.ForgejoJWTIssuer = cfg.Auth.ForgejoJWTIssuer
	flagPkg.ForgejoJWTSigningKeyFile = cfg.Auth.ForgejoJWTSigningKeyFile
	flagPkg.ForgejoJWTPublishedKeyFiles = cfg.Auth.ForgejoJWTPublishedKeyFiles
	flagPkg.ResourceServerSettings = cfg.SuppliedResourceServerSettings()
}

func Execute(buildVersion string) {
	version = buildVersion
	if config.IsVersionRequest(os.Args[1:]) {
		fmt.Printf("forgejo-mcp %s\n", version)
		return
	}

	// CLI mode: detect --cli early, skip default flag parsing since CLI
	// has its own args (tool name, --args, --output) that would confuse it.
	cliMode = hasCLIFlag()
	if cliMode {
		initConfig()
	} else {
		initFlags()
	}

	// Checked before either entry point, so that --cli, which never reaches
	// operation.Run, cannot run with an auth configuration the server refuses.
	if loadErr != nil {
		log.Fatal("Invalid configuration", log.ErrorField(loadErr))
	}

	// Sync flushes buffered logs at exit; its error (e.g. syncing stderr) is
	// not actionable here.
	defer func() { _ = log.Default().Sync() }()

	if cliMode {
		RunCLI(version)
		return
	}

	log.Infof("Starting Forgejo MCP Server %s", version)
	log.Info("Server configuration loaded",
		log.SanitizedURLField("url", flagPkg.URL),
		log.StringField("transport", transport),
		log.IntField("sse-port", flagPkg.SSEPort),
		log.BoolField("debug", flagPkg.Debug),
		log.BoolField("token_configured", flagPkg.Token != ""),
		log.StringField("user_agent", flagPkg.UserAgent),
	)

	if err := operation.Run(transport, version); err != nil {
		if errors.Is(err, context.Canceled) {
			log.Info("Server shutdown due to context cancellation")
			return
		}
		log.Fatalf("Run Forgejo MCP Server Error: %v", err)
	}
}

// flagWasPassed reports whether the named flag was actually given on the
// command line. Comparing a flag to its default value cannot tell "not set"
// from "set to the default", which is how an environment variable came to
// override an explicit -host 127.0.0.1.
func flagWasPassed(name string) bool {
	if flagSet == nil {
		return false
	}
	passed := false
	flagSet.Visit(func(f *stdflag.Flag) {
		if f.Name == name {
			passed = true
		}
	})
	return passed
}

// resolveString returns the flag value when the flag was passed, otherwise the
// environment variable, otherwise the flag's default.
func resolveString(value, envVar, flagName string) string {
	if flagWasPassed(flagName) {
		return value
	}
	if env := os.Getenv(envVar); env != "" {
		log.Debug("Using environment variable", log.StringField("variable", envVar))
		return env
	}
	return value
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

// isTruthy matches the spelling already accepted by this project's other
// boolean environment variables.
func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
